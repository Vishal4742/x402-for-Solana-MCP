# Software Requirements Specification

## Project

**Name:** x402 for Solana MCP Servers  
**Date:** 2026-04-14  
**Version:** 0.1 MVP  
**Goal:** Let founders monetize MCP servers and agent tools using wallet-based USDC payments on Solana with minimal setup.

## 1. Problem Statement

Founders shipping agent products can expose tools via MCP, but billing those tools is still clumsy. Current options are:

- API keys with manual credit tracking
- Stripe subscriptions disconnected from tool-level usage
- enterprise invoicing
- custom wallet/payment logic per product

This creates two problems:

1. developers cannot charge per tool call, session, or workflow with low integration effort
2. agents and users cannot pay through an open wallet-native flow that fits machine-to-machine usage

## 2. Product Vision

Build a payment gateway for MCP servers on Solana that:

- prices MCP tools in USDC
- returns a payment challenge before execution
- verifies payment
- executes the requested MCP tool after settlement
- records receipts and usage for operators

The MVP should work for one seller, one chain, one token, and one MCP transport path.

## 3. Target Users

### Primary

- founders shipping paid MCP servers
- teams exposing agent tools as APIs
- Solana-native builders who want wallet-based billing instead of Stripe-only billing

### Secondary

- agent framework builders
- infra teams selling premium tools to AI agents

## 4. Goals

### MVP Goals

- charge for MCP tool calls using USDC on Solana
- support fixed pricing per tool
- support wallet-based payment verification
- allow MCP server operators to wrap an existing server with minimal code changes
- show receipts, request logs, and payment status
- provide one example server and one example client

### Non-Goals for MVP

- subscriptions
- multi-chain support
- complex revenue sharing
- escrow and disputes
- onchain reputation
- fiat onramp/offramp
- generalized marketplace discovery

## 5. Why Crypto Is Necessary

This product fails the thesis if it becomes "Stripe for APIs with a token sticker." Crypto is necessary here because:

- settlement can happen wallet-to-wallet without a trusted billing intermediary
- agents and users can pay with programmable wallets
- pricing and receipts can be tied to open wallet identity
- the flow composes with Solana-native applications and agent wallets

If users only want monthly card billing, this product becomes a weaker SaaS wrapper.

## 6. Core MVP Use Cases

### UC-1 Paid tool invocation

1. client requests an MCP tool
2. gateway checks pricing policy
3. gateway returns payment challenge
4. client signs/pays in USDC on Solana
5. gateway verifies settlement
6. gateway forwards call to underlying MCP server
7. response is returned with payment receipt

### UC-2 Operator configures pricing

1. operator registers an MCP server
2. operator sets seller wallet
3. operator sets price per tool
4. operator enables paid routes

### UC-3 Operator inspects usage

1. operator opens dashboard
2. operator sees paid requests, failed requests, revenue, and receipt status

### UC-4 Client retries after payment

1. client gets `402 Payment Required`
2. client completes payment
3. client retries with payment proof
4. request succeeds

## 7. Functional Requirements

### 7.1 Gateway Layer

- The system must expose an HTTP gateway in front of an MCP server.
- The gateway must support a paid endpoint for MCP tool invocations.
- The gateway must support pass-through for unpaid tools.
- The gateway must return `402 Payment Required` when a paid tool is requested without valid payment.
- The gateway must attach pricing metadata to the challenge response.

### 7.2 Payment Verification

- The system must verify Solana payment proof before executing a paid tool.
- The system must initially support `USDC` only.
- The system must initially support one configured network: `devnet`.
- The verification module must be abstracted behind a facilitator interface so production Solana facilitators can be swapped in later.
- The system must reject duplicate or replayed payment proofs.

### 7.3 Pricing

- The system must allow tool-level fixed prices.
- The system must allow a default server-wide price.
- The operator must be able to disable payments for selected tools.

### 7.4 MCP Integration

- The system must support wrapping an existing MCP server without changing the business logic of the server.
- The system must preserve MCP request and response structure.
- The system must support MCP tool execution over one transport path in MVP.

### 7.5 Dashboard

- The system must show:
  - server name
  - tools and prices
  - paid requests
  - failed payments
  - revenue summary
  - latest receipts

### 7.6 Receipts and Logs

- The system must store a payment receipt per paid invocation.
- The system must store request metadata:
  - request id
  - server id
  - tool name
  - client wallet
  - amount
  - token
  - network
  - transaction signature
  - status
  - timestamps

### 7.7 Developer Experience

- The system must provide a starter SDK or middleware package for Node.js.
- The system must provide:
  - example MCP seller server
  - example buyer/client
  - local setup guide
  - devnet demo script

## 8. Non-Functional Requirements

### Performance

- Payment challenge response should return in under `500ms` excluding blockchain confirmation.
- Tool execution overhead added by the gateway should stay under `300ms` after payment verification is complete.

### Reliability

- The system must not execute a paid tool if payment verification fails.
- The system must tolerate temporary RPC or facilitator errors and return explicit retry-safe errors.

### Security

- Seller private keys must never be stored by the gateway.
- The system must support delegated verification rather than raw hot-wallet signing in the core service.
- The system must validate payment amount, token mint, recipient wallet, and network.
- The system must prevent replay of request ids and payment proofs.
- The system must redact secrets from logs.

### Maintainability

- The payment provider must be modular.
- The MCP wrapper must be framework-agnostic where possible.
- Core code should be split into `gateway`, `verifier`, `pricing`, `store`, and `dashboard` modules.

## 9. Proposed MVP Architecture

## Components

1. **MCP Gateway**
   - intercepts incoming tool calls
   - checks pricing rules
   - issues payment challenge
   - forwards verified calls to underlying MCP server

2. **Payment Verifier**
   - validates payment proof
   - checks Solana transaction details
   - marks receipts as settled

3. **Pricing Engine**
   - resolves price by tool name
   - generates quoted amount and token

4. **Receipt Store**
   - stores request/payment state
   - enables idempotency and replay protection

5. **Operator Dashboard**
   - configures tools and pricing
   - views receipts and revenue

6. **Example Seller Server**
   - demonstrates one paid MCP tool and one free tool

## 10. Suggested Tech Stack

### Backend

- `TypeScript`
- `Node.js`
- `Fastify` or `Hono`
- MCP server SDK for Node

### Data

- `Postgres` for receipts and configs
- `Prisma` or `Drizzle`
- `Redis` optional for request nonce cache

### Solana

- `@solana/web3.js`
- SPL token helpers for USDC mint handling
- Solana RPC provider such as Helius or QuickNode

### Frontend

- `Next.js`
- simple operator dashboard

### Auth

- wallet sign-in for operator dashboard, or simple email/password for MVP if speed matters

## 11. API Surface

### Public Endpoints

- `POST /mcp/:serverId`
  - entry point for MCP tool requests

- `GET /v1/challenge/:requestId`
  - returns payment challenge details

- `POST /v1/verify`
  - submits payment proof

- `GET /v1/receipts/:requestId`
  - fetches receipt status

### Operator Endpoints

- `POST /v1/servers`
- `POST /v1/servers/:id/tools`
- `PATCH /v1/tools/:id/pricing`
- `GET /v1/dashboard/summary`
- `GET /v1/dashboard/receipts`

## 12. Data Model

### Server

- `id`
- `name`
- `owner_wallet`
- `base_url`
- `network`
- `status`
- `created_at`

### ToolPricing

- `id`
- `server_id`
- `tool_name`
- `price_atomic`
- `token_mint`
- `is_paid`
- `created_at`

### PaymentRequest

- `id`
- `server_id`
- `tool_name`
- `client_wallet`
- `amount_atomic`
- `token_mint`
- `network`
- `status`
- `challenge_payload`
- `tx_signature`
- `created_at`
- `settled_at`

### Receipt

- `id`
- `payment_request_id`
- `execution_status`
- `mcp_response_hash`
- `created_at`

## 13. Key Flows

### Challenge Flow

1. client calls paid tool
2. gateway creates `payment_request`
3. gateway returns `402` with:
   - request id
   - amount
   - token mint
   - recipient
   - network
   - expiry

### Verification Flow

1. client pays on Solana
2. client submits transaction signature or facilitator proof
3. verifier checks:
   - transaction exists
   - recipient matches seller wallet
   - amount meets quote
   - token mint matches USDC
   - request id is unused
4. system marks request settled

### Execution Flow

1. paid request is settled
2. gateway forwards original MCP tool call
3. response is returned
4. receipt is stored

## 14. Security Requirements

- Use short-lived payment challenges with expiry.
- Enforce idempotency keys per request.
- Reject mismatched wallets, token mints, or underpayments.
- Add rate limits to challenge and verify endpoints.
- Log all failed payment attempts.
- Avoid custodial holding of buyer funds in MVP.
- Keep manual reconciliation path for failed post-payment execution.

## 15. Success Metrics

### MVP KPIs

- time to integrate existing MCP server under `30 minutes`
- at least `3` tool calls successfully paid on devnet in demo
- at least `2` external founders test the flow
- paid tool invocation success rate above `90%` in demo runs

## 16. Delivery Plan

### Week 1

- scaffold gateway and DB
- define pricing config model
- implement 402 challenge flow
- implement Solana devnet verification
- wrap one example MCP server

### Week 2

- add operator dashboard
- add receipts/log page
- add replay protection and rate limits
- write docs and demo script
- record walkthrough video

## 17. Risks and Mitigations

### Risk: standards move quickly

Mitigation: isolate payment logic behind an adapter and avoid hard-coding one facilitator.

### Risk: users prefer Stripe

Mitigation: focus initial wedge on wallet-native agent tools and Solana-native builders, not general API billing.

### Risk: payment verification edge cases

Mitigation: stay on one token, one chain, and one request model for MVP.

### Risk: weak distribution

Mitigation: ship as a wrapper for existing MCP servers so founders can test quickly.

## 18. MVP Build Recommendation

Build the first version as:

- one `Node.js` gateway
- one `Next.js` dashboard
- one `Postgres` database
- one example MCP tool server
- one Solana devnet payment verifier

Do not start with:

- custom onchain programs
- multi-chain logic
- subscriptions
- dynamic pricing
- enterprise auth

## 19. Open Questions

- Which production Solana facilitator should be the default after devnet?
- Should verification rely on raw transaction inspection, facilitator proof, or both?
- Should the first client be human-wallet driven, agent-wallet driven, or both?
- Is the first distribution channel an SDK, hosted gateway, or OSS reverse proxy?
