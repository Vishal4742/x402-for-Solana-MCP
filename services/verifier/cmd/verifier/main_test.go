package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

const (
	testWallet    = "BuyerWa11et1111111111111111111111111111111111"
	testRecipient = "Se11erRecipient111111111111111111111111111111"
	testMint      = "USDCdevMint11111111111111111111111111111111"
	testAmount    = int64(10000)
)

func basePayload() verifyPayload {
	return verifyPayload{
		RequestID:            "req_01HTEST",
		TxSignature:          "5VERYrealLookingSignature111111111111111111111111111111111111111111111111111111111111",
		ClientWallet:         testWallet,
		ExpectedRecipient:    testRecipient,
		ExpectedTokenMint:    testMint,
		ExpectedAmountAtomic: testAmount,
		ExpectedNetwork:      "devnet",
	}
}

type testEnv struct {
	server *httptest.Server
	redis  *miniredis.Miniredis
}

func newMockEnv(t *testing.T) testEnv {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ts := httptest.NewServer(newRouter(&mockVerifier{rdb: rdb}, rdb))
	t.Cleanup(ts.Close)
	return testEnv{server: ts, redis: mr}
}

func postVerify(t *testing.T, baseURL string, p verifyPayload) (int, verifyResponse) {
	t.Helper()
	body, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Post(baseURL+"/verify/solana", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out verifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, out
}

func i64(v int64) *int64 { return &v }

func TestMockHappyPath(t *testing.T) {
	env := newMockEnv(t)

	status, resp := postVerify(t, env.server.URL, basePayload())
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200 (resp %+v)", status, resp)
	}
	if resp.Status != "verified" {
		t.Errorf("Status = %q, want verified", resp.Status)
	}
	if resp.Mode != "mock" {
		t.Errorf("Mode = %q, want mock", resp.Mode)
	}
	if resp.VerifiedAt.IsZero() {
		t.Error("VerifiedAt is zero")
	}
	if !env.redis.Exists(replayKeyPrefix + basePayload().TxSignature) {
		t.Error("replay key not claimed after successful verify")
	}
}

func TestMockReplayRejected(t *testing.T) {
	env := newMockEnv(t)

	if status, _ := postVerify(t, env.server.URL, basePayload()); status != http.StatusOK {
		t.Fatalf("first verify status = %d, want 200", status)
	}

	second := basePayload()
	second.RequestID = "req_01HOTHER"
	status, resp := postVerify(t, env.server.URL, second)
	if status != http.StatusConflict {
		t.Fatalf("second verify status = %d, want 409", status)
	}
	if resp.FailureReason != "replay_detected" {
		t.Errorf("FailureReason = %q, want replay_detected", resp.FailureReason)
	}
	if resp.Status != "failed" || resp.Mode != "mock" {
		t.Errorf("Status/Mode = %q/%q, want failed/mock", resp.Status, resp.Mode)
	}
}

// The same request retrying its own signature (e.g. the gateway lost our
// 200 and re-verifies) is idempotent, not a replay.
func TestMockIdempotentSameRequest(t *testing.T) {
	env := newMockEnv(t)

	if status, _ := postVerify(t, env.server.URL, basePayload()); status != http.StatusOK {
		t.Fatalf("first verify status = %d, want 200", status)
	}
	status, resp := postVerify(t, env.server.URL, basePayload())
	if status != http.StatusOK || resp.Status != "verified" {
		t.Fatalf("retry with same requestId = %d/%q, want 200/verified", status, resp.Status)
	}
}

// The replay claim is permanent: no TTL, and a different requestId is still
// rejected long after the original verification.
func TestMockReplayClaimPermanent(t *testing.T) {
	env := newMockEnv(t)

	if status, _ := postVerify(t, env.server.URL, basePayload()); status != http.StatusOK {
		t.Fatalf("first verify status = %d, want 200", status)
	}
	key := replayKeyPrefix + basePayload().TxSignature
	if ttl := env.redis.TTL(key); ttl != 0 {
		t.Errorf("replay key TTL = %s, want 0 (permanent)", ttl)
	}

	env.redis.FastForward(30 * 24 * time.Hour)
	if !env.redis.Exists(key) {
		t.Fatal("replay key expired; claim must be permanent")
	}

	second := basePayload()
	second.RequestID = "req_01HOTHER"
	status, resp := postVerify(t, env.server.URL, second)
	if status != http.StatusConflict || resp.FailureReason != "replay_detected" {
		t.Fatalf("replay after 30d = %d/%q, want 409/replay_detected", status, resp.FailureReason)
	}
}

func TestMockUnderpayment(t *testing.T) {
	env := newMockEnv(t)

	p := basePayload()
	p.ObservedAmountAtomic = i64(testAmount - 1)
	status, resp := postVerify(t, env.server.URL, p)
	if status != http.StatusConflict {
		t.Fatalf("status = %d, want 409", status)
	}
	if resp.FailureReason != "underpayment" {
		t.Errorf("FailureReason = %q, want underpayment", resp.FailureReason)
	}
}

// A rejected verification must not consume the signature: a bad attempt
// followed by a good attempt with the same signature succeeds.
func TestMockFailedValidationDoesNotBurnSignature(t *testing.T) {
	env := newMockEnv(t)

	bad := basePayload()
	bad.ObservedRecipient = "WrongRecipient111111111111111111111111111111"
	status, resp := postVerify(t, env.server.URL, bad)
	if status != http.StatusConflict || resp.FailureReason != "recipient_mismatch" {
		t.Fatalf("bad attempt = %d/%q, want 409/recipient_mismatch", status, resp.FailureReason)
	}
	if env.redis.Exists(replayKeyPrefix + bad.TxSignature) {
		t.Fatal("failed validation burned the signature")
	}

	status, resp = postVerify(t, env.server.URL, basePayload())
	if status != http.StatusOK || resp.Status != "verified" {
		t.Fatalf("good attempt after failed one = %d/%q, want 200/verified", status, resp.Status)
	}
}

func TestMockTransactionNotFound(t *testing.T) {
	env := newMockEnv(t)

	p := basePayload()
	p.TxSignature = "missing"
	status, resp := postVerify(t, env.server.URL, p)
	if status != http.StatusConflict {
		t.Fatalf("status = %d, want 409", status)
	}
	if resp.FailureReason != "transaction_not_found" {
		t.Errorf("FailureReason = %q, want transaction_not_found", resp.FailureReason)
	}
}

func TestMockMissingFields(t *testing.T) {
	env := newMockEnv(t)

	p := basePayload()
	p.ClientWallet = ""
	status, _ := postVerify(t, env.server.URL, p)
	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", status)
	}
}

func TestHealthz(t *testing.T) {
	env := newMockEnv(t)

	resp, err := http.Get(env.server.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("healthz status = %d, want 200", resp.StatusCode)
	}
}

func TestNewVerifierFromConfig(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	if _, ok := newVerifierFromConfig(config{VerifyMode: "mock"}, rdb).(*mockVerifier); !ok {
		t.Error("mode mock did not produce mockVerifier")
	}
	v, ok := newVerifierFromConfig(config{
		VerifyMode: "rpc",
		RPCURL:     "https://api.devnet.solana.com",
		Network:    "devnet",
	}, rdb).(*rpcVerifier)
	if !ok {
		t.Fatal("mode rpc did not produce rpcVerifier")
	}
	if v.httpClient.Timeout != 15*time.Second {
		t.Errorf("rpc http timeout = %s, want 15s", v.httpClient.Timeout)
	}
}
