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

- operator write endpoints (pricing is configured through `TOOL_PRICING_JSON`, not the dashboard)
- operator auth
- rate limiting

Phases 2 and 3 are now implemented, including real Solana transaction inspection and forwarding to
a live MCP server. See the phase sections below.

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

Verification now runs on chain. The verifier has two modes, selected by `SOLANA_VERIFY_MODE`:

- `mock` trusts the observed fields in the request and is meant for local demos.
- `rpc` fetches the transaction with `getTransaction`, sums the recipient's token-balance change
  across its accounts, and checks the payer signed it, the mint and network match, and the amount
  covers the quote.

Two guarantees were added on top of the SRS to close real attacks:

- **Payment binding.** Each challenge carries a unique reference key. The buyer attaches it to the
  transfer as a read-only account, and the verifier requires it on chain. Without this, any payment
  to the seller for the right amount could be redeemed against someone else's challenge.
- **Idempotent, permanent replay claims.** A signature is claimed once, forever, keyed to the
  request that spent it. The same request can retry safely after a transient failure; any other
  request reusing that signature is rejected.

Acceptance runbook:

- [docs/PHASE2-PERSISTENCE.md](/mnt/c/Users/vg890/OneDrive/Desktop/x402%20for%20Solana%20MCP/docs/PHASE2-PERSISTENCE.md:1)

## Phase 3 status

Phase 3 wraps a real MCP server end to end:

- the gateway forwards settled paid calls to an upstream MCP server (`UPSTREAM_MCP_URL`), replaying
  the exact request that was paid for
- an example seller server (`apps/example-server`) with a free `ping` tool and paid `premium.search`
  and `premium.codegen` tools
- an example buyer client (`apps/example-client`) that pays on devnet with a real SPL transfer, or
  in a mock mode for offline runs
- `GET /v1/receipts/:requestId` for receipt lookup, with a response hash tying the receipt to the
  delivered result
- execution is serialized per request, so concurrent retries call the upstream at most once

Acceptance runbook:

- [docs/PHASE3-E2E.md](/mnt/c/Users/vg890/OneDrive/Desktop/x402%20for%20Solana%20MCP/docs/PHASE3-E2E.md:1)

## Phase 4 status

The operator dashboard (`apps/frontend`) reads live gateway data and can now change pricing:

- `PATCH /v1/servers/:serverId/tools/:toolName` updates a tool's price and/or enabled flag; the
  next challenge reflects it immediately
- `TOOL_PRICING_JSON` seeds a tool once — after that the operator owns price and enabled, so a
  restart no longer clobbers dashboard edits
- the gateway sends CORS headers (`DASHBOARD_ORIGIN`) so the dashboard can call it from the browser
- the dashboard Tools page edits price and enabled inline, persisting through the gateway with an
  optimistic update that rolls back on error

Point the dashboard at the gateway with `VITE_API_BASE_URL` (see `apps/frontend/.env.example`).
Operator write endpoints for registering servers and per-tool auth are still out of scope.

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

## Local infra without Docker

If Docker is not available, install Postgres and Redis natively (`postgres` + `redis-server`) and
point the services at them with `POSTGRES_URL` and `REDIS_URL`. The Go test suites need neither —
they run against an embedded Postgres and an in-process Redis on their own.

## Recommended next milestones

1. Connect the frontend dashboard to the live gateway endpoints.
2. Add operator write endpoints (register server, set per-tool pricing) so the dashboard can
   configure pricing instead of `TOOL_PRICING_JSON`.
3. Add rate limits and structured logs to the challenge and verify paths.
4. Add operator auth for the dashboard and operator endpoints.
5. Move from devnet to a mainnet facilitator behind the existing verifier interface.
