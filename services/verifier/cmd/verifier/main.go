package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
)

const (
	verifyModeMock = "mock"
	verifyModeRPC  = "rpc"

	replayKeyPrefix = "tx-replay:"
)

type config struct {
	Port       string
	RedisURL   string
	VerifyMode string
	RPCURL     string
	Network    string
}

type verifyPayload struct {
	RequestID            string `json:"requestId"`
	TxSignature          string `json:"txSignature"`
	ClientWallet         string `json:"clientWallet"`
	ExpectedRecipient    string `json:"expectedRecipient"`
	ExpectedTokenMint    string `json:"expectedTokenMint"`
	ExpectedAmountAtomic int64  `json:"expectedAmountAtomic"`
	ExpectedNetwork      string `json:"expectedNetwork"`
	ExpectedReference    string `json:"expectedReference,omitempty"`
	ObservedRecipient    string `json:"observedRecipient,omitempty"`
	ObservedTokenMint    string `json:"observedTokenMint,omitempty"`
	ObservedAmountAtomic *int64 `json:"observedAmountAtomic,omitempty"`
	ObservedNetwork      string `json:"observedNetwork,omitempty"`
}

type verifyResponse struct {
	RequestID      string    `json:"requestId"`
	TxSignature    string    `json:"txSignature"`
	ClientWallet   string    `json:"clientWallet"`
	Status         string    `json:"status"`
	Mode           string    `json:"mode"`
	VerifiedAt     time.Time `json:"verifiedAt,omitempty"`
	FailureReason  string    `json:"failureReason,omitempty"`
	FailureMessage string    `json:"failureMessage,omitempty"`
}

type paymentVerifier interface {
	Verify(ctx context.Context, p verifyPayload) (verifyResponse, error)
	mode() string
}

// verifyError distinguishes deterministic rejections (409) from transient
// infrastructure errors (502) so the gateway can retry the latter safely.
type verifyError struct {
	reason    string
	message   string
	transient bool
}

func (e *verifyError) Error() string { return e.message }

func reject(reason, message string) *verifyError {
	return &verifyError{reason: reason, message: message}
}

func transientErr(message string) *verifyError {
	return &verifyError{reason: "rpc_error", message: message, transient: true}
}

func main() {
	cfg := config{
		Port:       getEnv("VERIFIER_PORT", "8081"),
		RedisURL:   getEnv("REDIS_URL", "redis://localhost:6379"),
		VerifyMode: getEnv("SOLANA_VERIFY_MODE", verifyModeMock),
		RPCURL:     getEnv("SOLANA_RPC_URL", "https://api.devnet.solana.com"),
		Network:    getEnv("VERIFIER_NETWORK", "devnet"),
	}

	rdb := redis.NewClient(&redis.Options{Addr: strings.TrimPrefix(cfg.RedisURL, "redis://")})
	verifier := newVerifierFromConfig(cfg, rdb)
	router := newRouter(verifier, rdb)

	addr := ":" + cfg.Port
	log.Printf("verifier listening on %s (mode=%s network=%s)", addr, cfg.VerifyMode, cfg.Network)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatal(err)
	}
}

func newVerifierFromConfig(cfg config, rdb *redis.Client) paymentVerifier {
	if cfg.VerifyMode == verifyModeRPC {
		return &rpcVerifier{
			rpcURL:             cfg.RPCURL,
			network:            cfg.Network,
			rdb:                rdb,
			httpClient:         &http.Client{Timeout: 15 * time.Second},
			notFoundRetryDelay: 2 * time.Second,
		}
	}
	return &mockVerifier{rdb: rdb}
}

func newRouter(v paymentVerifier, rdb *redis.Client) http.Handler {
	mode := v.mode()
	router := chi.NewRouter()

	router.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := rdb.Ping(r.Context()).Err(); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "error", "error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "verifier"})
	})

	router.Post("/verify/solana", func(w http.ResponseWriter, r *http.Request) {
		var payload verifyPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}

		if payload.RequestID == "" || payload.TxSignature == "" || payload.ClientWallet == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "requestId, txSignature, and clientWallet are required"})
			return
		}

		result, err := v.Verify(r.Context(), payload)
		if err != nil {
			failed := verifyResponse{
				RequestID:    payload.RequestID,
				TxSignature:  payload.TxSignature,
				ClientWallet: payload.ClientWallet,
				Status:       "failed",
				Mode:         mode,
			}
			var vErr *verifyError
			if errors.As(err, &vErr) && !vErr.transient {
				failed.FailureReason = vErr.reason
				failed.FailureMessage = vErr.message
				writeJSON(w, http.StatusConflict, failed)
				return
			}
			// Unclassified errors (redis, transport) are transient by definition.
			failed.FailureReason = "rpc_error"
			failed.FailureMessage = err.Error()
			writeJSON(w, http.StatusBadGateway, failed)
			return
		}

		result.Mode = mode
		writeJSON(w, http.StatusOK, result)
	})

	return router
}

type mockVerifier struct {
	rdb *redis.Client
}

func (m *mockVerifier) mode() string { return verifyModeMock }

func (m *mockVerifier) Verify(ctx context.Context, payload verifyPayload) (verifyResponse, error) {
	if payload.TxSignature == "" || strings.EqualFold(payload.TxSignature, "missing") {
		return verifyResponse{}, reject("transaction_not_found", "transaction not found")
	}

	observedRecipient := fallback(payload.ObservedRecipient, payload.ExpectedRecipient)
	observedTokenMint := fallback(payload.ObservedTokenMint, payload.ExpectedTokenMint)
	observedNetwork := fallback(payload.ObservedNetwork, payload.ExpectedNetwork)
	observedAmount := payload.ExpectedAmountAtomic
	if payload.ObservedAmountAtomic != nil {
		observedAmount = *payload.ObservedAmountAtomic
	}

	switch {
	case observedRecipient != payload.ExpectedRecipient:
		return verifyResponse{}, reject("recipient_mismatch", "recipient mismatch")
	case observedTokenMint != payload.ExpectedTokenMint:
		return verifyResponse{}, reject("token_mint_mismatch", "token mint mismatch")
	case observedNetwork != payload.ExpectedNetwork:
		return verifyResponse{}, reject("network_mismatch", "network mismatch")
	case observedAmount < payload.ExpectedAmountAtomic:
		return verifyResponse{}, reject("underpayment", "underpayment")
	}

	// Claim the signature only after every check passes so a failed
	// validation never burns it.
	if err := claimSignature(ctx, m.rdb, payload); err != nil {
		return verifyResponse{}, err
	}

	return verifyResponse{
		RequestID:    payload.RequestID,
		TxSignature:  payload.TxSignature,
		ClientWallet: payload.ClientWallet,
		Status:       "verified",
		VerifiedAt:   time.Now().UTC(),
	}, nil
}

// claimSignature permanently binds a transaction signature to the first
// requestId that redeems it. A retry from the same request is idempotent
// (the gateway may re-verify after losing our 200); only a different
// requestId reusing the signature is a replay.
func claimSignature(ctx context.Context, rdb *redis.Client, payload verifyPayload) error {
	key := replayKeyPrefix + payload.TxSignature
	ok, err := rdb.SetNX(ctx, key, payload.RequestID, 0).Result()
	if err != nil {
		return transientErr("redis error: " + err.Error())
	}
	if ok {
		return nil
	}
	stored, err := rdb.Get(ctx, key).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return transientErr("redis error: " + err.Error())
	}
	if stored == payload.RequestID {
		return nil
	}
	return reject("replay_detected", "transaction already used")
}

func fallback(value, defaultValue string) string {
	if value != "" {
		return value
	}
	return defaultValue
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
