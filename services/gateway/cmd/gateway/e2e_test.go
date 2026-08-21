package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/redis/go-redis/v9"
)

// stubUpstream is an in-test MCP seller server implementing the upstream contract:
// POST /mcp with {"tool":...,"input":...} echoed back, gateway headers recorded.
type stubUpstream struct {
	mu         sync.Mutex
	hits       int
	failing    bool
	lastBody   map[string]any
	lastHeader http.Header
}

func (u *stubUpstream) handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || r.URL.Path != "/mcp" {
		http.NotFound(w, r)
		return
	}
	data, _ := io.ReadAll(r.Body)
	var parsed map[string]any
	_ = json.Unmarshal(data, &parsed)

	u.mu.Lock()
	u.hits++
	u.lastBody = parsed
	u.lastHeader = r.Header.Clone()
	failing := u.failing
	u.mu.Unlock()

	if failing {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "upstream exploded"})
		return
	}
	tool, _ := parsed["tool"].(string)
	writeJSON(w, http.StatusOK, map[string]any{
		"tool":     tool,
		"echo":     parsed["input"],
		"servedAt": "2026-01-01T00:00:00Z",
	})
}

func (u *stubUpstream) hitCount() int {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.hits
}

func (u *stubUpstream) setFailing(failing bool) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.failing = failing
}

func (u *stubUpstream) last() (map[string]any, http.Header) {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.lastBody, u.lastHeader
}

// stubVerifier mirrors the verifier's mock contract: validate observed vs expected
// fields first, then claim the tx signature via SetNX so replays are rejected.
// Deliberately duplicated from the verifier module to keep the modules decoupled.
// Successful verifications report mode "stub-rpc" so mode-threading assertions can
// tell the verifier's answer apart from markVerified's "mock" fallback. The failing
// toggle simulates a verifier outage with a 502 rpc_error. The expectedReference
// field is accepted and, like the real mock verifier, ignored.
type stubVerifier struct {
	mu      sync.Mutex
	failing bool
	rdb     *redis.Client
}

func (v *stubVerifier) setFailing(failing bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.failing = failing
}

func (v *stubVerifier) isFailing() bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.failing
}

func (v *stubVerifier) handler(w http.ResponseWriter, r *http.Request) {
	var p verifierRequest
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "failed", "failureReason": "bad_request", "failureMessage": "invalid body", "mode": "mock"})
		return
	}
	if v.isFailing() {
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"requestId":      p.RequestID,
			"txSignature":    p.TxSignature,
			"clientWallet":   p.ClientWallet,
			"status":         "failed",
			"failureReason":  "rpc_error",
			"failureMessage": "verifier rpc unavailable",
			"mode":           "mock",
		})
		return
	}
	fail := func(reason, message string) {
		writeJSON(w, http.StatusConflict, map[string]any{
			"requestId":      p.RequestID,
			"txSignature":    p.TxSignature,
			"clientWallet":   p.ClientWallet,
			"status":         "failed",
			"failureReason":  reason,
			"failureMessage": message,
			"mode":           "mock",
		})
	}
	switch {
	case p.ObservedNetwork != "" && p.ObservedNetwork != p.ExpectedNetwork:
		fail("network_mismatch", "network mismatch")
	case p.ObservedRecipient != "" && p.ObservedRecipient != p.ExpectedRecipient:
		fail("recipient_mismatch", "recipient mismatch")
	case p.ObservedTokenMint != "" && p.ObservedTokenMint != p.ExpectedTokenMint:
		fail("token_mint_mismatch", "token mint mismatch")
	case p.ObservedAmountAtomic != nil && *p.ObservedAmountAtomic < p.ExpectedAmountAtomic:
		fail("underpayment", "payment amount below required price")
	default:
		acquired, err := v.rdb.SetNX(r.Context(), "tx-replay:"+p.TxSignature, p.RequestID, 24*time.Hour).Result()
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]any{"status": "failed", "failureReason": "rpc_error", "failureMessage": err.Error(), "mode": "mock"})
			return
		}
		if !acquired {
			fail("replay_detected", "transaction signature already used")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"requestId":    p.RequestID,
			"txSignature":  p.TxSignature,
			"clientWallet": p.ClientWallet,
			"status":       "verified",
			"verifiedAt":   time.Now().UTC(),
			"mode":         "stub-rpc",
		})
	}
}

func freePort(t *testing.T) uint32 {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("free port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	return uint32(port)
}

func doJSON(t *testing.T, method, url string, headers map[string]string, body any) (int, map[string]any) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	var parsed map[string]any
	if len(data) > 0 {
		_ = json.Unmarshal(data, &parsed)
	}
	return resp.StatusCode, parsed
}

func TestGatewayE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in -short mode (starts embedded postgres)")
	}

	// Embedded postgres needs its data/runtime dirs on the native Linux filesystem;
	// the repo lives on /mnt/c (drvfs) where postgres cannot create unix sockets.
	baseDir, err := os.MkdirTemp("/tmp", "x402-gateway-e2e-")
	if err != nil {
		t.Fatalf("mkdir temp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(baseDir) })

	pgPort := freePort(t)
	pg := embeddedpostgres.NewDatabase(embeddedpostgres.DefaultConfig().
		Username("postgres").
		Password("postgres").
		Database("x402").
		Port(pgPort).
		RuntimePath(filepath.Join(baseDir, "pg-runtime")).
		DataPath(filepath.Join(baseDir, "pg-data")).
		StartTimeout(120 * time.Second).
		Logger(io.Discard))
	if err := pg.Start(); err != nil {
		t.Fatalf("start embedded postgres: %v", err)
	}
	t.Cleanup(func() { _ = pg.Stop() })

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	upstream := &stubUpstream{}
	upstreamSrv := httptest.NewServer(http.HandlerFunc(upstream.handler))
	t.Cleanup(upstreamSrv.Close)

	verifier := &stubVerifier{rdb: rdb}
	verifierSrv := httptest.NewServer(http.HandlerFunc(verifier.handler))
	t.Cleanup(verifierSrv.Close)

	cfg := config{
		Port:            "8080",
		SellerWallet:    "SellerWalletE2E",
		TokenMint:       defaultTokenMint,
		Network:         defaultNetwork,
		ChallengeTTL:    defaultChallengeTTL,
		PostgresURL:     fmt.Sprintf("postgres://postgres:postgres@127.0.0.1:%d/x402?sslmode=disable", pgPort),
		RedisURL:        "redis://" + mr.Addr(),
		VerifierBaseURL: verifierSrv.URL,
		UpstreamMCPURL:  upstreamSrv.URL,
		VerifyLockTTL:   defaultVerifyLockTTL,
		ServerID:        defaultServerID,
		ServerName:      defaultServerName,
		ToolSeeds: map[string]toolSeed{
			"ping":            {PriceAtomic: 0, Description: "Healthcheck endpoint.", Enabled: boolPtr(true)},
			"premium.search":  {PriceAtomic: 1000000, Description: "Paid MCP search tool.", Enabled: boolPtr(true)},
			"premium.codegen": {PriceAtomic: 2500000, Description: "Paid code generation tool.", Enabled: boolPtr(true)},
		},
	}

	ctx := context.Background()
	st, err := newStore(ctx, cfg)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(st.close)
	// Run initSchema twice to prove the CREATE + ALTER migration path is idempotent.
	for i := 0; i < 2; i++ {
		if err := st.initSchema(ctx); err != nil {
			t.Fatalf("init schema (pass %d): %v", i+1, err)
		}
	}
	if err := st.seedDefaults(ctx, cfg); err != nil {
		t.Fatalf("seed defaults: %v", err)
	}

	testApp := &app{
		cfg:            cfg,
		store:          st,
		redis:          rdb,
		httpClient:     &http.Client{Timeout: 10 * time.Second},
		upstreamClient: &http.Client{Timeout: 15 * time.Second},
	}
	gatewaySrv := httptest.NewServer(newRouterFor(testApp))
	t.Cleanup(gatewaySrv.Close)

	invokeURL := gatewaySrv.URL + "/mcp/" + cfg.ServerID
	verifyURL := gatewaySrv.URL + "/v1/verify"

	newChallenge := func(t *testing.T, tool string, input map[string]any) string {
		t.Helper()
		status, body := doJSON(t, http.MethodPost, invokeURL, nil, map[string]any{"tool": tool, "input": input})
		if status != http.StatusPaymentRequired {
			t.Fatalf("expected 402 challenge for %s, got %d: %v", tool, status, body)
		}
		challenge, _ := body["challenge"].(map[string]any)
		requestID, _ := challenge["requestId"].(string)
		if requestID == "" {
			t.Fatalf("challenge missing requestId: %v", body)
		}
		return requestID
	}

	requestStatus := func(t *testing.T, requestID string) (string, string) {
		t.Helper()
		status, body := doJSON(t, http.MethodGet, gatewaySrv.URL+"/v1/requests/"+requestID, nil, nil)
		if status != http.StatusOK {
			t.Fatalf("get request %s: %d %v", requestID, status, body)
		}
		state, _ := body["status"].(string)
		reason, _ := body["failureReason"].(string)
		return state, reason
	}

	var searchRequestID string
	originalInput := map[string]any{"query": "solana mcp"}

	t.Run("free tool forwards to upstream", func(t *testing.T) {
		status, body := doJSON(t, http.MethodPost, invokeURL, nil, map[string]any{"tool": "ping", "input": map[string]any{"hello": "world"}})
		if status != http.StatusOK {
			t.Fatalf("expected 200, got %d: %v", status, body)
		}
		if body["status"] != "executed" {
			t.Fatalf("expected executed, got %v", body)
		}
		result, ok := body["result"].(map[string]any)
		if !ok {
			t.Fatalf("expected object result, got %T", body["result"])
		}
		if result["tool"] != "ping" {
			t.Fatalf("upstream result not returned: %v", result)
		}
		echo, _ := result["echo"].(map[string]any)
		if echo["hello"] != "world" {
			t.Fatalf("input not echoed by upstream: %v", result)
		}
		lastBody, lastHeader := upstream.last()
		if lastHeader.Get("X-Gateway-Key") != defaultServerAPIKey {
			t.Fatalf("upstream did not receive gateway key, got %q", lastHeader.Get("X-Gateway-Key"))
		}
		if lastHeader.Get("X-Gateway-Server-Id") != cfg.ServerID {
			t.Fatalf("wrong X-Gateway-Server-Id: %q", lastHeader.Get("X-Gateway-Server-Id"))
		}
		if lastHeader.Get("X-Gateway-Request-Id") != "" {
			t.Fatalf("free tool should carry empty X-Gateway-Request-Id, got %q", lastHeader.Get("X-Gateway-Request-Id"))
		}
		if lastBody["tool"] != "ping" {
			t.Fatalf("upstream saw wrong body: %v", lastBody)
		}
	})

	t.Run("paid tool without header returns 402 challenge", func(t *testing.T) {
		status, body := doJSON(t, http.MethodPost, invokeURL, nil, map[string]any{"tool": "premium.search", "input": originalInput})
		if status != http.StatusPaymentRequired {
			t.Fatalf("expected 402, got %d: %v", status, body)
		}
		if body["error"] != "payment_required" {
			t.Fatalf("expected payment_required error, got %v", body)
		}
		challenge, _ := body["challenge"].(map[string]any)
		if challenge["requestId"] == "" || challenge["requestId"] == nil {
			t.Fatalf("challenge missing requestId: %v", body)
		}
		if challenge["amountAtomic"] != float64(1000000) {
			t.Fatalf("wrong challenge amount: %v", challenge["amountAtomic"])
		}
		if challenge["tokenMint"] != defaultTokenMint {
			t.Fatalf("wrong token mint: %v", challenge["tokenMint"])
		}
		if challenge["recipient"] != cfg.SellerWallet {
			t.Fatalf("wrong recipient: %v", challenge["recipient"])
		}
		if challenge["network"] != defaultNetwork {
			t.Fatalf("wrong network: %v", challenge["network"])
		}
		expiresAt, err := time.Parse(time.RFC3339, fmt.Sprintf("%v", challenge["expiresAt"]))
		if err != nil || !expiresAt.After(time.Now().UTC()) {
			t.Fatalf("challenge expiry not in the future: %v (%v)", challenge["expiresAt"], err)
		}
		reference, _ := challenge["reference"].(string)
		if len(reference) < 32 {
			t.Fatalf("challenge missing base58 reference: %v", challenge["reference"])
		}
		pricing, _ := body["pricing"].(map[string]any)
		if pricing["amountAtomic"] != float64(1000000) || pricing["recipient"] != cfg.SellerWallet {
			t.Fatalf("pricing metadata wrong: %v", pricing)
		}
		if pricing["reference"] != reference {
			t.Fatalf("pricing reference mismatch: %v vs %s", pricing["reference"], reference)
		}
		retry, _ := body["retry"].(map[string]any)
		if retry["header"] != "X-Payment-Request-Id" || retry["requestId"] != challenge["requestId"] {
			t.Fatalf("retry metadata wrong: %v", retry)
		}
		searchRequestID = challenge["requestId"].(string)
	})

	t.Run("verify then execute forwards original request and writes receipt", func(t *testing.T) {
		if searchRequestID == "" {
			t.Fatal("missing challenge from previous step")
		}
		status, body := doJSON(t, http.MethodPost, verifyURL, nil, map[string]any{
			"requestId":    searchRequestID,
			"txSignature":  "sig-e2e-1",
			"clientWallet": "BuyerWalletE2E",
		})
		if status != http.StatusOK || body["status"] != "verified" {
			t.Fatalf("verify failed: %d %v", status, body)
		}

		// mode threading: the verified timeline event must carry the verifier's mode.
		status, requestBody := doJSON(t, http.MethodGet, gatewaySrv.URL+"/v1/requests/"+searchRequestID, nil, nil)
		if status != http.StatusOK {
			t.Fatalf("get request: %d %v", status, requestBody)
		}
		timeline, _ := requestBody["timeline"].([]any)
		foundMode := false
		for _, item := range timeline {
			event, _ := item.(map[string]any)
			if event["status"] == "verified" {
				payload, _ := event["payload"].(map[string]any)
				if payload["rpc"] == "stub-rpc" {
					foundMode = true
				}
			}
		}
		if !foundMode {
			t.Fatalf("verified event missing mode=stub-rpc payload: %v", timeline)
		}

		hitsBefore := upstream.hitCount()
		// Retry with a TAMPERED body: the gateway must forward the original raw_request.
		status, body = doJSON(t, http.MethodPost, invokeURL, map[string]string{"X-Payment-Request-Id": searchRequestID}, map[string]any{
			"tool":  "premium.search",
			"input": map[string]any{"tampered": true},
		})
		if status != http.StatusOK || body["status"] != "executed" {
			t.Fatalf("execution failed: %d %v", status, body)
		}
		if body["requestId"] != searchRequestID {
			t.Fatalf("wrong requestId in execution response: %v", body)
		}
		if upstream.hitCount() != hitsBefore+1 {
			t.Fatalf("expected exactly one upstream call, hits %d -> %d", hitsBefore, upstream.hitCount())
		}
		lastBody, lastHeader := upstream.last()
		if !reflect.DeepEqual(lastBody, map[string]any{"tool": "premium.search", "input": originalInput}) {
			t.Fatalf("upstream did not receive the original raw_request: %v", lastBody)
		}
		if lastHeader.Get("X-Gateway-Request-Id") != searchRequestID {
			t.Fatalf("upstream missing request id header: %q", lastHeader.Get("X-Gateway-Request-Id"))
		}
		firstResult, ok := body["result"].(map[string]any)
		if !ok {
			t.Fatalf("expected object result, got %T", body["result"])
		}
		echo, _ := firstResult["echo"].(map[string]any)
		if echo["query"] != "solana mcp" {
			t.Fatalf("result does not reflect original input: %v", firstResult)
		}

		if state, _ := requestStatus(t, searchRequestID); state != requestStatusExecuted {
			t.Fatalf("expected executed status, got %s", state)
		}

		status, receipt := doJSON(t, http.MethodGet, gatewaySrv.URL+"/v1/receipts/"+searchRequestID, nil, nil)
		if status != http.StatusOK {
			t.Fatalf("receipt lookup failed: %d %v", status, receipt)
		}
		if receipt["requestId"] != searchRequestID || receipt["toolName"] != "premium.search" || receipt["txSignature"] != "sig-e2e-1" {
			t.Fatalf("receipt fields wrong: %v", receipt)
		}
		responseHash, _ := receipt["responseHash"].(string)
		if len(responseHash) != 64 {
			t.Fatalf("expected sha256 hex responseHash, got %q", responseHash)
		}
		expectedJSON, err := json.Marshal(firstResult)
		if err != nil {
			t.Fatalf("marshal result: %v", err)
		}
		if expected := fmt.Sprintf("%x", sha256.Sum256(expectedJSON)); responseHash != expected {
			t.Fatalf("responseHash mismatch: got %s want %s", responseHash, expected)
		}

		// Idempotent replay: same persisted result, no extra upstream call.
		hitsBefore = upstream.hitCount()
		status, body = doJSON(t, http.MethodPost, invokeURL, map[string]string{"X-Payment-Request-Id": searchRequestID}, map[string]any{
			"tool":  "premium.search",
			"input": map[string]any{},
		})
		if status != http.StatusOK || body["status"] != "executed" {
			t.Fatalf("idempotent replay failed: %d %v", status, body)
		}
		if !reflect.DeepEqual(body["result"], any(firstResult)) {
			t.Fatalf("replay result differs from persisted response: %v vs %v", body["result"], firstResult)
		}
		if upstream.hitCount() != hitsBefore {
			t.Fatalf("idempotent replay must not call upstream again")
		}
	})

	t.Run("replayed tx signature is rejected and request fails", func(t *testing.T) {
		requestID := newChallenge(t, "premium.codegen", map[string]any{"language": "go"})
		status, body := doJSON(t, http.MethodPost, verifyURL, nil, map[string]any{
			"requestId":    requestID,
			"txSignature":  "sig-e2e-1",
			"clientWallet": "BuyerWalletE2E",
		})
		if status != http.StatusConflict {
			t.Fatalf("expected 409 replay rejection, got %d: %v", status, body)
		}
		state, reason := requestStatus(t, requestID)
		if state != requestStatusFailed || reason != "replay_detected" {
			t.Fatalf("expected failed/replay_detected, got %s/%s", state, reason)
		}
	})

	t.Run("underpayment is rejected and request fails", func(t *testing.T) {
		requestID := newChallenge(t, "premium.search", originalInput)
		status, body := doJSON(t, http.MethodPost, verifyURL, nil, map[string]any{
			"requestId":    requestID,
			"txSignature":  "sig-e2e-underpaid",
			"clientWallet": "BuyerWalletE2E",
			"amountAtomic": 1,
		})
		if status != http.StatusConflict {
			t.Fatalf("expected 409 underpayment, got %d: %v", status, body)
		}
		state, reason := requestStatus(t, requestID)
		if state != requestStatusFailed || reason != "underpayment" {
			t.Fatalf("expected failed/underpayment, got %s/%s", state, reason)
		}
	})

	t.Run("expired challenge fails with timeout", func(t *testing.T) {
		shortCfg := cfg
		shortCfg.ChallengeTTL = time.Millisecond
		shortApp := &app{
			cfg:            shortCfg,
			store:          st,
			redis:          rdb,
			httpClient:     testApp.httpClient,
			upstreamClient: testApp.upstreamClient,
		}
		shortSrv := httptest.NewServer(newRouterFor(shortApp))
		defer shortSrv.Close()

		status, body := doJSON(t, http.MethodPost, shortSrv.URL+"/mcp/"+cfg.ServerID, nil, map[string]any{"tool": "premium.search", "input": originalInput})
		if status != http.StatusPaymentRequired {
			t.Fatalf("expected 402, got %d: %v", status, body)
		}
		challenge, _ := body["challenge"].(map[string]any)
		requestID, _ := challenge["requestId"].(string)

		time.Sleep(50 * time.Millisecond)
		status, body = doJSON(t, http.MethodPost, verifyURL, nil, map[string]any{
			"requestId":    requestID,
			"txSignature":  "sig-e2e-expired",
			"clientWallet": "BuyerWalletE2E",
		})
		if status != http.StatusConflict {
			t.Fatalf("expected 409 challenge expired, got %d: %v", status, body)
		}
		if body["error"] != "challenge expired" {
			t.Fatalf("expected challenge expired error, got %v", body)
		}
		state, reason := requestStatus(t, requestID)
		if state != requestStatusFailed || reason != "timeout" {
			t.Fatalf("expected failed/timeout, got %s/%s", state, reason)
		}
	})

	t.Run("upstream failure keeps request verified and retry succeeds", func(t *testing.T) {
		requestID := newChallenge(t, "premium.search", map[string]any{"query": "retry me"})
		status, body := doJSON(t, http.MethodPost, verifyURL, nil, map[string]any{
			"requestId":    requestID,
			"txSignature":  "sig-e2e-2",
			"clientWallet": "BuyerWalletE2E",
		})
		if status != http.StatusOK {
			t.Fatalf("verify failed: %d %v", status, body)
		}

		upstream.setFailing(true)
		status, body = doJSON(t, http.MethodPost, invokeURL, map[string]string{"X-Payment-Request-Id": requestID}, map[string]any{
			"tool":  "premium.search",
			"input": map[string]any{},
		})
		if status != http.StatusBadGateway {
			t.Fatalf("expected 502 on upstream failure, got %d: %v", status, body)
		}
		if body["error"] != "upstream_error" || body["requestId"] != requestID {
			t.Fatalf("wrong upstream error payload: %v", body)
		}
		if state, _ := requestStatus(t, requestID); state != requestStatusVerified {
			t.Fatalf("request must stay verified after upstream failure, got %s", state)
		}

		upstream.setFailing(false)
		status, body = doJSON(t, http.MethodPost, invokeURL, map[string]string{"X-Payment-Request-Id": requestID}, map[string]any{
			"tool":  "premium.search",
			"input": map[string]any{},
		})
		if status != http.StatusOK || body["status"] != "executed" {
			t.Fatalf("retry after upstream recovery failed: %d %v", status, body)
		}
		if state, _ := requestStatus(t, requestID); state != requestStatusExecuted {
			t.Fatalf("expected executed after retry, got %s", state)
		}
		status, receipt := doJSON(t, http.MethodGet, gatewaySrv.URL+"/v1/receipts/"+requestID, nil, nil)
		if status != http.StatusOK || receipt["requestId"] != requestID {
			t.Fatalf("receipt missing after recovered execution: %d %v", status, receipt)
		}
	})

	t.Run("retry with header while pending or failed never hits upstream", func(t *testing.T) {
		requestID := newChallenge(t, "premium.search", map[string]any{"query": "pending retry"})
		hitsBefore := upstream.hitCount()

		// Pending: retry with the header returns the same 402 challenge, no upstream call.
		status, body := doJSON(t, http.MethodPost, invokeURL, map[string]string{"X-Payment-Request-Id": requestID}, map[string]any{
			"tool":  "premium.search",
			"input": map[string]any{},
		})
		if status != http.StatusPaymentRequired {
			t.Fatalf("expected 402 for pending retry, got %d: %v", status, body)
		}
		challenge, _ := body["challenge"].(map[string]any)
		if challenge["requestId"] != requestID {
			t.Fatalf("pending retry must return the same challenge: %v", challenge)
		}
		if reference, _ := challenge["reference"].(string); reference == "" {
			t.Fatalf("re-issued challenge missing reference: %v", challenge)
		}
		if upstream.hitCount() != hitsBefore {
			t.Fatalf("pending retry must not hit upstream, hits %d -> %d", hitsBefore, upstream.hitCount())
		}

		// Drive the request to failed via underpayment, then retry with the header.
		status, body = doJSON(t, http.MethodPost, verifyURL, nil, map[string]any{
			"requestId":    requestID,
			"txSignature":  "sig-e2e-pending-underpaid",
			"clientWallet": "BuyerWalletE2E",
			"amountAtomic": 1,
		})
		if status != http.StatusConflict {
			t.Fatalf("expected 409 underpayment, got %d: %v", status, body)
		}
		status, body = doJSON(t, http.MethodPost, invokeURL, map[string]string{"X-Payment-Request-Id": requestID}, map[string]any{
			"tool":  "premium.search",
			"input": map[string]any{},
		})
		if status != http.StatusConflict {
			t.Fatalf("expected 409 for failed request retry, got %d: %v", status, body)
		}
		if upstream.hitCount() != hitsBefore {
			t.Fatalf("failed retry must not hit upstream, hits %d -> %d", hitsBefore, upstream.hitCount())
		}
	})

	t.Run("verifier outage keeps request pending and retry verifies", func(t *testing.T) {
		requestID := newChallenge(t, "premium.search", map[string]any{"query": "verifier down"})

		verifier.setFailing(true)
		status, body := doJSON(t, http.MethodPost, verifyURL, nil, map[string]any{
			"requestId":    requestID,
			"txSignature":  "sig-e2e-verifier-outage",
			"clientWallet": "BuyerWalletE2E",
		})
		if status != http.StatusBadGateway {
			t.Fatalf("expected 502 while verifier is down, got %d: %v", status, body)
		}
		if state, _ := requestStatus(t, requestID); state != requestStatusChallenged {
			t.Fatalf("request must stay pending after verifier outage, got %s", state)
		}

		verifier.setFailing(false)
		status, body = doJSON(t, http.MethodPost, verifyURL, nil, map[string]any{
			"requestId":    requestID,
			"txSignature":  "sig-e2e-verifier-outage",
			"clientWallet": "BuyerWalletE2E",
		})
		if status != http.StatusOK || body["status"] != "verified" {
			t.Fatalf("retry after verifier recovery failed: %d %v", status, body)
		}
		if state, _ := requestStatus(t, requestID); state != requestStatusVerified {
			t.Fatalf("expected verified after recovery, got %s", state)
		}
	})

	t.Run("concurrent retries execute exactly once", func(t *testing.T) {
		requestID := newChallenge(t, "premium.search", map[string]any{"query": "race"})
		status, body := doJSON(t, http.MethodPost, verifyURL, nil, map[string]any{
			"requestId":    requestID,
			"txSignature":  "sig-e2e-race",
			"clientWallet": "BuyerWalletE2E",
		})
		if status != http.StatusOK {
			t.Fatalf("verify failed: %d %v", status, body)
		}

		hitsBefore := upstream.hitCount()
		type retryResult struct {
			status int
			body   map[string]any
			err    error
		}
		results := make(chan retryResult, 2)
		start := make(chan struct{})
		for i := 0; i < 2; i++ {
			go func() {
				<-start
				payload, err := json.Marshal(map[string]any{"tool": "premium.search", "input": map[string]any{}})
				if err != nil {
					results <- retryResult{err: err}
					return
				}
				req, err := http.NewRequest(http.MethodPost, invokeURL, bytes.NewReader(payload))
				if err != nil {
					results <- retryResult{err: err}
					return
				}
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("X-Payment-Request-Id", requestID)
				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					results <- retryResult{err: err}
					return
				}
				defer resp.Body.Close()
				data, err := io.ReadAll(resp.Body)
				if err != nil {
					results <- retryResult{err: err}
					return
				}
				var parsed map[string]any
				_ = json.Unmarshal(data, &parsed)
				results <- retryResult{status: resp.StatusCode, body: parsed}
			}()
		}
		close(start)

		executed := 0
		for i := 0; i < 2; i++ {
			res := <-results
			if res.err != nil {
				t.Fatalf("concurrent retry failed: %v", res.err)
			}
			switch res.status {
			case http.StatusOK:
				if res.body["status"] != "executed" {
					t.Fatalf("expected executed response, got %v", res.body)
				}
				executed++
			case http.StatusConflict:
				if res.body["error"] != "execution already in progress" {
					t.Fatalf("unexpected 409 payload: %v", res.body)
				}
			default:
				t.Fatalf("unexpected concurrent retry status %d: %v", res.status, res.body)
			}
		}
		if executed == 0 {
			t.Fatal("at least one concurrent retry must execute")
		}
		if upstream.hitCount() != hitsBefore+1 {
			t.Fatalf("expected exactly one upstream hit, hits %d -> %d", hitsBefore, upstream.hitCount())
		}

		status, requestBody := doJSON(t, http.MethodGet, gatewaySrv.URL+"/v1/requests/"+requestID, nil, nil)
		if status != http.StatusOK {
			t.Fatalf("get request: %d %v", status, requestBody)
		}
		timeline, _ := requestBody["timeline"].([]any)
		executedEvents := 0
		for _, item := range timeline {
			event, _ := item.(map[string]any)
			if event["status"] == requestStatusExecuted {
				executedEvents++
			}
		}
		if executedEvents != 1 {
			t.Fatalf("expected exactly one executed timeline event, got %d: %v", executedEvents, timeline)
		}
		rawResponse, ok := requestBody["rawResponse"].(map[string]any)
		if !ok || len(rawResponse) == 0 {
			t.Fatalf("missing persisted rawResponse: %v", requestBody)
		}
		rawResponseJSON, err := json.Marshal(rawResponse)
		if err != nil {
			t.Fatalf("marshal rawResponse: %v", err)
		}
		status, receipt := doJSON(t, http.MethodGet, gatewaySrv.URL+"/v1/receipts/"+requestID, nil, nil)
		if status != http.StatusOK {
			t.Fatalf("receipt lookup failed: %d %v", status, receipt)
		}
		if expected := fmt.Sprintf("%x", sha256.Sum256(rawResponseJSON)); receipt["responseHash"] != expected {
			t.Fatalf("responseHash mismatch: got %v want %s", receipt["responseHash"], expected)
		}
	})

	t.Run("unknown receipt returns 404", func(t *testing.T) {
		status, body := doJSON(t, http.MethodGet, gatewaySrv.URL+"/v1/receipts/does-not-exist", nil, nil)
		if status != http.StatusNotFound {
			t.Fatalf("expected 404, got %d: %v", status, body)
		}
		if body["error"] != "receipt not found" {
			t.Fatalf("wrong 404 payload: %v", body)
		}
	})

	t.Run("operator price update is reflected in the next challenge", func(t *testing.T) {
		toolURL := gatewaySrv.URL + "/v1/servers/" + cfg.ServerID + "/tools/premium.codegen"

		status, updated := doJSON(t, http.MethodPatch, toolURL, nil, map[string]any{"priceUsdc": 4})
		if status != http.StatusOK {
			t.Fatalf("expected 200 on price update, got %d: %v", status, updated)
		}
		if updated["priceUsdc"] != float64(4) {
			t.Fatalf("update did not return new price: %v", updated)
		}

		status, body := doJSON(t, http.MethodPost, invokeURL, nil, map[string]any{"tool": "premium.codegen", "input": map[string]any{}})
		if status != http.StatusPaymentRequired {
			t.Fatalf("expected 402, got %d: %v", status, body)
		}
		challenge, _ := body["challenge"].(map[string]any)
		if challenge["amountAtomic"] != float64(4000000) {
			t.Fatalf("challenge did not reflect updated price, got %v", challenge["amountAtomic"])
		}

		status, _ = doJSON(t, http.MethodPatch, toolURL, nil, map[string]any{"enabled": false})
		if status != http.StatusOK {
			t.Fatalf("expected 200 disabling tool, got %d", status)
		}
		status, body = doJSON(t, http.MethodPost, invokeURL, nil, map[string]any{"tool": "premium.codegen", "input": map[string]any{}})
		if status != http.StatusNotFound {
			t.Fatalf("disabled tool should 404, got %d: %v", status, body)
		}

		status, body = doJSON(t, http.MethodPatch, gatewaySrv.URL+"/v1/servers/"+cfg.ServerID+"/tools/nope", nil, map[string]any{"priceUsdc": 1})
		if status != http.StatusNotFound {
			t.Fatalf("unknown tool should 404, got %d: %v", status, body)
		}
	})
}
