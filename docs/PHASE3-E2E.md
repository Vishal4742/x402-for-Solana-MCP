# Phase 3: End-to-End MCP Demo

This runbook takes a buyer from a `402` all the way through a settled, executed paid tool call and a
receipt. It has two paths: a mock path that needs no chain, and a devnet path that sends a real SPL
transfer.

## What Phase 3 adds

- The gateway forwards a settled paid call to an upstream MCP server set by `UPSTREAM_MCP_URL`. It
  replays the request that was actually paid for, not whatever the retry carries.
- An example seller server (`apps/example-server`): free `ping`, paid `premium.search` and
  `premium.codegen`.
- An example buyer client (`apps/example-client`): pays the challenge, verifies, retries, then reads
  the receipt.
- `GET /v1/receipts/:requestId` returns the receipt, including a `responseHash` over the delivered
  result.

## Prerequisites

- Go 1.23+, Node 20+, pnpm.
- Postgres and Redis. With Docker: `docker compose up -d`. Without Docker, install and run them
  natively and set `POSTGRES_URL` / `REDIS_URL`.
- `pnpm install` and `pnpm -r build` at the repo root.

## Mock path (no chain)

Four terminals, or background the first three.

1. Infra: `docker compose up -d`.
2. Verifier: `SOLANA_VERIFY_MODE=mock make verifier`.
3. Example server: `GATEWAY_KEY=x402_sk_dev_local pnpm example:server`.
4. Gateway: `UPSTREAM_MCP_URL=http://localhost:9090 make gateway`.
5. Demo: `PAY_MODE=mock pnpm example:demo`.

Expected output, in order:

- `ping` returns immediately (free tool, forwarded to the upstream).
- `premium.search` returns `402`, the client "pays", verifies, retries, and gets the tool result
  from the example server.
- The receipt prints with a `responseHash`.
- The replay step reuses the signature against a fresh `premium.search` challenge and is rejected
  with `transaction already used`.

## Devnet path (real SPL transfer)

The verifier inspects the real transaction, so the buyer needs devnet SOL and an SPL token to spend.

1. Create fixtures: `pnpm example:setup`. This generates buyer and seller keypairs, funds the buyer
   from the faucet, creates a 6-decimal mint, and mints 100 tokens to the buyer. It writes
   `apps/example-client/devnet-state.json` and prints the `SELLER_WALLET`, `SOLANA_USDC_MINT`, and
   `SOLANA_VERIFY_MODE=rpc` lines to use.

   The devnet faucet rate-limits hard. If it refuses, the keypairs are still saved, so the buyer
   address stays stable — fund it manually at https://faucet.solana.com and rerun. `AIRDROP_SOL`
   controls the request size (default 1).

2. Start the verifier in rpc mode:
   `SOLANA_VERIFY_MODE=rpc VERIFIER_NETWORK=devnet make verifier`.
3. Start the example server: `GATEWAY_KEY=x402_sk_dev_local pnpm example:server`.
4. Start the gateway with the values setup printed:
   `SELLER_WALLET=<seller> SOLANA_USDC_MINT=<mint> UPSTREAM_MCP_URL=http://localhost:9090 make gateway`.
5. Run the demo: `pnpm example:demo` (devnet is the default `PAY_MODE`).

The buyer builds an SPL transfer of exactly the quoted amount to the seller's token account and
attaches the challenge reference so the transfer is bound to this challenge. After confirmation it
submits the signature, the verifier checks it on chain, and the gateway forwards to the example
server.

## Acceptance checklist

1. Buyer calls a paid tool and receives `402`.
2. Buyer pays on devnet.
3. Buyer verifies the payment.
4. Buyer retries with the request id and the tool executes.
5. The receipt can be retrieved and its `responseHash` matches the delivered result.
6. Replaying the signature against a new challenge is rejected.

## Failure modes worth knowing

- `underpayment` — the transfer moved less than the quote. The signature is not consumed; pay the
  full amount and verify again.
- `reference_mismatch` — the transfer did not carry the challenge reference. The buyer must attach
  it; an old or unrelated transfer cannot settle this challenge.
- `replay_detected` — the signature was already spent by another request.
- A `502` from verify is transient (RPC hiccup). The request stays open; retry the same signature.
