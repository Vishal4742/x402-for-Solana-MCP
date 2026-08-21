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
	UpstreamMCPURL     string
	DashboardOrigin    string
	VerifyLockTTL      time.Duration
	ServerID           string
	ServerName         string
	ToolSeeds          map[string]toolSeed
}

type app struct {
	cfg            config
	store          *store
	redis          *redis.Client
	httpClient     *http.Client
	upstreamClient *http.Client
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
	ExpectedReference    string `json:"expectedReference,omitempty"`
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
	Mode           string    `json:"mode,omitempty"`
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
	Reference string     `json:"reference"`
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
	Reference    string `json:"reference"`
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
	Result    any    `json:"result"`
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
		upstreamClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}

	router := newRouterFor(app)

	addr := ":" + cfg.Port
	log.Printf("gateway listening on %s", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatal(err)
	}
}

func newRouterFor(a *app) http.Handler {
	router := chi.NewRouter()
	router.Use(a.corsMiddleware)
	router.Get("/healthz", a.handleHealth)
	router.Post("/mcp/{serverID}", a.handleInvokeMCP)
	router.Get("/v1/challenge/{requestID}", a.handleGetChallenge)
	router.Post("/v1/verify", a.handleVerify)
	router.Get("/v1/servers", a.handleGetServers)
	router.Get("/v1/servers/{serverID}/tools", a.handleGetTools)
	router.Patch("/v1/servers/{serverID}/tools/{toolName}", a.handleUpdateTool)
	router.Get("/v1/requests", a.handleGetRequests)
	router.Get("/v1/requests/{requestID}", a.handleGetRequest)
	router.Get("/v1/receipts/{requestID}", a.handleGetReceipt)
	router.Get("/v1/dashboard/summary", a.handleGetDashboardSummary)
	router.Get("/v1/dashboard/receipts", a.handleGetReceipts)
	return router
}

// corsMiddleware lets the operator dashboard (a separate origin) call the API from
// the browser. DASHBOARD_ORIGIN pins it in production; it defaults to "*" for local dev.
func (a *app) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", a.cfg.DashboardOrigin)
		w.Header().Set("Vary", "Origin")
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Payment-Request-Id")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
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
		case requestStatusExecuted:
			writeJSON(w, http.StatusOK, executionResponse{
				ServerID:  serverID,
				Tool:      req.Tool,
				Status:    "executed",
				RequestID: requestID,
				Result:    record.RawResponse,
			})
			return
		case requestStatusVerified:
			// Serialize execution so concurrent retries make at most one upstream call.
			lockKey := fmt.Sprintf("execute-lock:%s", record.ID)
			acquired, err := a.redis.SetNX(ctx, lockKey, "1", a.cfg.VerifyLockTTL).Result()
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			if !acquired {
				writeJSON(w, http.StatusConflict, map[string]string{"error": "execution already in progress"})
				return
			}
			defer a.redis.Del(context.Background(), lockKey)

			// Re-read under the lock: a concurrent retry may have executed already.
			record, err = a.store.getPaymentRequest(ctx, record.ID)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			if record.Status == requestStatusExecuted {
				writeJSON(w, http.StatusOK, executionResponse{
					ServerID:  serverID,
					Tool:      req.Tool,
					Status:    "executed",
					RequestID: requestID,
					Result:    record.RawResponse,
				})
				return
			}

			forwarding, err := a.store.getServerForwarding(ctx, serverID)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			if forwarding.UpstreamURL == "" {
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
					Result:    result,
				})
				return
			}

			// Forward the original persisted request so what was paid for is what executes.
			result, err := a.forwardToUpstream(ctx, forwarding, serverID, record.ID, record.RawRequest)
			if err != nil {
				// Leave the request verified so the client can retry execution safely.
				writeJSON(w, http.StatusBadGateway, map[string]string{
					"error":     "upstream_error",
					"message":   err.Error(),
					"requestId": record.ID,
				})
				return
			}
			if err := a.store.markExecuted(ctx, record.ID, result); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusOK, executionResponse{
				ServerID:  serverID,
				Tool:      req.Tool,
				Status:    "executed",
				RequestID: requestID,
				Result:    result,
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
		forwarding, err := a.store.getServerForwarding(ctx, serverID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if forwarding.UpstreamURL == "" {
			writeJSON(w, http.StatusOK, executionResponse{
				ServerID: serverID,
				Tool:     req.Tool,
				Status:   "executed",
				Result:   "free tool executed",
			})
			return
		}
		result, err := a.forwardToUpstream(ctx, forwarding, serverID, "", map[string]any{"tool": req.Tool, "input": req.Input})
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "upstream_error", "message": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, executionResponse{
			ServerID: serverID,
			Tool:     req.Tool,
			Status:   "executed",
			Result:   result,
		})
		return
	}

	reference, err := newPaymentReference()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
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
		Reference:    reference,
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
		ExpectedReference:    record.Reference,
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

	if err := a.store.markVerified(ctx, record.ID, req.TxSignature, req.ClientWallet, result.VerifiedAt, result.Mode); err != nil {
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

func (a *app) handleUpdateTool(w http.ResponseWriter, r *http.Request) {
	var body struct {
		PriceUSDC *float64 `json:"priceUsdc"`
		Enabled   *bool    `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if body.PriceUSDC == nil && body.Enabled == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "provide priceUsdc and/or enabled"})
		return
	}

	var priceAtomic *int64
	if body.PriceUSDC != nil {
		if *body.PriceUSDC < 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "priceUsdc must not be negative"})
			return
		}
		atomic := int64(*body.PriceUSDC*1_000_000 + 0.5)
		priceAtomic = &atomic
	}

	updated, err := a.store.updateToolPricing(r.Context(), chi.URLParam(r, "serverID"), chi.URLParam(r, "toolName"), priceAtomic, body.Enabled)
	if errors.Is(err, errToolNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "tool not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, updated)
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

func (a *app) handleGetReceipt(w http.ResponseWriter, r *http.Request) {
	item, err := a.store.getReceiptByRequestID(r.Context(), chi.URLParam(r, "requestID"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "receipt not found"})
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
		UpstreamMCPURL:     getEnv("UPSTREAM_MCP_URL", ""),
		DashboardOrigin:    getEnv("DASHBOARD_ORIGIN", "*"),
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
			Reference:    record.Reference,
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
		Reference: record.Reference,
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
