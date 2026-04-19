# Project Phases

This project should be executed as six phases. Each phase has a goal, concrete outputs, and an exit gate.

## Phase 0: Bootstrap

### Goal

Make the repo runnable locally with a predictable development setup.

### Outputs

- Go services scaffolded
- local `docker-compose` for Postgres and Redis
- `.env.example`
- TypeScript SDK workspace
- start and architecture docs

### Exit gate

- `gateway` starts
- `verifier` starts
- `docker compose up -d` works
- SDK builds successfully

## Phase 1: Gateway Core

### Goal

Make the payment challenge flow work before touching real verification.

### Outputs

- `POST /mcp/:serverId`
- `GET /v1/challenge/:requestId`
- paid tool map or pricing config
- free tool passthrough
- `402 Payment Required` response with challenge payload

### Acceptance demo

1. Call free tool
2. Get immediate success
3. Call paid tool
4. Receive `402` challenge with amount, mint, recipient, network, and expiry

### Main files

- `services/gateway/cmd/gateway/main.go`

## Phase 2: Verification and Persistence

### Goal

Replace placeholders with real system state and replay protection.

### Outputs

- Postgres models for `Server`, `ToolPricing`, `PaymentRequest`, and `Receipt`
- Redis-backed replay lock and idempotency handling
- verifier service that checks:
  - transaction exists
  - recipient matches
  - mint matches USDC
  - amount is sufficient
  - request is not replayed
- settled state persisted before execution

### Acceptance demo

1. Paid request creates a database record
2. Valid payment settles exactly once
3. Duplicate verification attempt is rejected
4. Underpayment or wrong mint is rejected

### Main files

- `services/gateway`
- `services/verifier`

## Phase 3: End-to-End MCP Demo

### Goal

Prove that an existing MCP server can be wrapped with minimal friction.

### Outputs

- example seller server with:
  - one free tool
  - one paid tool
- example buyer client using `@x402/sdk`
- gateway forwarding verified paid calls to the seller server
- receipt lookup flow

### Acceptance demo

1. Buyer calls paid tool
2. Gateway returns `402`
3. Buyer pays on devnet
4. Buyer verifies payment
5. Buyer retries with request id
6. Tool executes successfully
7. Receipt can be retrieved

### Main files

- `apps/example-server`
- `apps/example-client`
- `packages/sdk`

## Phase 4: Operator Dashboard

### Goal

Give the operator visibility and control without slowing the payment core.

### Outputs

- pricing configuration UI
- receipt list
- failed payment list
- revenue summary
- latest request activity

### Acceptance demo

1. Operator updates a tool price
2. New challenge reflects updated price
3. Operator sees latest settled and failed requests

### Main files

- `apps/dashboard`

## Phase 5: Hardening and Launch Prep

### Goal

Make the MVP stable enough for external founders to try.

### Outputs

- rate limiting
- structured logging
- retry-safe errors
- health checks
- setup docs
- demo script
- walkthrough video outline

### Acceptance demo

- full devnet flow works repeatedly without manual cleanup
- at least two external founders can follow the setup and understand the value

## Suggested timeline

### Week 1

- Phase 0
- Phase 1
- start Phase 2

### Week 2

- finish Phase 2
- Phase 3
- begin Phase 4 only if Phase 3 is stable

### Week 3

- Phase 4
- Phase 5

## Non-negotiables

- keep one chain, one token, one flow
- settle before execution
- do not build the dashboard before the payment path works
- optimize for demo reliability before feature breadth

