# Example Seller Server

A minimal MCP-style tool server the x402 gateway forwards settled calls to. Plain `node:http`, no framework, three tools:

| Tool | Gateway price (seed) | Behavior |
| --- | --- | --- |
| `ping` | free | Echoes the input back with a timestamp |
| `premium.search` | 1 USDC (1,000,000 atomic) | Returns three fake search results |
| `premium.codegen` | 2.5 USDC (2,500,000 atomic) | Returns a small fake code snippet |

Pricing lives in the gateway, not here — this server just executes whatever the gateway forwards after payment settles.

## API

- `POST /mcp` with body `{"tool": "<name>", "input": { ... }}` → tool result JSON. Unknown tool → `404 {"error":"unknown_tool"}`.
- `GET /healthz` → `{"status":"ok","service":"example-server"}`.

If the env var `GATEWAY_KEY` is set, every `POST /mcp` must carry a matching `X-Gateway-Key` header, otherwise the server answers `401 {"error":"unauthorized"}`. The gateway sends its server API key (`x402_sk_dev_local` by default) in that header, plus `X-Gateway-Server-Id` and `X-Gateway-Request-Id`.

## Run

```bash
pnpm --filter @x402/example-server dev        # tsx, port 9090
# or from the repo root:
pnpm example:server
```

Env:

- `EXAMPLE_SERVER_PORT` — listen port, default `9090`
- `GATEWAY_KEY` — optional shared secret; when set, must match the gateway's server API key

## Hooking it up to the gateway

Start the gateway with:

```bash
UPSTREAM_MCP_URL=http://localhost:9090 go run ./services/gateway/cmd/gateway
```

The gateway then forwards free tools immediately and paid tools once their payment request reaches `verified`, POSTing the originally paid-for `{"tool","input"}` body to `http://localhost:9090/mcp`.
