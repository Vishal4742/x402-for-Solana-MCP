# x402 for Solana MCP

Go-first scaffold for an MCP payment gateway on Solana.

## Chosen stack

- `Go + Chi` for the gateway and verifier hot path
- `Postgres` for receipts, pricing, and idempotency state
- `Redis` for short-lived challenge cache, replay locks, and rate limits
- `TypeScript` SDK for seller and buyer integration

## Why this stack

This project needs two different things:

- low-overhead request handling and verification logic
- easy integration into MCP servers that are usually built in Node/TypeScript

Keeping the gateway in Go and the SDK in TypeScript gives you both.

## Repo layout

```text
.
├── services
│   ├── gateway
│   └── verifier
├── packages
│   └── sdk
├── docker-compose.yml
├── go.work
├── package.json
└── pnpm-workspace.yaml
```

## Service boundaries

### `services/gateway`

- receives MCP tool requests
- resolves pricing
- issues `402 Payment Required` challenges
- checks settled status before execution
- will later proxy the verified request to the underlying MCP server

### `services/verifier`

- validates Solana payment proofs
- checks recipient, mint, amount, and replay conditions
- will later be called by the gateway or consume a verification queue

### `packages/sdk`

- wraps buyer-side retry flow
- normalizes `402` challenge responses
- provides helper functions for sellers and test clients

## Current scaffold status

This scaffold includes:

- a `Chi` gateway with persisted payment requests, challenge lookup, dashboard endpoints, and MCP retry flow
- a verifier service with replay detection and mock Solana payment checks
- local Postgres and Redis via Docker Compose
- a TypeScript SDK skeleton for the challenge -> verify -> retry flow

What is still intentionally stubbed:

- actual Solana RPC transaction inspection
- proxying to a live MCP server
- operator auth
- rate limiting

## Phase 1 status

Phase 1 gateway core is implemented:

- configurable tool pricing via `TOOL_PRICING_JSON`
- free tool passthrough
- `402 Payment Required` challenge flow
- challenge lookup by request id
- deterministic retry using `X-Payment-Request-Id`

Current contract notes:

- challenge lookup uses status `pending`
- the current gateway runtime depends on Postgres and Redis for startup, even when running the basic Phase 1 challenge demo

Acceptance runbook:

- [docs/PHASE1-GATEWAY.md](/mnt/c/Users/vg890/OneDrive/Desktop/x402%20for%20Solana%20MCP/docs/PHASE1-GATEWAY.md:1)

## Phase 2 status

Phase 2 verification and persistence are implemented in code:

- Postgres-backed `Server`, `ToolPricing`, `PaymentRequest`, `RequestEvent`, and `Receipt` state
- Redis-backed verify lock in the gateway
- verifier replay detection keyed by transaction signature
- persisted challenge, verify, fail, and execute timelines
- dashboard-facing endpoints for servers, tools, requests, receipts, and summary

Acceptance runbook:

- [docs/PHASE2-PERSISTENCE.md](/mnt/c/Users/vg890/OneDrive/Desktop/x402%20for%20Solana%20MCP/docs/PHASE2-PERSISTENCE.md:1)

## Start here

- [docs/START-HERE.md](/mnt/c/Users/vg890/OneDrive/Desktop/x402%20for%20Solana%20MCP/docs/START-HERE.md:1)
- [docs/PHASES.md](/mnt/c/Users/vg890/OneDrive/Desktop/x402%20for%20Solana%20MCP/docs/PHASES.md:1)

## Local development

1. Copy `.env.example` to `.env`
2. Start infra:

```bash
docker compose up -d
```

3. Run the Go services after installing Go:

```bash
cd services/gateway && go run ./cmd/gateway
cd services/verifier && go run ./cmd/verifier
```

4. Build the SDK:

```bash
pnpm install
pnpm --filter @x402/sdk build
```

## Recommended next milestones

1. Replace mock verification with real Solana devnet USDC transaction inspection.
2. Forward settled requests to an example MCP server.
3. Build the example buyer and seller apps around the TypeScript SDK.
4. Connect the frontend dashboard to the live gateway endpoints.
5. Add rate limits, structured logs, and retry-safe hardening.
