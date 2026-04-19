package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type store struct {
	pool *pgxpool.Pool
}

type paymentRequestRecord struct {
	ID            string
	ServerID      string
	ToolName      string
	PayerWallet   *string
	AmountAtomic  int64
	TokenMint     string
	Recipient     string
	Network       string
	Scheme        string
	Status        string
	FailureReason *string
	TxSignature   *string
	CreatedAt     time.Time
	ExpiresAt     time.Time
	SettledAt     *time.Time
	RawRequest    map[string]any
	RawResponse   map[string]any
}

type requestEvent struct {
	Status        string         `json:"status"`
	Timestamp     string         `json:"timestamp"`
	Payload       map[string]any `json:"payload,omitempty"`
	SignatureHash string         `json:"signatureHash,omitempty"`
	DurationMs    *int           `json:"durationMs,omitempty"`
}

type serverView struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Endpoint     string    `json:"endpoint"`
	PayoutWallet string    `json:"payoutWallet"`
	WebhookURL   string    `json:"webhookUrl"`
	Network      string    `json:"network"`
	CreatedAt    time.Time `json:"createdAt"`
	APIKey       string    `json:"apiKey"`
}

type toolView struct {
	ID           string  `json:"id"`
	ServerID     string  `json:"serverId"`
	ToolName     string  `json:"toolName"`
	Description  string  `json:"description"`
	PriceUSDC    float64 `json:"priceUsdc"`
	Enabled      bool    `json:"enabled"`
	CallsLast24h int64   `json:"callsLast24h"`
}

type requestView struct {
	ID            string         `json:"id"`
	ServerID      string         `json:"serverId"`
	ToolName      string         `json:"toolName"`
	PayerWallet   string         `json:"payerWallet"`
	AmountUSDC    float64        `json:"amountUsdc"`
	Status        string         `json:"status"`
	FailureReason *string        `json:"failureReason,omitempty"`
	TxSignature   *string        `json:"txSignature,omitempty"`
	CreatedAt     string         `json:"createdAt"`
	SettledAt     *string        `json:"settledAt,omitempty"`
	Timeline      []requestEvent `json:"timeline"`
	RawRequest    map[string]any `json:"rawRequest"`
	RawResponse   map[string]any `json:"rawResponse,omitempty"`
}

type receiptView struct {
	ID          string  `json:"id"`
	RequestID   string  `json:"requestId"`
	ServerID    string  `json:"serverId"`
	ToolName    string  `json:"toolName"`
	AmountUSDC  float64 `json:"amountUsdc"`
	PayerWallet string  `json:"payerWallet"`
	TxSignature string  `json:"txSignature"`
	BlockTime   string  `json:"blockTime"`
	CreatedAt   string  `json:"createdAt"`
}

type dashboardSummaryView struct {
	TotalRevenueUSDC    float64 `json:"totalRevenueUsdc"`
	PaidRequests        int64   `json:"paidRequests"`
	FailedVerifications int64   `json:"failedVerifications"`
	AvgSettlementMs     int64   `json:"avgSettlementMs"`
	PaidToolCount       int64   `json:"paidToolCount"`
	FreeToolCount       int64   `json:"freeToolCount"`
	RevenueDelta7d      float64 `json:"revenueDelta7d"`
	PaidDelta7d         float64 `json:"paidDelta7d"`
}

type createPaymentRequestInput struct {
	ID           string
	ServerID     string
	ToolName     string
	AmountAtomic int64
	TokenMint    string
	Recipient    string
	Network      string
	Scheme       string
	ExpiresAt    time.Time
	RawRequest   any
}

func newStore(ctx context.Context, cfg config) (*store, error) {
	pool, err := pgxpool.New(ctx, cfg.PostgresURL)
	if err != nil {
		return nil, err
	}
	return &store{pool: pool}, nil
}

func (s *store) close() {
	s.pool.Close()
}

func (s *store) ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

func (s *store) initSchema(ctx context.Context) error {
	schema := `
CREATE TABLE IF NOT EXISTS servers (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  endpoint TEXT NOT NULL,
  payout_wallet TEXT NOT NULL,
  webhook_url TEXT NOT NULL DEFAULT '',
  network TEXT NOT NULL,
  api_key TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS tool_pricing (
  id TEXT PRIMARY KEY,
  server_id TEXT NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
  tool_name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  price_atomic BIGINT NOT NULL DEFAULT 0,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(server_id, tool_name)
);

CREATE TABLE IF NOT EXISTS payment_requests (
  id TEXT PRIMARY KEY,
  server_id TEXT NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
  tool_name TEXT NOT NULL,
  payer_wallet TEXT,
  amount_atomic BIGINT NOT NULL,
  token_mint TEXT NOT NULL,
  recipient TEXT NOT NULL,
  network TEXT NOT NULL,
  scheme TEXT NOT NULL,
  status TEXT NOT NULL,
  failure_reason TEXT,
  tx_signature TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  expires_at TIMESTAMPTZ NOT NULL,
  settled_at TIMESTAMPTZ,
  raw_request JSONB NOT NULL DEFAULT '{}'::jsonb,
  raw_response JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE TABLE IF NOT EXISTS request_events (
  id BIGSERIAL PRIMARY KEY,
  request_id TEXT NOT NULL REFERENCES payment_requests(id) ON DELETE CASCADE,
  status TEXT NOT NULL,
  payload JSONB NOT NULL DEFAULT '{}'::jsonb,
  signature_hash TEXT,
  duration_ms INTEGER,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS receipts (
  id TEXT PRIMARY KEY,
  request_id TEXT NOT NULL UNIQUE REFERENCES payment_requests(id) ON DELETE CASCADE,
  server_id TEXT NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
  tool_name TEXT NOT NULL,
  amount_atomic BIGINT NOT NULL,
  payer_wallet TEXT NOT NULL,
  tx_signature TEXT NOT NULL,
  block_time TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_payment_requests_server_status ON payment_requests(server_id, status);
CREATE INDEX IF NOT EXISTS idx_payment_requests_created_at ON payment_requests(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_receipts_server_created_at ON receipts(server_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_request_events_request_id_created_at ON request_events(request_id, created_at ASC);
`

	_, err := s.pool.Exec(ctx, schema)
	return err
}

func (s *store) seedDefaults(ctx context.Context, cfg config) error {
	endpoint := fmt.Sprintf("http://localhost:%s/mcp/%s", cfg.Port, cfg.ServerID)
	_, err := s.pool.Exec(ctx, `
INSERT INTO servers (id, name, endpoint, payout_wallet, webhook_url, network, api_key)
VALUES ($1, $2, $3, $4, '', $5, $6)
ON CONFLICT (id) DO UPDATE SET
  name = EXCLUDED.name,
  endpoint = EXCLUDED.endpoint,
  payout_wallet = EXCLUDED.payout_wallet,
  network = EXCLUDED.network,
  api_key = EXCLUDED.api_key
`, cfg.ServerID, cfg.ServerName, endpoint, cfg.SellerWallet, cfg.Network, defaultServerAPIKey)
	if err != nil {
		return err
	}

	for toolName, seed := range cfg.ToolSeeds {
		enabled := true
		if seed.Enabled != nil {
			enabled = *seed.Enabled
		}
		description := seed.Description
		if description == "" {
			description = "Configured via TOOL_PRICING_JSON."
		}
		toolID := fmt.Sprintf("tool_%s_%s", cfg.ServerID, sanitizeIdentifier(toolName))
		_, err := s.pool.Exec(ctx, `
INSERT INTO tool_pricing (id, server_id, tool_name, description, price_atomic, enabled)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (server_id, tool_name) DO UPDATE SET
  description = EXCLUDED.description,
  price_atomic = EXCLUDED.price_atomic,
  enabled = EXCLUDED.enabled
`, toolID, cfg.ServerID, toolName, description, seed.PriceAtomic, enabled)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *store) getToolPricingByName(ctx context.Context, serverID, toolName string) (toolSeed, error) {
	var result toolSeed
	var enabled bool
	err := s.pool.QueryRow(ctx, `
SELECT price_atomic, description, enabled
FROM tool_pricing
WHERE server_id = $1 AND tool_name = $2
`, serverID, toolName).Scan(&result.PriceAtomic, &result.Description, &enabled)
	if err != nil {
		return toolSeed{}, err
	}
	result.Enabled = &enabled
	return result, nil
}

func (s *store) createPaymentRequest(ctx context.Context, input createPaymentRequestInput) (paymentRequestRecord, error) {
	rawRequest, err := json.Marshal(input.RawRequest)
	if err != nil {
		return paymentRequestRecord{}, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return paymentRequestRecord{}, err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
INSERT INTO payment_requests (
  id, server_id, tool_name, amount_atomic, token_mint, recipient, network, scheme,
  status, expires_at, raw_request, raw_response
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, '{}'::jsonb)
`, input.ID, input.ServerID, input.ToolName, input.AmountAtomic, input.TokenMint, input.Recipient, input.Network, input.Scheme, requestStatusChallenged, input.ExpiresAt.UTC(), rawRequest)
	if err != nil {
		return paymentRequestRecord{}, err
	}

	payload, _ := json.Marshal(map[string]any{
		"code":   402,
		"scheme": input.Scheme,
	})
	if _, err := tx.Exec(ctx, `
INSERT INTO request_events (request_id, status, payload, duration_ms)
VALUES ($1, $2, $3, $4)
`, input.ID, requestStatusChallenged, payload, 12); err != nil {
		return paymentRequestRecord{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return paymentRequestRecord{}, err
	}

	return s.getPaymentRequest(ctx, input.ID)
}

func (s *store) getPaymentRequest(ctx context.Context, requestID string) (paymentRequestRecord, error) {
	row := s.pool.QueryRow(ctx, `
SELECT id, server_id, tool_name, payer_wallet, amount_atomic, token_mint, recipient, network,
       scheme, status, failure_reason, tx_signature, created_at, expires_at, settled_at,
       raw_request, raw_response
FROM payment_requests
WHERE id = $1
`, requestID)

	var record paymentRequestRecord
	var rawRequest []byte
	var rawResponse []byte
	err := row.Scan(
		&record.ID,
		&record.ServerID,
		&record.ToolName,
		&record.PayerWallet,
		&record.AmountAtomic,
		&record.TokenMint,
		&record.Recipient,
		&record.Network,
		&record.Scheme,
		&record.Status,
		&record.FailureReason,
		&record.TxSignature,
		&record.CreatedAt,
		&record.ExpiresAt,
		&record.SettledAt,
		&rawRequest,
		&rawResponse,
	)
	if err != nil {
		return paymentRequestRecord{}, err
	}

	record.RawRequest = decodeJSON(rawRequest)
	record.RawResponse = decodeJSON(rawResponse)
	return record, nil
}

func (s *store) markVerified(ctx context.Context, requestID, txSignature, clientWallet string, verifiedAt time.Time) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	commandTag, err := tx.Exec(ctx, `
UPDATE payment_requests
SET status = $2,
    tx_signature = $3,
    payer_wallet = $4,
    settled_at = $5
WHERE id = $1 AND status = $6
`, requestID, requestStatusVerified, txSignature, clientWallet, verifiedAt.UTC(), requestStatusChallenged)
	if err != nil {
		return err
	}
	if commandTag.RowsAffected() != 1 {
		return errors.New("request not in a verifiable state")
	}

	paidPayload, _ := json.Marshal(map[string]any{
		"tx": txSignature,
	})
	if _, err := tx.Exec(ctx, `
INSERT INTO request_events (request_id, status, payload, signature_hash, duration_ms)
VALUES ($1, $2, $3, $4, $5)
`, requestID, "paid", paidPayload, txSignature, 480); err != nil {
		return err
	}

	verifiedPayload, _ := json.Marshal(map[string]any{
		"rpc":           "mock",
		"confirmations": 1,
	})
	if _, err := tx.Exec(ctx, `
INSERT INTO request_events (request_id, status, payload, signature_hash, duration_ms)
VALUES ($1, $2, $3, $4, $5)
`, requestID, requestStatusVerified, verifiedPayload, txSignature, 220); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (s *store) markFailed(ctx context.Context, requestID, failureReason string, payload map[string]any) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	commandTag, err := tx.Exec(ctx, `
UPDATE payment_requests
SET status = $2,
    failure_reason = $3
WHERE id = $1 AND status = $4
`, requestID, requestStatusFailed, failureReason, requestStatusChallenged)
	if err != nil {
		return err
	}
	if commandTag.RowsAffected() == 0 {
		return errors.New("request not in a fail-able state")
	}

	body, _ := json.Marshal(payload)
	if _, err := tx.Exec(ctx, `
INSERT INTO request_events (request_id, status, payload, duration_ms)
VALUES ($1, $2, $3, $4)
`, requestID, requestStatusFailed, body, 220); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (s *store) markExecuted(ctx context.Context, requestID string, rawResponse map[string]any) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	record, err := s.getPaymentRequestTx(ctx, tx, requestID)
	if err != nil {
		return err
	}
	if record.Status == requestStatusExecuted {
		return nil
	}
	if record.Status != requestStatusVerified {
		return errors.New("request is not verified")
	}

	rawResponseJSON, _ := json.Marshal(rawResponse)
	if _, err := tx.Exec(ctx, `
UPDATE payment_requests
SET status = $2,
    raw_response = $3
WHERE id = $1
`, requestID, requestStatusExecuted, rawResponseJSON); err != nil {
		return err
	}

	execPayload, _ := json.Marshal(map[string]any{
		"result": "ok",
	})
	if _, err := tx.Exec(ctx, `
INSERT INTO request_events (request_id, status, payload, signature_hash, duration_ms)
VALUES ($1, $2, $3, $4, $5)
`, requestID, requestStatusExecuted, execPayload, record.TxSignature, 312); err != nil {
		return err
	}

	if record.PayerWallet == nil || record.TxSignature == nil {
		return errors.New("verified request is missing payer wallet or transaction signature")
	}

	receiptID := "rcpt_" + uuid.NewString()
	if _, err := tx.Exec(ctx, `
INSERT INTO receipts (id, request_id, server_id, tool_name, amount_atomic, payer_wallet, tx_signature, block_time)
VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
ON CONFLICT (request_id) DO NOTHING
`, receiptID, requestID, record.ServerID, record.ToolName, record.AmountAtomic, *record.PayerWallet, *record.TxSignature); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (s *store) getPaymentRequestTx(ctx context.Context, tx pgx.Tx, requestID string) (paymentRequestRecord, error) {
	row := tx.QueryRow(ctx, `
SELECT id, server_id, tool_name, payer_wallet, amount_atomic, token_mint, recipient, network,
       scheme, status, failure_reason, tx_signature, created_at, expires_at, settled_at,
       raw_request, raw_response
FROM payment_requests
WHERE id = $1
`, requestID)

	var record paymentRequestRecord
	var rawRequest []byte
	var rawResponse []byte
	err := row.Scan(
		&record.ID,
		&record.ServerID,
		&record.ToolName,
		&record.PayerWallet,
		&record.AmountAtomic,
		&record.TokenMint,
		&record.Recipient,
		&record.Network,
		&record.Scheme,
		&record.Status,
		&record.FailureReason,
		&record.TxSignature,
		&record.CreatedAt,
		&record.ExpiresAt,
		&record.SettledAt,
		&rawRequest,
		&rawResponse,
	)
	if err != nil {
		return paymentRequestRecord{}, err
	}
	record.RawRequest = decodeJSON(rawRequest)
	record.RawResponse = decodeJSON(rawResponse)
	return record, nil
}

func (s *store) listServers(ctx context.Context) ([]serverView, error) {
	rows, err := s.pool.Query(ctx, `
SELECT id, name, endpoint, payout_wallet, webhook_url, network, created_at, api_key
FROM servers
ORDER BY created_at ASC
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []serverView
	for rows.Next() {
		var item serverView
		if err := rows.Scan(&item.ID, &item.Name, &item.Endpoint, &item.PayoutWallet, &item.WebhookURL, &item.Network, &item.CreatedAt, &item.APIKey); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *store) listTools(ctx context.Context, serverID string) ([]toolView, error) {
	rows, err := s.pool.Query(ctx, `
SELECT t.id, t.server_id, t.tool_name, t.description, t.price_atomic, t.enabled,
       COALESCE(c.calls_last_24h, 0) AS calls_last_24h
FROM tool_pricing t
LEFT JOIN (
  SELECT server_id, tool_name, COUNT(*) AS calls_last_24h
  FROM payment_requests
  WHERE created_at >= NOW() - INTERVAL '24 hours'
  GROUP BY server_id, tool_name
) c ON c.server_id = t.server_id AND c.tool_name = t.tool_name
WHERE t.server_id = $1
ORDER BY t.tool_name ASC
`, serverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []toolView
	for rows.Next() {
		var item toolView
		var priceAtomic int64
		if err := rows.Scan(&item.ID, &item.ServerID, &item.ToolName, &item.Description, &priceAtomic, &item.Enabled, &item.CallsLast24h); err != nil {
			return nil, err
		}
		item.PriceUSDC = atomicToUSDC(priceAtomic)
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *store) listRequests(ctx context.Context, serverID, status string) ([]requestView, error) {
	query := `
SELECT id, server_id, tool_name, payer_wallet, amount_atomic, status, failure_reason, tx_signature,
       created_at, settled_at, raw_request, raw_response
FROM payment_requests
WHERE 1=1`
	args := []any{}
	if serverID != "" {
		args = append(args, serverID)
		query += fmt.Sprintf(" AND server_id = $%d", len(args))
	}
	if status != "" {
		args = append(args, status)
		query += fmt.Sprintf(" AND status = $%d", len(args))
	}
	query += " ORDER BY created_at DESC"

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []requestView
	for rows.Next() {
		record, err := scanRequestRow(rows)
		if err != nil {
			return nil, err
		}
		events, err := s.listRequestEvents(ctx, record.ID)
		if err != nil {
			return nil, err
		}
		result = append(result, toRequestView(record, events))
	}
	return result, rows.Err()
}

func (s *store) getRequestView(ctx context.Context, requestID string) (requestView, error) {
	record, err := s.getPaymentRequest(ctx, requestID)
	if err != nil {
		return requestView{}, err
	}
	events, err := s.listRequestEvents(ctx, requestID)
	if err != nil {
		return requestView{}, err
	}
	return toRequestView(record, events), nil
}

func (s *store) listRequestEvents(ctx context.Context, requestID string) ([]requestEvent, error) {
	rows, err := s.pool.Query(ctx, `
SELECT status, payload, signature_hash, duration_ms, created_at
FROM request_events
WHERE request_id = $1
ORDER BY created_at ASC, id ASC
`, requestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []requestEvent
	for rows.Next() {
		var item requestEvent
		var payloadBytes []byte
		var createdAt time.Time
		if err := rows.Scan(&item.Status, &payloadBytes, &item.SignatureHash, &item.DurationMs, &createdAt); err != nil {
			return nil, err
		}
		item.Timestamp = createdAt.UTC().Format(time.RFC3339)
		item.Payload = decodeJSON(payloadBytes)
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *store) listReceipts(ctx context.Context, serverID string) ([]receiptView, error) {
	query := `
SELECT id, request_id, server_id, tool_name, amount_atomic, payer_wallet, tx_signature, block_time, created_at
FROM receipts`
	args := []any{}
	if serverID != "" {
		query += " WHERE server_id = $1"
		args = append(args, serverID)
	}
	query += " ORDER BY created_at DESC"

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []receiptView
	for rows.Next() {
		var item receiptView
		var amountAtomic int64
		var blockTime time.Time
		var createdAt time.Time
		if err := rows.Scan(&item.ID, &item.RequestID, &item.ServerID, &item.ToolName, &amountAtomic, &item.PayerWallet, &item.TxSignature, &blockTime, &createdAt); err != nil {
			return nil, err
		}
		item.AmountUSDC = atomicToUSDC(amountAtomic)
		item.BlockTime = blockTime.UTC().Format(time.RFC3339)
		item.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *store) getDashboardSummary(ctx context.Context, serverID string) (dashboardSummaryView, error) {
	var summary dashboardSummaryView
	var totalRevenueAtomic int64
	var avgSettlementMs float64
	args := []any{}
	filter := ""
	if serverID != "" {
		filter = "WHERE server_id = $1"
		args = append(args, serverID)
	}

	if err := s.pool.QueryRow(ctx, fmt.Sprintf(`
SELECT
  COALESCE(SUM(amount_atomic), 0),
  COUNT(*) FILTER (WHERE status IN ('verified', 'executed')),
  COUNT(*) FILTER (WHERE status = 'failed' AND failure_reason <> 'timeout'),
  COALESCE(AVG(EXTRACT(EPOCH FROM (settled_at - created_at)) * 1000) FILTER (WHERE settled_at IS NOT NULL), 0)
FROM payment_requests
%s
`, filter), args...).Scan(&totalRevenueAtomic, &summary.PaidRequests, &summary.FailedVerifications, &avgSettlementMs); err != nil {
		return dashboardSummaryView{}, err
	}

	summary.TotalRevenueUSDC = atomicToUSDC(totalRevenueAtomic)
	summary.AvgSettlementMs = int64(avgSettlementMs)

	args = []any{}
	filter = ""
	if serverID != "" {
		filter = "WHERE server_id = $1"
		args = append(args, serverID)
	}
	if err := s.pool.QueryRow(ctx, fmt.Sprintf(`
SELECT
  COUNT(*) FILTER (WHERE price_atomic > 0 AND enabled),
  COUNT(*) FILTER (WHERE price_atomic = 0 AND enabled)
FROM tool_pricing
%s
`, filter), args...).Scan(&summary.PaidToolCount, &summary.FreeToolCount); err != nil {
		return dashboardSummaryView{}, err
	}

	summary.RevenueDelta7d = 0
	summary.PaidDelta7d = 0
	return summary, nil
}

func scanRequestRow(scanner interface {
	Scan(...any) error
}) (paymentRequestRecord, error) {
	var record paymentRequestRecord
	var rawRequest []byte
	var rawResponse []byte
	err := scanner.Scan(
		&record.ID,
		&record.ServerID,
		&record.ToolName,
		&record.PayerWallet,
		&record.AmountAtomic,
		&record.Status,
		&record.FailureReason,
		&record.TxSignature,
		&record.CreatedAt,
		&record.SettledAt,
		&rawRequest,
		&rawResponse,
	)
	if err != nil {
		return paymentRequestRecord{}, err
	}
	record.RawRequest = decodeJSON(rawRequest)
	record.RawResponse = decodeJSON(rawResponse)
	return record, nil
}

func toRequestView(record paymentRequestRecord, timeline []requestEvent) requestView {
	settledAt := timePtrToString(record.SettledAt)
	payerWallet := ""
	if record.PayerWallet != nil {
		payerWallet = *record.PayerWallet
	}

	return requestView{
		ID:            record.ID,
		ServerID:      record.ServerID,
		ToolName:      record.ToolName,
		PayerWallet:   payerWallet,
		AmountUSDC:    atomicToUSDC(record.AmountAtomic),
		Status:        record.Status,
		FailureReason: record.FailureReason,
		TxSignature:   record.TxSignature,
		CreatedAt:     record.CreatedAt.UTC().Format(time.RFC3339),
		SettledAt:     settledAt,
		Timeline:      timeline,
		RawRequest:    record.RawRequest,
		RawResponse:   record.RawResponse,
	}
}

func atomicToUSDC(amount int64) float64 {
	return float64(amount) / 1_000_000
}

func timePtrToString(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.UTC().Format(time.RFC3339)
	return &formatted
}

func decodeJSON(data []byte) map[string]any {
	if len(data) == 0 {
		return map[string]any{}
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return map[string]any{}
	}
	return result
}

func sanitizeIdentifier(value string) string {
	replacer := strings.NewReplacer(".", "_", "-", "_", "/", "_", " ", "_")
	return replacer.Replace(strings.ToLower(value))
}

var _ sql.Scanner
