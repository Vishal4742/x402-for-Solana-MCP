# Phase 1 Gateway Demo

This runbook covers the Phase 1 acceptance flow for the Go gateway.

## Goal

Prove:

1. free tools pass through immediately
2. paid tools return `402 Payment Required`
3. the challenge contains amount, mint, recipient, network, and expiry

## Prerequisites

- Go installed locally
- Docker Desktop running locally
- `.env` created from `.env.example`
- `SELLER_WALLET` set in `.env`

Start infrastructure:

```bash
docker compose up -d
```

Start the gateway:

```bash
cd services/gateway
go run ./cmd/gateway
```

## Phase 1 acceptance flow

### 1. Call a free tool

```bash
curl -s http://localhost:8080/mcp/srv_01HX3K \
  -H "Content-Type: application/json" \
  -d '{"tool":"ping","input":{"message":"hello"}}'
```

Expected:

- `200 OK`
- response status is `executed`
- result says `free tool executed`

### 2. Call a paid tool

```bash
curl -i http://localhost:8080/mcp/srv_01HX3K \
  -H "Content-Type: application/json" \
  -d '{"tool":"premium.search","input":{"query":"solana mcp"}}'
```

Expected:

- `402 Payment Required`
- response body contains:
  - `challenge.requestId`
  - `challenge.amountAtomic`
  - `challenge.tokenMint`
  - `challenge.recipient`
  - `challenge.network`
  - `challenge.expiresAt`
  - `pricing.scheme`
  - `retry.header`

### 3. Read the challenge directly

Replace `REQUEST_ID` with the value from the previous response.

```bash
curl -s http://localhost:8080/v1/challenge/REQUEST_ID
```

Expected:

- `200 OK`
- challenge status is `pending`

## Optional retry flow

Phase 1 does not require real verification, but the scaffold supports a mocked settle-and-retry loop:

```bash
curl -s http://localhost:8080/v1/verify \
  -H "Content-Type: application/json" \
  -d '{"requestId":"REQUEST_ID","txSignature":"mock_sig","clientWallet":"mock_wallet"}'
```

Then retry:

```bash
curl -s http://localhost:8080/mcp/srv_01HX3K \
  -H "Content-Type: application/json" \
  -H "X-Payment-Request-Id: REQUEST_ID" \
  -d '{"tool":"premium.search","input":{"query":"solana mcp"}}'
```

Expected:

- `200 OK`
- response status is `executed`
- result says `paid tool executed after settlement`

## Notes

- Pricing is configured via `TOOL_PRICING_JSON`
- Unlisted tools are free unless `DEFAULT_TOOL_PRICE_ATOMIC` is set
- The current gateway boot path requires Postgres and Redis even for the Phase 1 challenge demo
- `POST /v1/verify` is available for the mocked settle-and-retry loop, but real Solana inspection remains a later phase concern
