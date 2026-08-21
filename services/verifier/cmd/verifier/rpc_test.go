package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

const (
	tokenProgram  = "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"
	otherMint     = "OtherMint1111111111111111111111111111111111"
	otherOwner    = "OtherOwner111111111111111111111111111111111"
	testReference = "Referenc3Key1111111111111111111111111111111"
	testBlockTime = int64(1755640000)
)

// tb is a compact token-balance fixture entry.
type tb struct {
	idx    int
	mint   string
	owner  string
	amount string
}

func tokenBalanceJSON(b tb) map[string]any {
	return map[string]any{
		"accountIndex": b.idx,
		"mint":         b.mint,
		"owner":        b.owner,
		"programId":    tokenProgram,
		"uiTokenAmount": map[string]any{
			"amount":         b.amount,
			"decimals":       6,
			"uiAmountString": b.amount,
		},
	}
}

func keyJSON(pubkey string, signer bool) map[string]any {
	return map[string]any{"pubkey": pubkey, "signer": signer, "writable": true, "source": "transaction"}
}

func defaultKeys() []map[string]any {
	return []map[string]any{
		keyJSON(testWallet, true),
		keyJSON(testRecipient, false),
		{"pubkey": tokenProgram, "signer": false, "writable": false, "source": "transaction"},
	}
}

// txResultJSON mirrors the jsonParsed getTransaction result shape.
func txResultJSON(keys []map[string]any, metaErr any, blockTime any, pre, post []tb) map[string]any {
	pres := make([]any, 0, len(pre))
	for _, b := range pre {
		pres = append(pres, tokenBalanceJSON(b))
	}
	posts := make([]any, 0, len(post))
	for _, b := range post {
		posts = append(posts, tokenBalanceJSON(b))
	}
	return map[string]any{
		"blockTime": blockTime,
		"slot":      394812345,
		"meta": map[string]any{
			"err":                  metaErr,
			"fee":                  5000,
			"computeUnitsConsumed": 4650,
			"preTokenBalances":     pres,
			"postTokenBalances":    posts,
			"logMessages":          []string{"Program " + tokenProgram + " invoke [1]"},
		},
		"transaction": map[string]any{
			"message": map[string]any{
				"accountKeys": keys,
				"instructions": []any{
					map[string]any{"programId": tokenProgram, "parsed": map[string]any{"type": "transferChecked"}},
				},
			},
			"signatures": []string{basePayload().TxSignature},
		},
	}
}

// happyResult moves exactly testAmount to the recipient's single ATA.
func happyResult() map[string]any {
	return txResultJSON(defaultKeys(), nil, testBlockTime,
		[]tb{{idx: 2, mint: testMint, owner: testRecipient, amount: "5000"}},
		[]tb{{idx: 2, mint: testMint, owner: testRecipient, amount: "15000"}})
}

type rpcEnv struct {
	server *httptest.Server
	redis  *miniredis.Miniredis
	calls  *int32
}

// newRPCEnv boots a verifier in rpc mode against a fake Solana RPC that
// serves `result` for every getTransaction call.
func newRPCEnv(t *testing.T, result any) rpcEnv {
	t.Helper()
	return newRPCEnvHandler(t, func(w http.ResponseWriter, r *http.Request) {
		var req rpcRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("bad rpc request: %v", err)
		}
		if req.Method != "getTransaction" {
			t.Errorf("rpc method = %q, want getTransaction", req.Method)
		}
		writeJSON(w, http.StatusOK, map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
	})
}

func newRPCEnvHandler(t *testing.T, handler http.HandlerFunc) rpcEnv {
	t.Helper()
	calls := new(int32)
	rpc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(calls, 1)
		handler(w, r)
	}))
	t.Cleanup(rpc.Close)

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	v := &rpcVerifier{
		rpcURL:             rpc.URL,
		network:            "devnet",
		rdb:                rdb,
		httpClient:         &http.Client{Timeout: 5 * time.Second},
		notFoundRetryDelay: 5 * time.Millisecond,
	}
	ts := httptest.NewServer(newRouter(v, rdb))
	t.Cleanup(ts.Close)
	return rpcEnv{server: ts, redis: mr, calls: calls}
}

func TestRPCHappyPath(t *testing.T) {
	env := newRPCEnv(t, happyResult())

	status, resp := postVerify(t, env.server.URL, basePayload())
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200 (resp %+v)", status, resp)
	}
	if resp.Status != "verified" || resp.Mode != "rpc" {
		t.Errorf("Status/Mode = %q/%q, want verified/rpc", resp.Status, resp.Mode)
	}
	if want := time.Unix(testBlockTime, 0).UTC(); !resp.VerifiedAt.Equal(want) {
		t.Errorf("VerifiedAt = %s, want blockTime %s", resp.VerifiedAt, want)
	}
	if !env.redis.Exists(replayKeyPrefix + basePayload().TxSignature) {
		t.Error("replay key not claimed after successful verify")
	}
}

func TestRPCOverpayment(t *testing.T) {
	result := txResultJSON(defaultKeys(), nil, testBlockTime,
		[]tb{{idx: 2, mint: testMint, owner: testRecipient, amount: "0"}},
		[]tb{{idx: 2, mint: testMint, owner: testRecipient, amount: "999999"}})
	env := newRPCEnv(t, result)

	status, resp := postVerify(t, env.server.URL, basePayload())
	if status != http.StatusOK || resp.Status != "verified" {
		t.Fatalf("overpayment = %d/%q, want 200/verified", status, resp.Status)
	}
}

func TestRPCUnderpayment(t *testing.T) {
	result := txResultJSON(defaultKeys(), nil, testBlockTime,
		[]tb{{idx: 2, mint: testMint, owner: testRecipient, amount: "0"}},
		[]tb{{idx: 2, mint: testMint, owner: testRecipient, amount: "9999"}})
	env := newRPCEnv(t, result)

	status, resp := postVerify(t, env.server.URL, basePayload())
	if status != http.StatusConflict {
		t.Fatalf("status = %d, want 409", status)
	}
	if resp.FailureReason != "underpayment" {
		t.Errorf("FailureReason = %q, want underpayment", resp.FailureReason)
	}
	if env.redis.Exists(replayKeyPrefix + basePayload().TxSignature) {
		t.Error("underpayment burned the signature")
	}
}

func TestRPCRecipientMismatch(t *testing.T) {
	// The expected mint moved, but to a different owner.
	result := txResultJSON(defaultKeys(), nil, testBlockTime,
		[]tb{{idx: 2, mint: testMint, owner: otherOwner, amount: "0"}},
		[]tb{{idx: 2, mint: testMint, owner: otherOwner, amount: "10000"}})
	env := newRPCEnv(t, result)

	status, resp := postVerify(t, env.server.URL, basePayload())
	if status != http.StatusConflict || resp.FailureReason != "recipient_mismatch" {
		t.Fatalf("got %d/%q, want 409/recipient_mismatch", status, resp.FailureReason)
	}
}

func TestRPCMintMismatch(t *testing.T) {
	// Only a different mint moved (even to the right recipient).
	result := txResultJSON(defaultKeys(), nil, testBlockTime,
		[]tb{{idx: 2, mint: otherMint, owner: testRecipient, amount: "0"}},
		[]tb{{idx: 2, mint: otherMint, owner: testRecipient, amount: "10000"}})
	env := newRPCEnv(t, result)

	status, resp := postVerify(t, env.server.URL, basePayload())
	if status != http.StatusConflict || resp.FailureReason != "token_mint_mismatch" {
		t.Fatalf("got %d/%q, want 409/token_mint_mismatch", status, resp.FailureReason)
	}
}

// Documents the classification default: a tx with no token movement at all is
// reported as recipient_mismatch.
func TestRPCNoTokenMovement(t *testing.T) {
	result := txResultJSON(defaultKeys(), nil, testBlockTime, nil, nil)
	env := newRPCEnv(t, result)

	status, resp := postVerify(t, env.server.URL, basePayload())
	if status != http.StatusConflict || resp.FailureReason != "recipient_mismatch" {
		t.Fatalf("got %d/%q, want 409/recipient_mismatch", status, resp.FailureReason)
	}
}

func TestRPCTransactionNotFound(t *testing.T) {
	env := newRPCEnv(t, nil)

	status, resp := postVerify(t, env.server.URL, basePayload())
	if status != http.StatusConflict {
		t.Fatalf("status = %d, want 409", status)
	}
	if resp.FailureReason != "transaction_not_found" {
		t.Errorf("FailureReason = %q, want transaction_not_found", resp.FailureReason)
	}
	if got := atomic.LoadInt32(env.calls); got != 2 {
		t.Errorf("rpc calls = %d, want 2 (one retry for confirmation lag)", got)
	}
}

func TestRPCTransactionFailed(t *testing.T) {
	result := txResultJSON(defaultKeys(), map[string]any{"InstructionError": []any{0, map[string]any{"Custom": 1}}},
		testBlockTime,
		[]tb{{idx: 2, mint: testMint, owner: testRecipient, amount: "5000"}},
		[]tb{{idx: 2, mint: testMint, owner: testRecipient, amount: "5000"}})
	env := newRPCEnv(t, result)

	status, resp := postVerify(t, env.server.URL, basePayload())
	if status != http.StatusConflict || resp.FailureReason != "transaction_failed" {
		t.Fatalf("got %d/%q, want 409/transaction_failed", status, resp.FailureReason)
	}
}

func TestRPCPayerNotSigner(t *testing.T) {
	keys := []map[string]any{
		keyJSON("FeePayerSomeoneE1se1111111111111111111111111", true),
		keyJSON(testWallet, false), // present but not a signer
		keyJSON(testRecipient, false),
	}
	result := txResultJSON(keys, nil, testBlockTime,
		[]tb{{idx: 2, mint: testMint, owner: testRecipient, amount: "0"}},
		[]tb{{idx: 2, mint: testMint, owner: testRecipient, amount: "10000"}})
	env := newRPCEnv(t, result)

	status, resp := postVerify(t, env.server.URL, basePayload())
	if status != http.StatusConflict || resp.FailureReason != "payer_mismatch" {
		t.Fatalf("got %d/%q, want 409/payer_mismatch", status, resp.FailureReason)
	}
}

func TestRPCServerError(t *testing.T) {
	env := newRPCEnvHandler(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	})

	status, resp := postVerify(t, env.server.URL, basePayload())
	if status != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", status)
	}
	if resp.FailureReason != "rpc_error" || resp.Status != "failed" || resp.Mode != "rpc" {
		t.Errorf("got %q/%q/%q, want rpc_error/failed/rpc", resp.FailureReason, resp.Status, resp.Mode)
	}
}

func TestRPCJSONRPCError(t *testing.T) {
	env := newRPCEnvHandler(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"jsonrpc": "2.0", "id": 1,
			"error": map[string]any{"code": -32005, "message": "node is behind"},
		})
	})

	status, resp := postVerify(t, env.server.URL, basePayload())
	if status != http.StatusBadGateway || resp.FailureReason != "rpc_error" {
		t.Fatalf("got %d/%q, want 502/rpc_error", status, resp.FailureReason)
	}
}

func TestRPCNetworkMismatch(t *testing.T) {
	env := newRPCEnv(t, happyResult())

	p := basePayload()
	p.ExpectedNetwork = "mainnet"
	status, resp := postVerify(t, env.server.URL, p)
	if status != http.StatusConflict || resp.FailureReason != "network_mismatch" {
		t.Fatalf("got %d/%q, want 409/network_mismatch", status, resp.FailureReason)
	}
	if got := atomic.LoadInt32(env.calls); got != 0 {
		t.Errorf("rpc calls = %d, want 0 (network check precedes rpc)", got)
	}
}

func TestRPCReplay(t *testing.T) {
	env := newRPCEnv(t, happyResult())

	if status, _ := postVerify(t, env.server.URL, basePayload()); status != http.StatusOK {
		t.Fatalf("first verify status = %d, want 200", status)
	}
	second := basePayload()
	second.RequestID = "req_01HOTHER"
	status, resp := postVerify(t, env.server.URL, second)
	if status != http.StatusConflict || resp.FailureReason != "replay_detected" {
		t.Fatalf("second verify = %d/%q, want 409/replay_detected", status, resp.FailureReason)
	}
}

// The same request re-verifying its own signature is idempotent in rpc mode.
func TestRPCIdempotentSameRequest(t *testing.T) {
	env := newRPCEnv(t, happyResult())

	if status, _ := postVerify(t, env.server.URL, basePayload()); status != http.StatusOK {
		t.Fatalf("first verify status = %d, want 200", status)
	}
	status, resp := postVerify(t, env.server.URL, basePayload())
	if status != http.StatusOK || resp.Status != "verified" {
		t.Fatalf("retry with same requestId = %d/%q, want 200/verified", status, resp.Status)
	}
}

// A transfer carrying the challenge reference as an extra account key passes
// the reference check.
func TestRPCReferencePresent(t *testing.T) {
	keys := append(defaultKeys(), keyJSON(testReference, false))
	result := txResultJSON(keys, nil, testBlockTime,
		[]tb{{idx: 2, mint: testMint, owner: testRecipient, amount: "5000"}},
		[]tb{{idx: 2, mint: testMint, owner: testRecipient, amount: "15000"}})
	env := newRPCEnv(t, result)

	p := basePayload()
	p.ExpectedReference = testReference
	status, resp := postVerify(t, env.server.URL, p)
	if status != http.StatusOK || resp.Status != "verified" {
		t.Fatalf("reference present = %d/%q, want 200/verified", status, resp.Status)
	}
}

// A transfer that does not carry the challenge reference is rejected before
// the signature is claimed.
func TestRPCReferenceAbsent(t *testing.T) {
	env := newRPCEnv(t, happyResult())

	p := basePayload()
	p.ExpectedReference = testReference
	status, resp := postVerify(t, env.server.URL, p)
	if status != http.StatusConflict || resp.FailureReason != "reference_mismatch" {
		t.Fatalf("reference absent = %d/%q, want 409/reference_mismatch", status, resp.FailureReason)
	}
	if env.redis.Exists(replayKeyPrefix + p.TxSignature) {
		t.Error("reference mismatch burned the signature")
	}
}

// An empty expectedReference skips the check (mock mode and direct callers).
func TestRPCReferenceEmptySkipped(t *testing.T) {
	env := newRPCEnv(t, happyResult())

	p := basePayload()
	p.ExpectedReference = ""
	status, resp := postVerify(t, env.server.URL, p)
	if status != http.StatusOK || resp.Status != "verified" {
		t.Fatalf("empty reference = %d/%q, want 200/verified", status, resp.Status)
	}
}

// Pins the getTransaction request construction: params[0] is the signature
// and the options ask for jsonParsed encoding.
func TestRPCGetTransactionRequestShape(t *testing.T) {
	env := newRPCEnvHandler(t, func(w http.ResponseWriter, r *http.Request) {
		var req rpcRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("bad rpc request: %v", err)
		}
		if req.JSONRPC != "2.0" || req.Method != "getTransaction" {
			t.Errorf("jsonrpc/method = %q/%q, want 2.0/getTransaction", req.JSONRPC, req.Method)
		}
		if len(req.Params) != 2 {
			t.Errorf("params length = %d, want 2", len(req.Params))
		} else {
			if sig, _ := req.Params[0].(string); sig != basePayload().TxSignature {
				t.Errorf("params[0] = %q, want tx signature %q", sig, basePayload().TxSignature)
			}
			opts, _ := req.Params[1].(map[string]any)
			if opts["encoding"] != "jsonParsed" {
				t.Errorf("encoding = %v, want jsonParsed", opts["encoding"])
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": happyResult()})
	})

	status, resp := postVerify(t, env.server.URL, basePayload())
	if status != http.StatusOK || resp.Status != "verified" {
		t.Fatalf("status = %d/%q, want 200/verified", status, resp.Status)
	}
}

// A recipient with two ATAs for the mint: deltas are summed across accounts.
func TestRPCMultiATASum(t *testing.T) {
	result := txResultJSON(defaultKeys(), nil, testBlockTime,
		[]tb{
			{idx: 2, mint: testMint, owner: testRecipient, amount: "1000"},
			{idx: 3, mint: testMint, owner: testRecipient, amount: "500"},
		},
		[]tb{
			{idx: 2, mint: testMint, owner: testRecipient, amount: "7000"}, // +6000
			{idx: 3, mint: testMint, owner: testRecipient, amount: "4500"}, // +4000
		})
	env := newRPCEnv(t, result)

	status, resp := postVerify(t, env.server.URL, basePayload())
	if status != http.StatusOK || resp.Status != "verified" {
		t.Fatalf("multi-ATA sum = %d/%q, want 200/verified", status, resp.Status)
	}
}

// An ATA created by the transaction has no preTokenBalances entry; the
// missing pre balance counts as 0.
func TestRPCMissingPreTreatedAsZero(t *testing.T) {
	result := txResultJSON(defaultKeys(), nil, testBlockTime,
		nil,
		[]tb{{idx: 2, mint: testMint, owner: testRecipient, amount: "10000"}})
	env := newRPCEnv(t, result)

	status, resp := postVerify(t, env.server.URL, basePayload())
	if status != http.StatusOK || resp.Status != "verified" {
		t.Fatalf("missing pre = %d/%q, want 200/verified", status, resp.Status)
	}
}

// blockTime can be null on some RPC responses; verifiedAt falls back to now.
func TestRPCNullBlockTime(t *testing.T) {
	result := txResultJSON(defaultKeys(), nil, nil,
		nil,
		[]tb{{idx: 2, mint: testMint, owner: testRecipient, amount: "10000"}})
	env := newRPCEnv(t, result)

	status, resp := postVerify(t, env.server.URL, basePayload())
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	if resp.VerifiedAt.IsZero() {
		t.Error("VerifiedAt should fall back to now when blockTime is null")
	}
}
