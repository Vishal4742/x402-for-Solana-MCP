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
)

type config struct {
	Port       string
	RedisURL   string
	VerifyMode string
}

type verifyPayload struct {
	RequestID            string `json:"requestId"`
	TxSignature          string `json:"txSignature"`
	ClientWallet         string `json:"clientWallet"`
	ExpectedRecipient    string `json:"expectedRecipient"`
	ExpectedTokenMint    string `json:"expectedTokenMint"`
	ExpectedAmountAtomic int64  `json:"expectedAmountAtomic"`
	ExpectedNetwork      string `json:"expectedNetwork"`
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
	VerifiedAt     time.Time `json:"verifiedAt,omitempty"`
	FailureReason  string    `json:"failureReason,omitempty"`
	FailureMessage string    `json:"failureMessage,omitempty"`
}

func main() {
	cfg := config{
		Port:       getEnv("VERIFIER_PORT", "8081"),
		RedisURL:   getEnv("REDIS_URL", "redis://localhost:6379"),
		VerifyMode: getEnv("SOLANA_VERIFY_MODE", verifyModeMock),
	}

	rdb := redis.NewClient(&redis.Options{Addr: strings.TrimPrefix(cfg.RedisURL, "redis://")})
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

		result, err := verifyMock(r.Context(), rdb, payload)
		if err != nil {
			writeJSON(w, http.StatusConflict, verifyResponse{
				RequestID:      payload.RequestID,
				TxSignature:    payload.TxSignature,
				ClientWallet:   payload.ClientWallet,
				Status:         "failed",
				FailureReason:  classifyFailure(err),
				FailureMessage: err.Error(),
			})
			return
		}

		writeJSON(w, http.StatusOK, result)
	})

	addr := ":" + cfg.Port
	log.Printf("verifier listening on %s", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatal(err)
	}
}

func verifyMock(ctx context.Context, rdb *redis.Client, payload verifyPayload) (verifyResponse, error) {
	if payload.TxSignature == "" || strings.EqualFold(payload.TxSignature, "missing") {
		return verifyResponse{}, errors.New("transaction not found")
	}

	replayKey := "tx-replay:" + payload.TxSignature
	ok, err := rdb.SetNX(ctx, replayKey, payload.RequestID, 24*time.Hour).Result()
	if err != nil {
		return verifyResponse{}, err
	}
	if !ok {
		return verifyResponse{}, errors.New("transaction already used")
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
		return verifyResponse{}, errors.New("recipient mismatch")
	case observedTokenMint != payload.ExpectedTokenMint:
		return verifyResponse{}, errors.New("token mint mismatch")
	case observedNetwork != payload.ExpectedNetwork:
		return verifyResponse{}, errors.New("network mismatch")
	case observedAmount < payload.ExpectedAmountAtomic:
		return verifyResponse{}, errors.New("underpayment")
	}

	return verifyResponse{
		RequestID:    payload.RequestID,
		TxSignature:  payload.TxSignature,
		ClientWallet: payload.ClientWallet,
		Status:       "verified",
		VerifiedAt:   time.Now().UTC(),
	}, nil
}

func classifyFailure(err error) string {
	switch err.Error() {
	case "transaction not found":
		return "transaction_not_found"
	case "transaction already used":
		return "replay_detected"
	case "recipient mismatch":
		return "recipient_mismatch"
	case "token mint mismatch":
		return "token_mint_mismatch"
	case "network mismatch":
		return "network_mismatch"
	case "underpayment":
		return "underpayment"
	default:
		return "verification_failed"
	}
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
