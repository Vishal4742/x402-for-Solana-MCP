# Phase 2 Runbook

This runbook verifies the Postgres-backed request lifecycle, Redis replay protection, and verifier integration.

## What Phase 2 adds

- Postgres state for:
  - `servers`
  - `tool_pricing`
  - `payment_requests`
  - `request_events`
  - `receipts`
- Redis verify lock in the gateway
- Redis replay detection in the verifier
- dashboard endpoints:
  - `GET /v1/servers`
  - `GET /v1/servers/:serverId/tools`
  - `GET /v1/requests`
  - `GET /v1/requests/:requestId`
  - `GET /v1/dashboard/summary`
  - `GET /v1/dashboard/receipts`

## Prerequisites

- Go `1.23+`
- Docker Desktop or another local Postgres + Redis setup
- copied `.env.example` to `.env`

Start infra:

```bash
docker compose up -d
```

Start the verifier:

```bash
cd services/verifier
env GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod-cache go run ./cmd/verifier
```

Start the gateway:

```bash
cd services/gateway
env GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod-cache go run ./cmd/gateway
```

## Health checks

Verifier:

```bash
curl http://127.0.0.1:8081/healthz
```

Gateway:

```bash
curl http://127.0.0.1:8080/healthz
```

Both should return `{"status":"ok",...}`.

## Acceptance flow

### 1. Create a paid request

```bash
curl -sS -X POST http://127.0.0.1:8080/mcp/srv_01HX3K \
  -H 'Content-Type: application/json' \
  -d '{"tool":"premium.search","input":{"query":"solana mcp"}}'
```

Expected:

- HTTP `402`
- challenge payload with:
  - `requestId`
  - `amountAtomic`
  - `tokenMint`
  - `recipient`
  - `network`
  - `expiresAt`

### 2. Inspect the persisted request

```bash
curl -sS http://127.0.0.1:8080/v1/requests
```

Expected:

- one request with status `challenged`
- timeline includes the initial challenge event

### 3. Verify a valid payment exactly once

Replace `<REQUEST_ID>` with the challenge request id.

```bash
curl -sS -X POST http://127.0.0.1:8080/v1/verify \
  -H 'Content-Type: application/json' \
  -d '{"requestId":"<REQUEST_ID>","txSignature":"sig-valid-001","clientWallet":"wallet-devnet-001"}'
```

Expected:

- HTTP `200`
- status `verified`

### 4. Retry the tool call with the request id

```bash
curl -sS -X POST http://127.0.0.1:8080/mcp/srv_01HX3K \
  -H 'Content-Type: application/json' \
  -H 'X-Payment-Request-Id: <REQUEST_ID>' \
  -d '{"tool":"premium.search","input":{"query":"solana mcp"}}'
```

Expected:

- HTTP `200`
- status `executed`

### 5. Confirm receipt and summary state

```bash
curl -sS http://127.0.0.1:8080/v1/dashboard/receipts
curl -sS http://127.0.0.1:8080/v1/dashboard/summary
curl -sS http://127.0.0.1:8080/v1/requests/<REQUEST_ID>
```

Expected:

- receipt exists for the request
- summary shows paid requests and revenue
- request timeline includes:
  - `challenged`
  - `paid`
  - `verified`
  - `executed`

## Failure checks

### Duplicate verification attempt

```bash
curl -sS -X POST http://127.0.0.1:8080/v1/verify \
  -H 'Content-Type: application/json' \
  -d '{"requestId":"<REQUEST_ID>","txSignature":"sig-valid-001","clientWallet":"wallet-devnet-002"}'
```

Expected:

- HTTP `409`
- failure reason reflects replay detection or already-verified request state

### Underpayment

Create a fresh request id first, then:

```bash
curl -sS -X POST http://127.0.0.1:8080/v1/verify \
  -H 'Content-Type: application/json' \
  -d '{"requestId":"<NEW_REQUEST_ID>","txSignature":"sig-underpay-001","clientWallet":"wallet-devnet-003","amountAtomic":500000}'
```

Expected:

- HTTP `409`
- request stored as `failed`
- failure reason `underpayment`

### Wrong mint

Create a fresh request id first, then:

```bash
curl -sS -X POST http://127.0.0.1:8080/v1/verify \
  -H 'Content-Type: application/json' \
  -d '{"requestId":"<NEW_REQUEST_ID>","txSignature":"sig-wrong-mint-001","clientWallet":"wallet-devnet-004","tokenMint":"WrongMint1111111111111111111111111111111111"}'
```

Expected:

- HTTP `409`
- request stored as `failed`
- failure reason `token_mint_mismatch`

## Notes

- `SOLANA_VERIFY_MODE=mock` is the current Phase 2 default.
- The verifier still needs real Solana RPC transaction inspection to complete the devnet payment path.
