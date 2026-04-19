# Build Context

## Project

- Name: x402 for Solana MCP Servers
- Phase: build
- Goal: ship an MVP that wraps an existing MCP server, returns a `402 Payment Required` challenge for paid tools, verifies Solana devnet USDC payment, then executes the tool and stores a receipt

## Product Wedge

- Positioning: MCP-first wallet-native billing for Solana tool operators
- Core promise: wrap one existing MCP server and start charging for one premium tool in under 30 minutes
- First users: founders shipping MCP tools, agent APIs, and private infra tools
- Non-goal: general API billing platform or broad multi-chain payments infra

## Build Decisions

- Backend: Go
- Gateway framework: Chi
- MCP integration: Go gateway proxying to MCP servers, with a TypeScript SDK for integration
- Dashboard: Next.js later in the roadmap, after the payment path is stable
- Database: Postgres
- Cache: Redis
- Network: Solana devnet only
- Token: USDC only
- Verification model: adapter interface with one initial Solana transaction verifier
- Auth: minimal operator auth for MVP, prefer wallet sign-in only if it does not slow shipping

## Architecture

- `services/gateway`: HTTP gateway that receives MCP requests, resolves pricing, creates payment challenges, verifies payment proofs, and forwards paid requests
- `services/verifier`: Solana payment verification service behind an adapter boundary
- `apps/dashboard`: operator UI for tool pricing, receipts, and revenue summary
- `apps/example-server`: example MCP server with one free tool and one paid tool
- `apps/example-client`: example buyer/client flow for challenge -> pay -> verify -> retry
- `packages/sdk`: seller and buyer integration helpers in TypeScript

## MVP Milestones

1. Gateway core complete: pricing config, `402` challenge response, challenge lookup, and retry header flow in Go
2. Solana devnet USDC verification plus replay protection and receipt persistence
3. Example MCP seller server and buyer client end-to-end demo using the TypeScript SDK
4. Minimal dashboard for tool pricing, receipts, failed payments, and revenue summary
5. Docs, local setup, and a devnet demo script that proves the full flow

## Success Criteria

- One paid tool can be invoked end to end on devnet
- Duplicate or replayed payment proofs are rejected
- Existing MCP server integration takes under 30 minutes using the example
- Demo includes challenge, pay, verify, execute, and receipt lookup
- Two external founders can understand the value from the docs and demo alone

## Product Constraints

- Do not build subscriptions in MVP
- Do not build multi-chain support in MVP
- Do not build a marketplace in MVP
- Do not build custom onchain programs in MVP
- Do not optimize for enterprise auth before the payment flow is solid

## Competitive Design Rules

- Differentiate on wrapper UX, not payment rail claims
- Make pricing setup obvious in minutes, not configurable in every possible way
- Show payment status and receipts clearly enough that operators trust the system
- Keep challenge -> pay -> execute flow deterministic and easy to debug
- Prefer a narrow and polished devnet demo over a broad but fragile platform story

## Build Status

- milestones:
  - "Phase 1 gateway core completed"
  - "Phase 2 persistence and replay protection implemented"
- mvp_complete: false
- tests_passing: false
- devnet_deployed: false
- program_id: n/a
