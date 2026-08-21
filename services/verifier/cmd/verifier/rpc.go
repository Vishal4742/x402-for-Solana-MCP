package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

type rpcVerifier struct {
	rpcURL             string
	network            string
	rdb                *redis.Client
	httpClient         *http.Client
	notFoundRetryDelay time.Duration
}

func (v *rpcVerifier) mode() string { return verifyModeRPC }

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
	Params  []any  `json:"params"`
}

type rpcEnvelope struct {
	Result *rpcTransaction `json:"result"`
	Error  *rpcError       `json:"error"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcTransaction struct {
	BlockTime   *int64  `json:"blockTime"`
	Meta        *txMeta `json:"meta"`
	Transaction struct {
		Message struct {
			AccountKeys []accountKey `json:"accountKeys"`
		} `json:"message"`
	} `json:"transaction"`
}

type accountKey struct {
	Pubkey string `json:"pubkey"`
	Signer bool   `json:"signer"`
}

type txMeta struct {
	Err               any            `json:"err"`
	PreTokenBalances  []tokenBalance `json:"preTokenBalances"`
	PostTokenBalances []tokenBalance `json:"postTokenBalances"`
}

type tokenBalance struct {
	AccountIndex  int    `json:"accountIndex"`
	Mint          string `json:"mint"`
	Owner         string `json:"owner"`
	UITokenAmount struct {
		Amount string `json:"amount"`
	} `json:"uiTokenAmount"`
}

func (v *rpcVerifier) Verify(ctx context.Context, payload verifyPayload) (verifyResponse, error) {
	if payload.ExpectedNetwork != v.network {
		return verifyResponse{}, reject("network_mismatch",
			fmt.Sprintf("verifier is bound to %s, payment expects %s", v.network, payload.ExpectedNetwork))
	}

	tx, err := v.getTransaction(ctx, payload.TxSignature)
	if err != nil {
		return verifyResponse{}, err
	}
	if tx == nil && v.notFoundRetryDelay > 0 {
		// Absorb confirmation lag with a single retry.
		select {
		case <-ctx.Done():
			return verifyResponse{}, transientErr("context cancelled: " + ctx.Err().Error())
		case <-time.After(v.notFoundRetryDelay):
		}
		tx, err = v.getTransaction(ctx, payload.TxSignature)
		if err != nil {
			return verifyResponse{}, err
		}
	}
	if tx == nil {
		return verifyResponse{}, reject("transaction_not_found", "transaction not found on "+v.network)
	}

	if tx.Meta == nil {
		return verifyResponse{}, transientErr("malformed rpc response: missing meta")
	}
	if tx.Meta.Err != nil {
		return verifyResponse{}, reject("transaction_failed", "transaction failed on chain")
	}

	signer := false
	for _, key := range tx.Transaction.Message.AccountKeys {
		if key.Pubkey == payload.ClientWallet && key.Signer {
			signer = true
			break
		}
	}
	if !signer {
		return verifyResponse{}, reject("payer_mismatch", "clientWallet is not a signer of the transaction")
	}

	// The gateway binds each challenge to a unique reference key the client
	// must attach to the transfer; an empty reference skips the check (mock
	// mode and direct-verifier callers).
	if payload.ExpectedReference != "" {
		referenced := false
		for _, key := range tx.Transaction.Message.AccountKeys {
			if key.Pubkey == payload.ExpectedReference {
				referenced = true
				break
			}
		}
		if !referenced {
			return verifyResponse{}, reject("reference_mismatch", "transaction is not bound to this payment request")
		}
	}

	delta, matched, err := recipientDelta(tx.Meta, payload.ExpectedRecipient, payload.ExpectedTokenMint)
	if err != nil {
		return verifyResponse{}, err
	}
	if !matched {
		return verifyResponse{}, classifyNoMatch(tx.Meta, payload)
	}
	if delta < payload.ExpectedAmountAtomic {
		return verifyResponse{}, reject("underpayment",
			fmt.Sprintf("recipient received %d atomic units, expected %d", delta, payload.ExpectedAmountAtomic))
	}

	// Claim the signature only after every check passes so a failed
	// validation never burns it.
	if err := claimSignature(ctx, v.rdb, payload); err != nil {
		return verifyResponse{}, err
	}

	verifiedAt := time.Now().UTC()
	if tx.BlockTime != nil {
		verifiedAt = time.Unix(*tx.BlockTime, 0).UTC()
	}

	return verifyResponse{
		RequestID:    payload.RequestID,
		TxSignature:  payload.TxSignature,
		ClientWallet: payload.ClientWallet,
		Status:       "verified",
		VerifiedAt:   verifiedAt,
	}, nil
}

// recipientDelta sums post-pre token balances across every account owned by
// the recipient holding the expected mint (a recipient may hold multiple
// ATAs; an account missing from pre counts as 0).
func recipientDelta(meta *txMeta, recipient, mint string) (int64, bool, error) {
	matched := false
	var pre, post int64
	for _, b := range meta.PreTokenBalances {
		if b.Owner != recipient || b.Mint != mint {
			continue
		}
		amount, err := strconv.ParseInt(b.UITokenAmount.Amount, 10, 64)
		if err != nil {
			return 0, false, transientErr("malformed rpc response: bad token amount " + b.UITokenAmount.Amount)
		}
		matched = true
		pre += amount
	}
	for _, b := range meta.PostTokenBalances {
		if b.Owner != recipient || b.Mint != mint {
			continue
		}
		amount, err := strconv.ParseInt(b.UITokenAmount.Amount, 10, 64)
		if err != nil {
			return 0, false, transientErr("malformed rpc response: bad token amount " + b.UITokenAmount.Amount)
		}
		matched = true
		post += amount
	}
	return post - pre, matched, nil
}

// classifyNoMatch: expected mint moved for another owner -> recipient_mismatch;
// only other mints moved -> token_mint_mismatch; nothing moved -> recipient_mismatch.
func classifyNoMatch(meta *txMeta, payload verifyPayload) *verifyError {
	mintSeen := false
	otherMintSeen := false
	for _, b := range append(append([]tokenBalance{}, meta.PreTokenBalances...), meta.PostTokenBalances...) {
		if b.Mint == payload.ExpectedTokenMint {
			mintSeen = true
		} else {
			otherMintSeen = true
		}
	}
	if mintSeen {
		return reject("recipient_mismatch", "expected recipient received no expected-mint tokens")
	}
	if otherMintSeen {
		return reject("token_mint_mismatch", "transaction moved a different token mint")
	}
	return reject("recipient_mismatch", "transaction contains no token transfer to the expected recipient")
}

func (v *rpcVerifier) getTransaction(ctx context.Context, signature string) (*rpcTransaction, error) {
	body, err := json.Marshal(rpcRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "getTransaction",
		Params: []any{signature, map[string]any{
			"encoding":                       "jsonParsed",
			"commitment":                     "confirmed",
			"maxSupportedTransactionVersion": 0,
		}},
	})
	if err != nil {
		return nil, transientErr("marshal rpc request: " + err.Error())
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, v.rpcURL, bytes.NewReader(body))
	if err != nil {
		return nil, transientErr("build rpc request: " + err.Error())
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return nil, transientErr("rpc unreachable: " + err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, transientErr(fmt.Sprintf("rpc returned status %d", resp.StatusCode))
	}

	var envelope rpcEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, transientErr("malformed rpc response: " + err.Error())
	}
	if envelope.Error != nil {
		return nil, transientErr(fmt.Sprintf("rpc error %d: %s", envelope.Error.Code, envelope.Error.Message))
	}
	return envelope.Result, nil
}
