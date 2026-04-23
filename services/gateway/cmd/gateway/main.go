package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	defaultTokenMint        = "4zMMC9srt5Ri5X14GAgXhaHii3GnPAEERYPJgZJDncDU"
	defaultNetwork          = "devnet"
	defaultChallengeTTL     = 300 * time.Second
	defaultVerifyLockTTL    = 30 * time.Second
	defaultVerifierURL      = "http://localhost:8081"
	defaultServerID         = "srv_01HX3K"
	defaultServerName       = "agent-tools-prod"
	defaultPaymentScheme    = "x402.sol.usdc.v1"
	defaultServerAPIKey     = "x402_sk_dev_local"
	requestStatusChallenged = "pending"
	requestStatusVerified   = "verified"
	requestStatusExecuted   = "executed"
	requestStatusFailed     = "failed"
)

type toolSeed struct {
	PriceAtomic int64  `json:"priceAtomic"`
	Description string `json:"description,omitempty"`
	Enabled     *bool  `json:"enabled,omitempty"`
}

type config struct {
	Port               string
	SellerWallet       string
	TokenMint          string
	Network            string
	ChallengeTTL       time.Duration
	DefaultPriceAtomic int64
	PostgresURL        string
	RedisURL           string
	VerifierBaseURL    string
	VerifyLockTTL      time.Duration
	ServerID           string
	ServerName         string
	ToolSeeds          map[string]toolSeed
}

type app struct {
	cfg        config
	store      *store
	redis      *redis.Client
	httpClient *http.Client
}

type mcpRequest struct {
	Tool  string         `json:"tool"`
	Input map[string]any `json:"input"`
}

type verifyRequest struct {
	RequestID    string `json:"requestId"`
	TxSignature  string `json:"txSignature"`
	ClientWallet string `json:"clientWallet"`
	TokenMint    string `json:"tokenMint,omitempty"`
	Recipient    string `json:"recipient,omitempty"`
	Network      string `json:"network,omitempty"`
	AmountAtomic *int64 `json:"amountAtomic,omitempty"`
}

type verifierRequest struct {
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

type verifierResponse struct {
	RequestID      string    `json:"requestId"`
	TxSignature    string    `json:"txSignature"`
	ClientWallet   string    `json:"clientWallet"`
	Status         string    `json:"status"`
	VerifiedAt     time.Time `json:"verifiedAt"`
	FailureReason  string    `json:"failureReason,omitempty"`
	FailureMessage string    `json:"failureMessage,omitempty"`
}

type challengeResponse struct {
	Error     string        `json:"error"`
	Message   string        `json:"message"`
	Challenge challengeView `json:"challenge"`
	Pricing   pricingMeta   `json:"pricing"`
	Retry     retryMeta     `json:"retry"`
}

type challengeView struct {
	RequestID string     `json:"requestId"`
	ServerID  string     `json:"serverId"`
	ToolName  string     `json:"toolName"`
	Amount    int64      `json:"amountAtomic"`
	TokenMint string     `json:"tokenMint"`
	Recipient string     `json:"recipient"`
	Network   string     `json:"network"`
	Scheme    string     `json:"scheme"`
	Status    string     `json:"status"`
	CreatedAt time.Time  `json:"createdAt"`
	ExpiresAt time.Time  `json:"expiresAt"`
	Settled   bool       `json:"settled"`
	SettledAt *time.Time `json:"settledAt,omitempty"`
}

type pricingMeta struct {
	Scheme       string `json:"scheme"`
	AmountAtomic int64  `json:"amountAtomic"`
	TokenMint    string `json:"tokenMint"`
	Recipient    string `json:"recipient"`
	Network      string `json:"network"`
}

type retryMeta struct {
	RequestID      string `json:"requestId"`
	RequestIDField string `json:"requestIdField"`
	Header         string `json:"header"`
}

type executionResponse struct {
	ServerID  string `json:"serverId"`
	Tool      string `json:"tool"`
	Status    string `json:"status"`
	RequestID string `json:"requestId,omitempty"`
	Result    string `json:"result"`
}

func main() {
	cfg, err := loadConfig()
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	store, err := newStore(ctx, cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer store.close()

	if err := store.initSchema(ctx); err != nil {
		log.Fatal(err)
	}
	if err := store.seedDefaults(ctx, cfg); err != nil {
		log.Fatal(err)
	}

	rdb := redis.NewClient(&redis.Options{Addr: strings.TrimPrefix(cfg.RedisURL, "redis://")})
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatal(err)
	}

	app := &app{
		cfg:   cfg,
		store: store,
		redis: rdb,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	router := chi.NewRouter()
	router.Get("/healthz", app.handleHealth)
	router.Post("/mcp/{serverID}", app.handleInvokeMCP)
	router.Get("/v1/challenge/{requestID}", app.handleGetChallenge)
	router.Post("/v1/verify", app.handleVerify)
	router.Get("/v1/servers", app.handleGetServers)
	router.Get("/v1/servers/{serverID}/tools", app.handleGetTools)
	router.Get("/v1/requests", app.handleGetRequests)
	router.Get("/v1/requests/{requestID}", app.handleGetRequest)
	router.Get("/v1/dashboard/summary", app.handleGetDashboardSummary)
	router.Get("/v1/dashboard/receipts", app.handleGetReceipts)

	addr := ":" + cfg.Port
	log.Printf("gateway listening on %s", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatal(err)
	}
}

func (a *app) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	if err := a.store.ping(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "error", "error": err.Error()})
		return
	}
	if err := a.redis.Ping(ctx).Err(); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "error", "error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "gateway"})
}

func (a *app) handleInvokeMCP(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "serverID")

	var req mcpRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if req.Tool == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "tool is required"})
		return
	}

	ctx := r.Context()
	requestID := r.Header.Get("X-Payment-Request-Id")
	if requestID != "" {
		record, err := a.store.getPaymentRequest(ctx, requestID)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown request id"})
			return
		}
		if record.ServerID != serverID || record.ToolName != req.Tool {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "request id does not match this tool invocation"})
			return
		}
		if time.Now().UTC().After(record.ExpiresAt) && record.Status == requestStatusChallenged {
			if err := a.store.markFailed(ctx, record.ID, "timeout", map[string]any{"code": 402, "reason": "challenge_expired"}); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			record.Status = requestStatusFailed
			record.FailureReason = strPtr("timeout")
		}

		switch record.Status {
		case requestStatusVerified, requestStatusExecuted:
			if record.Status == requestStatusExecuted {
				writeJSON(w, http.StatusOK, executionResponse{
					ServerID:  serverID,
					Tool:      req.Tool,
					Status:    "executed",
					RequestID: requestID,
					Result:    "paid tool executed after settlement",
				})
				return
			}

			result := map[string]any{"result": "paid tool executed after settlement"}
			if err := a.store.markExecuted(ctx, record.ID, result); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusOK, executionResponse{
				ServerID:  serverID,
				Tool:      req.Tool,
				Status:    "executed",
				RequestID: requestID,
				Result:    "paid tool executed after settlement",
			})
			return
		case requestStatusChallenged:
			writeJSON(w, http.StatusPaymentRequired, newChallengeResponse(record))
			return
		case requestStatusFailed:
			writeJSON(w, http.StatusConflict, map[string]string{"error": "payment request failed; create a new challenge"})
			return
		default:
			writeJSON(w, http.StatusConflict, map[string]string{"error": "request is not ready for execution"})
			return
		}
	}

	tool, err := a.store.getToolPricingByName(ctx, serverID, req.Tool)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "tool not configured"})
		return
	}
	toolEnabled := tool.Enabled == nil || *tool.Enabled
	if !toolEnabled {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "tool disabled"})
		return
	}
	if tool.PriceAtomic == 0 {
		writeJSON(w, http.StatusOK, executionResponse{
			ServerID: serverID,
			Tool:     req.Tool,
			Status:   "executed",
			Result:   "free tool executed",
		})
		return
	}

	record, err := a.store.createPaymentRequest(ctx, createPaymentRequestInput{
		ID:           uuid.NewString(),
		ServerID:     serverID,
		ToolName:     req.Tool,
		AmountAtomic: tool.PriceAtomic,
		TokenMint:    a.cfg.TokenMint,
		Recipient:    a.cfg.SellerWallet,
		Network:      a.cfg.Network,
		Scheme:       defaultPaymentScheme,
		ExpiresAt:    time.Now().UTC().Add(a.cfg.ChallengeTTL),
		RawRequest:   req,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusPaymentRequired, newChallengeResponse(record))
}

func (a *app) handleGetChallenge(w http.ResponseWriter, r *http.Request) {
	record, err := a.store.getPaymentRequest(r.Context(), chi.URLParam(r, "requestID"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "challenge not found"})
		return
	}
	writeJSON(w, http.StatusOK, challengeToView(record))
}

func (a *app) handleVerify(w http.ResponseWriter, r *http.Request) {
	var req verifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if req.RequestID == "" || req.TxSignature == "" || req.ClientWallet == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "requestId, txSignature, and clientWallet are required"})
		return
	}

	ctx := r.Context()
	record, err := a.store.getPaymentRequest(ctx, req.RequestID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown request"})
		return
	}
	if record.Status == requestStatusVerified || record.Status == requestStatusExecuted {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "request already verified"})
		return
	}
	if record.Status == requestStatusFailed {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "request already failed"})
		return
	}
	if time.Now().UTC().After(record.ExpiresAt) {
		if err := a.store.markFailed(ctx, record.ID, "timeout", map[string]any{"code": 402, "reason": "challenge_expired"}); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusConflict, map[string]string{"error": "challenge expired"})
		return
	}

	lockKey := fmt.Sprintf("verify-lock:%s", req.RequestID)
	acquired, err := a.redis.SetNX(ctx, lockKey, "1", a.cfg.VerifyLockTTL).Result()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if !acquired {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "verification already in progress"})
		return
	}
	defer a.redis.Del(context.Background(), lockKey)

	verifyPayload := verifierRequest{
		RequestID:            req.RequestID,
		TxSignature:          req.TxSignature,
		ClientWallet:         req.ClientWallet,
		ExpectedRecipient:    record.Recipient,
		ExpectedTokenMint:    record.TokenMint,
		ExpectedAmountAtomic: record.AmountAtomic,
		ExpectedNetwork:      record.Network,
		ObservedRecipient:    req.Recipient,
		ObservedTokenMint:    req.TokenMint,
		ObservedAmountAtomic: req.AmountAtomic,
		ObservedNetwork:      req.Network,
	}

	result, err := a.callVerifier(ctx, verifyPayload)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	if result.Status != "verified" {
		reason := mapFailureReason(result.FailureReason)
		if markErr := a.store.markFailed(ctx, record.ID, reason, map[string]any{
			"txSignature": req.TxSignature,
			"message":     result.FailureMessage,
			"reason":      result.FailureReason,
		}); markErr != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": markErr.Error()})
			return
		}
		writeJSON(w, http.StatusConflict, map[string]string{"error": result.FailureMessage})
		return
	}

	if err := a.store.markVerified(ctx, record.ID, req.TxSignature, req.ClientWallet, result.VerifiedAt); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"requestId":    req.RequestID,
		"txSignature":  req.TxSignature,
		"clientWallet": req.ClientWallet,
		"status":       requestStatusVerified,
		"settledAt":    result.VerifiedAt.UTC(),
	})
}

func (a *app) handleGetServers(w http.ResponseWriter, r *http.Request) {
	items, err := a.store.listServers(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (a *app) handleGetTools(w http.ResponseWriter, r *http.Request) {
	items, err := a.store.listTools(r.Context(), chi.URLParam(r, "serverID"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (a *app) handleGetRequests(w http.ResponseWriter, r *http.Request) {
	items, err := a.store.listRequests(r.Context(), r.URL.Query().Get("serverId"), r.URL.Query().Get("status"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (a *app) handleGetRequest(w http.ResponseWriter, r *http.Request) {
	item, err := a.store.getRequestView(r.Context(), chi.URLParam(r, "requestID"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "request not found"})
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (a *app) handleGetDashboardSummary(w http.ResponseWriter, r *http.Request) {
	item, err := a.store.getDashboardSummary(r.Context(), r.URL.Query().Get("serverId"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (a *app) handleGetReceipts(w http.ResponseWriter, r *http.Request) {
	items, err := a.store.listReceipts(r.Context(), r.URL.Query().Get("serverId"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (a *app) callVerifier(ctx context.Context, payload verifierRequest) (verifierResponse, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return verifierResponse{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(a.cfg.VerifierBaseURL, "/")+"/verify/solana", bytes.NewReader(body))
	if err != nil {
		return verifierResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return verifierResponse{}, err
	}
	defer resp.Body.Close()

	var result verifierResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return verifierResponse{}, err
	}
	if resp.StatusCode >= 500 {
		return verifierResponse{}, fmt.Errorf("%s", firstNonEmpty(result.FailureMessage, resp.Status))
	}
	if result.Status == "" {
		result.Status = "failed"
	}
	return result, nil
}

func loadConfig() (config, error) {
	cfg := config{
		Port:               getEnv("GATEWAY_PORT", "8080"),
		SellerWallet:       getEnv("SELLER_WALLET", "ReplaceMeWithDevnetSellerWallet"),
		TokenMint:          getEnv("SOLANA_USDC_MINT", defaultTokenMint),
		Network:            getEnv("GATEWAY_NETWORK", defaultNetwork),
		ChallengeTTL:       defaultChallengeTTL,
		DefaultPriceAtomic: getEnvInt64("DEFAULT_TOOL_PRICE_ATOMIC", 0),
		PostgresURL:        getEnv("POSTGRES_URL", "postgres://postgres:postgres@localhost:5432/x402?sslmode=disable"),
		RedisURL:           getEnv("REDIS_URL", "redis://localhost:6379"),
		VerifierBaseURL:    getEnv("VERIFIER_BASE_URL", defaultVerifierURL),
		VerifyLockTTL:      defaultVerifyLockTTL,
		ServerID:           getEnv("DEFAULT_SERVER_ID", defaultServerID),
		ServerName:         getEnv("DEFAULT_SERVER_NAME", defaultServerName),
		ToolSeeds: map[string]toolSeed{
			"ping":            {PriceAtomic: 0, Description: "Healthcheck endpoint.", Enabled: boolPtr(true)},
			"premium.search":  {PriceAtomic: 1000000, Description: "Paid MCP search tool.", Enabled: boolPtr(true)},
			"premium.codegen": {PriceAtomic: 2500000, Description: "Paid code generation tool.", Enabled: boolPtr(true)},
		},
	}

	if ttl := os.Getenv("PAYMENT_CHALLENGE_TTL_SECONDS"); ttl != "" {
		seconds, err := strconv.Atoi(ttl)
		if err != nil || seconds <= 0 {
			return config{}, errors.New("PAYMENT_CHALLENGE_TTL_SECONDS must be a positive integer")
		}
		cfg.ChallengeTTL = time.Duration(seconds) * time.Second
	}
	if ttl := os.Getenv("VERIFY_LOCK_TTL_SECONDS"); ttl != "" {
		seconds, err := strconv.Atoi(ttl)
		if err != nil || seconds <= 0 {
			return config{}, errors.New("VERIFY_LOCK_TTL_SECONDS must be a positive integer")
		}
		cfg.VerifyLockTTL = time.Duration(seconds) * time.Second
	}

	if raw := strings.TrimSpace(os.Getenv("TOOL_PRICING_JSON")); raw != "" {
		var parsed map[string]toolSeed
		if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
			return config{}, errors.New("TOOL_PRICING_JSON must be valid JSON: " + err.Error())
		}
		for toolName, tool := range parsed {
			if tool.Enabled == nil {
				tool.Enabled = boolPtr(true)
			}
			cfg.ToolSeeds[toolName] = tool
		}
	}

	return cfg, nil
}

func newChallengeResponse(record paymentRequestRecord) challengeResponse {
	view := challengeToView(record)
	return challengeResponse{
		Error:     "payment_required",
		Message:   "tool requires payment before execution",
		Challenge: view,
		Pricing: pricingMeta{
			Scheme:       record.Scheme,
			AmountAtomic: record.AmountAtomic,
			TokenMint:    record.TokenMint,
			Recipient:    record.Recipient,
			Network:      record.Network,
		},
		Retry: retryMeta{
			RequestID:      record.ID,
			RequestIDField: "requestId",
			Header:         "X-Payment-Request-Id",
		},
	}
}

func challengeToView(record paymentRequestRecord) challengeView {
	settled := record.Status == requestStatusVerified || record.Status == requestStatusExecuted
	return challengeView{
		RequestID: record.ID,
		ServerID:  record.ServerID,
		ToolName:  record.ToolName,
		Amount:    record.AmountAtomic,
		TokenMint: record.TokenMint,
		Recipient: record.Recipient,
		Network:   record.Network,
		Scheme:    record.Scheme,
		Status:    record.Status,
		CreatedAt: record.CreatedAt,
		ExpiresAt: record.ExpiresAt,
		Settled:   settled,
		SettledAt: record.SettledAt,
	}
}

func mapFailureReason(reason string) string {
	if strings.TrimSpace(reason) == "" {
		return "verification_failed"
	}
	return reason
}

func stringifyMessage(payload map[string]any, fallback string) string {
	if payload == nil {
		return fallback
	}
	if value, ok := payload["error"].(string); ok && value != "" {
		return value
	}
	if value, ok := payload["message"].(string); ok && value != "" {
		return value
	}
	return fallback
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
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

func getEnvInt64(key string, fallback int64) int64 {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func strPtr(value string) *string {
	return &value
}

func boolPtr(value bool) *bool {
	return &value
}
