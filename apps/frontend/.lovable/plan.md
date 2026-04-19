## x402 for Solana MCP — Frontend Build Plan

**Stack note:** This Lovable project runs on **TanStack Start + React + TypeScript + Tailwind v4** (functionally equivalent to Next.js for a frontend-only build with file-based routing and SSR). I'll build there instead of swapping frameworks. All UI stays modular and ready to wire to the Go/Chi endpoints later.

---

### Design system (shared across landing + dashboard)

Adapted from the reference, **not copied**:

- **Palette:** pure black `#0A0A0A` base, off-white `#FAFAFA` text, muted `#7A7A7A`, hairline border `rgba(255,255,255,0.14)`. Status accents only: success `#7CE7B2`, pending `#E8C97A`, warning `#E8A87A`, fail `#FF6B6B`.
- **Type:** Helvetica Neue / Inter fallback. Oversized display weight 400, tight `-0.03em` tracking. Uppercase micro-labels at 0.7rem with the `— ` suffix.
- **Geometry:** 12-col editorial grid, 1400px max, hairline dividers between every section, pill buttons (`border-radius: 999px`, 1px border).
- **Motion:** Hero gets a custom **Bayer-dithered animated canvas** (ported from the reference) but reshaped as a **flowing payment-stream waveform** — visual signature, not decoration.
- **Mock data layer:** typed `Server`, `ToolPricing`, `PaymentRequest`, `Receipt` in `src/lib/mock/` with realistic values (USDC amounts, base58 sigs, devnet addresses, timeline events).

---

### Surface 1 — Marketing landing page (`/`)

Cinematic, single scroll, hairline-divided sections:

1. **Fixed header** — `x402/sol` wordmark left; pill nav: Docs · Dashboard · Connect Wallet.
2. **Hero** — full-viewport, dithered canvas background.
   - Micro-label: `PROTOCOL · SOLANA DEVNET —`
   - Display headline (clamp 4–11rem): **"CHARGE PER TOOL CALL."**
   - Sub: "Wallet-native USDC billing for MCP servers. 402 in. Receipt out."
   - Primary pill `LAUNCH DASHBOARD ⊕`, secondary pill `READ THE SPEC ⊕`.
3. **Stat strip** — three editorial stat blocks: avg settlement time, USDC settled (mock), active MCP servers — with `— ` labels.
4. **Flow section ("THE LIFECYCLE —")** — six numbered steps as a horizontal hairline-bordered rail: `01 Request → 02 402 Challenge → 03 Pay USDC → 04 Verify → 05 Execute → 06 Receipt`. Each cell shows a tiny code/JSON snippet of the actual payload at that step.
5. **Big-copy section** — asymmetric grid (col 5/12), editorial paragraph with hover-highlighted phrases: _"Wrap an existing MCP server in minutes. Price tools individually. Settle on Solana. Inspect every receipt."_
6. **Feature grid (2×3 hairline cells)** — Tool-level pricing · USDC settlement on Solana · Drop-in MCP wrapper · Signed receipts · Failed-payment visibility · Devnet-first developer flow. Each cell: micro-label + 2-line description + small terminal-style snippet.
7. **Architecture section** — full-width diagram built in CSS/SVG: `Client → Gateway → Verifier → Solana RPC → MCP Server → Receipt Store`, hairline boxes with directional ticks.
8. **Operator use cases** — three editorial cards: Paid AI tool APIs · Metered data sources · Premium agent skills.
9. **Code block** — a tabbed snippet (cURL / TS SDK / Go middleware) showing the 402 challenge response and verify call.
10. **Final CTA** — oversized "SHIP A PAID MCP TODAY." with two pills.
11. **Minimal footer** — three columns (Protocol / Developers / Company), copyright row, devnet badge.

---

### Surface 2 — Operator dashboard (`/dashboard/*`)

Same visual DNA but utility-dense. Persistent **left sidebar nav** + top bar with server selector pill + wallet pill.

**Routes (TanStack file-based):**

- `/dashboard` — **Overview**: 4 KPI cards (Total Revenue USDC, Paid Requests, Failed Verifications, Avg Settlement ms), Paid vs Free tools split bar, Recent Receipts table (last 8), Latest Activity feed.
- `/dashboard/tools` — **Tools & Pricing**: table of tools per server with inline price editor (USDC), enable/disable toggle, "paid/free" badge, drawer for tool detail.
- `/dashboard/receipts` — **Receipts** table: requestId, tool, amount, payer (truncated base58), tx sig (links to devnet explorer), status badge, timestamp. Filters: server, tool, status, date range. Search by requestId/sig.
- `/dashboard/failed` — **Failed Payments**: same table shape, failure-reason column (Insufficient funds / Signature invalid / Timeout / Tool error), retry action.
- `/dashboard/settings` — **Server Settings**: server metadata, gateway endpoint, payout wallet, webhook URL, API keys with reveal/copy.
- `/dashboard/requests/$requestId` — **Request Detail / Timeline**: vertical timeline with five states (`challenged → paid → verified → executed`, or `failed` branch), each step shows timestamp + payload snippet + signed hash. Side panel: payer, amount, tool, server, raw JSON viewer.

**Reusable components** (`src/components/`):

- `StatusBadge` (challenged/paid/verified/executed/failed — each with its restrained accent)
- `KpiCard`, `DataTable` (sortable + filterable), `FilterBar`, `Pill`, `MicroLabel`, `HairlineDivider`
- `Timeline` + `TimelineStep`
- `DetailDrawer` (slide-in from right)
- `EmptyState`, `ErrorState`, `TableSkeleton`, `JsonViewer`
- `CodeBlock` with tabs, `WalletPill`, `ServerSwitcher`

**States covered everywhere:** loading skeletons, empty (with illustration of an empty receipt), error (with retry), responsive collapse to single column + hamburger nav on mobile.

---

### Deliverables

- 1 landing route + 6 dashboard routes, all SSR-friendly with proper per-route `head()` metadata.
- ~20 reusable primitives in `src/components/`.
- Typed mock data + a thin `src/lib/api.ts` indirection so swapping to the real Go endpoints later is a single-file change.
- Consistent design tokens defined in `src/styles.css` via Tailwind v4 `@theme`.
