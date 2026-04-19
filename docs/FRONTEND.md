# Frontend Setup

The frontend app is kept separate from the Go services and lives at:

```text
apps/frontend
```

This is a cloned copy of:

```text
https://github.com/Vishal4742/mcp-pay-vault.git
```

## Stack

- TanStack Start
- React + TypeScript
- Vite
- Tailwind
- Radix UI primitives
- Cloudflare Wrangler build target

## Local commands

Install dependencies:

```bash
cd apps/frontend
npm install
```

Create the frontend env file:

```bash
cd apps/frontend
cp .env.example .env
```

Run the dev server:

```bash
cd apps/frontend
npm run dev
```

## Environment variables

- `VITE_API_BASE_URL`
  - base URL for the Go gateway, for example `http://localhost:8080`
- `VITE_USE_MOCK_FALLBACK`
  - keep as `true` while dashboard endpoints are still incomplete
  - set to `false` when you want missing backend routes to fail loudly

Build the frontend:

```bash
cd apps/frontend
npm run build
```

Lint the frontend:

```bash
cd apps/frontend
npm run lint
```

## Current status

- Repo cloned locally
- Dependencies installed successfully
- Production build completes successfully
- Frontend API layer prefers the live backend and falls back to mocks for incomplete dashboard routes

## Build note

The build currently emits a Wrangler log-path warning about:

```text
/home/vishal/.config/.wrangler
```

This did not block the build. It is a local environment logging issue, not a frontend compile failure.

## Intended role in this project

This repo is the frontend-only app for:

- landing page
- operator dashboard

It should later connect to the Go backend services for:

- gateway
- verifier
- receipts
- pricing
