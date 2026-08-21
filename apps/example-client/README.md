# Example Buyer Client

Scriptable buyer demo built on `@x402/sdk`. It walks the full flow:

1. call a free tool (`ping`) — executes immediately
2. call a paid tool (`premium.search`) — receive `402 Payment Required`
3. pay the challenge (real SPL transfer on devnet, or a mock signature)
4. submit verification, retry with the settled request id
5. print the execution result and the receipt
6. negative test: replay the same tx signature against a fresh challenge and show the rejection

## Two scripts

- `src/setup-devnet.ts` — one-time devnet fixture: generates seller + buyer keypairs, airdrops SOL (with faucet retry), creates a 6-decimal SPL mint standing in for USDC, creates both ATAs, mints 100 tokens to the buyer, and writes `devnet-state.json` (never committed). Prints the env vars the gateway/verifier need.
- `src/demo.ts` — the demo itself. `PAY_MODE=devnet` (default) sends a real SPL transfer of exactly `challenge.amountAtomic`; `PAY_MODE=mock` fabricates a signature (works with the verifier's mock mode).

Env for `demo`:

| Var | Default | Meaning |
| --- | --- | --- |
| `GATEWAY_URL` | `http://localhost:8080` | gateway base URL |
| `SERVER_ID` | `srv_01HX3K` | seeded server id |
| `PAY_MODE` | `devnet` | `devnet` or `mock` |
| `SOLANA_RPC_URL` | value stored in devnet-state.json | devnet RPC |

## Mock run (no chain, fastest)

```bash
# terminal 1 — postgres + redis without docker
make devstack

# terminal 2 — verifier in mock mode
go run ./services/verifier/cmd/verifier

# terminal 3 — example seller server on :9090
pnpm example:server

# terminal 4 — gateway forwarding to the seller server
UPSTREAM_MCP_URL=http://localhost:9090 go run ./services/gateway/cmd/gateway

# terminal 5 — the demo
PAY_MODE=mock pnpm example:demo
```

## Devnet run (real payment, rpc verification)

```bash
# 1. create keypairs, mint, ATAs, tokens (retries the faucet; it can be dry — rerun later if so)
pnpm example:setup
# prints something like:
#   SELLER_WALLET=7f9z...
#   SOLANA_USDC_MINT=9dQ3...
#   SOLANA_VERIFY_MODE=rpc

# 2. devstack + seller server as above (terminals 1 and 3)

# 3. verifier in rpc mode
SOLANA_VERIFY_MODE=rpc SOLANA_RPC_URL=https://api.devnet.solana.com go run ./services/verifier/cmd/verifier

# 4. gateway configured with the printed values
SELLER_WALLET=<from setup> SOLANA_USDC_MINT=<from setup> \
UPSTREAM_MCP_URL=http://localhost:9090 go run ./services/gateway/cmd/gateway

# 5. run the demo (devnet is the default PAY_MODE)
pnpm example:demo
```

The demo exits 0 when every step passes, including the replay rejection. `devnet-state.json` is reused on re-runs (keypairs and mint survive), so `example:setup` only needs to run again if devnet resets or the buyer runs out of tokens.
