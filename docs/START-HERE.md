# Start Here

This project should be built in a strict order. Do not start with the dashboard, auth, or advanced payment features.

## Immediate setup

1. Install the missing local tools:
   - Go `1.23+`
   - Docker Desktop
   - `pnpm`
2. Copy `.env.example` to `.env`
3. Start infrastructure:

```bash
docker compose up -d
```

4. Verify local services are available:
   - Postgres on `localhost:5432`
   - Redis on `localhost:6379`
5. Run the gateway and verifier:

```bash
cd services/gateway && go run ./cmd/gateway
cd services/verifier && go run ./cmd/verifier
```

6. Install workspace packages:

```bash
pnpm install
pnpm --filter @x402/sdk build
```

## Frontend app

The frontend is a separate cloned app at:

```text
apps/frontend
```

Run it with:

```bash
cd apps/frontend
npm install
npm run dev
```

More details:

- `docs/FRONTEND.md`

## Work order

1. `Phase 0`: environment and repo bootstrap
2. `Phase 1`: gateway challenge flow
3. `Phase 2`: persistence, replay protection, and Solana verification
4. `Phase 3`: example seller server and buyer client
5. `Phase 4`: operator dashboard
6. `Phase 5`: hardening, docs, and demo

Do not move to the next phase until the current phase has a working acceptance demo.

## Current priorities

### Priority 1

Run the Phase 2 persistence demo with Postgres, Redis, verifier, and gateway.

### Priority 2

Replace mock verification with real Solana devnet USDC transaction inspection.

### Priority 3

Start Phase 3 with an example MCP seller server and buyer client.

## What to ignore for now

- subscriptions
- multi-chain support
- marketplace discovery
- enterprise auth
- advanced pricing models
- custom onchain programs
