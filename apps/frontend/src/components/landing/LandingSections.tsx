import { Link } from "@tanstack/react-router";
import { DitherCanvas } from "@/components/DitherCanvas";
import { Pill, MicroLabel, HairlineDivider } from "@/components/Editorial";
import { CodeBlock } from "@/components/CodeBlock";
import { StatusBadge } from "@/components/StatusBadge";

export function LandingHeader() {
  return (
    <header className="fixed top-0 inset-x-0 z-40 hairline-b bg-background/80 backdrop-blur-md">
      <div className="container-editorial flex items-center justify-between h-14">
        <Link to="/" className="flex items-center gap-2 font-mono text-sm tracking-tight">
          <span className="inline-block h-2 w-2 bg-foreground rounded-full" />
          x402<span className="text-muted-foreground">/sol</span>
        </Link>
        <nav className="flex items-center gap-2">
          <a href="#lifecycle" className="pill text-[0.7rem] hidden sm:inline-flex">
            Spec
          </a>
          <a href="#architecture" className="pill text-[0.7rem] hidden md:inline-flex">
            Architecture
          </a>
          <Link to="/dashboard" className="pill text-[0.7rem]">
            Dashboard
          </Link>
          <button className="pill pill-solid text-[0.7rem]">Connect Wallet ⊕</button>
        </nav>
      </div>
    </header>
  );
}

export function Hero() {
  return (
    <section className="relative min-h-[100svh] hairline-b overflow-hidden">
      <DitherCanvas className="absolute inset-0 h-full w-full" />
      <div className="absolute inset-0 scanlines pointer-events-none" />
      <div className="absolute inset-0 bg-gradient-to-b from-background/30 via-transparent to-background" />

      <div className="relative container-editorial pt-32 pb-16 min-h-[100svh] flex flex-col">
        <MicroLabel number="001">Protocol · Solana Devnet</MicroLabel>

        <div className="mt-auto">
          <h1 className="display-xl">
            CHARGE PER
            <br />
            <span className="text-foreground/60">TOOL CALL.</span>
          </h1>
          <div className="mt-10 grid grid-cols-1 md:grid-cols-12 gap-8 items-end">
            <p className="md:col-span-5 text-lg text-foreground/80 leading-relaxed">
              Wallet-native USDC billing for MCP servers.
              <span className="text-muted-foreground"> 402 in. Receipt out.</span>
              <br />
              Wrap an existing server, price each tool, settle on Solana.
            </p>
            <div className="md:col-span-4 md:col-start-9 flex flex-wrap gap-3 md:justify-end">
              <Link to="/dashboard" className="pill pill-solid">
                Launch Dashboard ⊕
              </Link>
              <a href="#lifecycle" className="pill">
                Read the Spec ⊕
              </a>
            </div>
          </div>
        </div>

        <div className="mt-16 hairline-t pt-4 grid grid-cols-3 gap-4 font-mono text-[0.7rem] text-muted-foreground">
          <div>
            STATUS — <span className="text-status-success">● devnet live</span>
          </div>
          <div className="hidden sm:block text-center">SCHEME — x402.sol.usdc.v1</div>
          <div className="text-right">BUILD — 2025.04.19</div>
        </div>
      </div>
    </section>
  );
}

export function StatStrip() {
  const stats = [
    { label: "Avg settlement", value: "482", unit: "ms" },
    { label: "USDC settled · 7d", value: "12,847", unit: "USDC" },
    { label: "Active MCP servers", value: "27", unit: "nodes" },
  ];
  return (
    <section className="hairline-b">
      <div className="container-editorial grid grid-cols-1 md:grid-cols-3">
        {stats.map((s, i) => (
          <div
            key={s.label}
            className={`p-8 md:p-10 ${i < stats.length - 1 ? "md:hairline-r" : ""} hairline-b md:border-b-0`}
          >
            <MicroLabel>{s.label}</MicroLabel>
            <div className="mt-6 flex items-baseline gap-3">
              <span className="text-6xl md:text-7xl font-light tracking-tight">{s.value}</span>
              <span className="font-mono text-sm text-muted-foreground">{s.unit}</span>
            </div>
          </div>
        ))}
      </div>
    </section>
  );
}

const lifecycleSteps = [
  { n: "01", title: "Request", code: `POST /mcp/srv_01HX3K\n{ "tool": "web.search" }` },
  {
    n: "02",
    title: "402 Challenge",
    code: `HTTP 402 Payment Required\nx-pay-scheme: x402.sol.usdc.v1\namount: 0.005 USDC`,
  },
  {
    n: "03",
    title: "Pay USDC",
    code: `signTransaction({\n  to: payoutWallet,\n  lamports: 5_000\n})`,
  },
  { n: "04", title: "Verify", code: `POST /v1/verify\n{ "sig": "5KJp7z…", "req": "req_8KpL" }` },
  { n: "05", title: "Execute", code: `→ MCP tool dispatch\nresult: { ok: true }` },
  {
    n: "06",
    title: "Receipt",
    code: `{ "id": "rcpt_8KpL",\n  "amount": "0.005 USDC",\n  "block": 312_887_142 }`,
  },
];

export function Lifecycle() {
  return (
    <section id="lifecycle" className="hairline-b">
      <div className="container-editorial py-20">
        <div className="grid grid-cols-12 gap-8 mb-12">
          <div className="col-span-12 md:col-span-4">
            <MicroLabel number="002">The Lifecycle</MicroLabel>
            <h2 className="display-md mt-4">From request to receipt in six steps.</h2>
          </div>
          <p className="col-span-12 md:col-span-6 md:col-start-7 text-foreground/70 self-end">
            Every paid tool call follows the same path. The gateway handles the 402, the verifier
            checks the on-chain transfer, the MCP server only runs after settlement.
          </p>
        </div>

        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-6 gap-px bg-hairline">
          {lifecycleSteps.map((s) => (
            <div key={s.n} className="bg-background p-5 min-h-[200px] flex flex-col">
              <div className="flex items-center justify-between">
                <span className="font-mono text-xs text-muted-foreground">{s.n}</span>
                <span className="h-1 w-1 rounded-full bg-foreground" />
              </div>
              <div className="mt-4 text-base tracking-tight">{s.title}</div>
              <pre className="mt-auto pt-4 text-[0.65rem] font-mono text-muted-foreground/80 leading-relaxed whitespace-pre-wrap">
                {s.code}
              </pre>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}

export function BigCopy() {
  return (
    <section className="hairline-b">
      <div className="container-editorial py-24 grid grid-cols-12 gap-8">
        <div className="col-span-12 md:col-span-2">
          <MicroLabel number="003">Thesis</MicroLabel>
        </div>
        <p className="col-span-12 md:col-span-8 text-2xl md:text-4xl tracking-tight leading-[1.15] text-foreground/85">
          Wrap an existing MCP server in minutes.{" "}
          <span className="text-foreground">Price tools individually.</span> Settle on Solana.{" "}
          <span className="text-foreground">Inspect every receipt</span> — and every failed payment
          — without writing a billing system.
        </p>
      </div>
    </section>
  );
}

const features = [
  {
    label: "Pricing",
    title: "Tool-level pricing",
    desc: "Set a USDC price per tool. Free tools stay free.",
    code: `tools.set("web.search", "0.005")`,
  },
  {
    label: "Settlement",
    title: "USDC on Solana",
    desc: "Sub-second confirmation on devnet. SPL Token transfers.",
    code: `network: "devnet"\nmint:    EPjFW…UsdcZ`,
  },
  {
    label: "Integration",
    title: "Drop-in MCP wrapper",
    desc: "One middleware in front of your existing MCP server.",
    code: `app.Use(x402.Middleware(cfg))`,
  },
  {
    label: "Receipts",
    title: "Signed receipts",
    desc: "Every paid call gets a verifiable receipt with tx signature.",
    code: `GET /v1/dashboard/receipts`,
  },
  {
    label: "Failures",
    title: "Failed payment visibility",
    desc: "Insufficient funds, bad sigs, timeouts — all surfaced.",
    code: `reason: "signature_invalid"`,
  },
  {
    label: "Devflow",
    title: "Devnet first",
    desc: "Build, test, and inspect locally before going live.",
    code: `RPC = api.devnet.solana.com`,
  },
];

export function Features() {
  return (
    <section className="hairline-b">
      <div className="container-editorial py-20">
        <div className="grid grid-cols-12 gap-8 mb-12">
          <div className="col-span-12 md:col-span-4">
            <MicroLabel number="004">Capabilities</MicroLabel>
            <h2 className="display-md mt-4">Built for paid MCP infrastructure.</h2>
          </div>
        </div>
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-px bg-hairline">
          {features.map((f) => (
            <div key={f.title} className="bg-background p-8 min-h-[260px] flex flex-col">
              <div className="micro-label">— {f.label}</div>
              <h3 className="mt-4 text-2xl tracking-tight">{f.title}</h3>
              <p className="mt-2 text-sm text-muted-foreground">{f.desc}</p>
              <pre className="mt-auto pt-6 font-mono text-[0.7rem] text-foreground/60 hairline-t pt-4">
                {f.code}
              </pre>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}

export function Architecture() {
  const nodes = [
    { label: "Client", sub: "MCP caller" },
    { label: "Gateway", sub: "402 issuer" },
    { label: "Verifier", sub: "tx checker" },
    { label: "Solana RPC", sub: "devnet" },
    { label: "MCP Server", sub: "your tools" },
    { label: "Receipts", sub: "store + log" },
  ];
  return (
    <section id="architecture" className="hairline-b">
      <div className="container-editorial py-20">
        <div className="grid grid-cols-12 gap-8 mb-12">
          <div className="col-span-12 md:col-span-5">
            <MicroLabel number="005">Architecture</MicroLabel>
            <h2 className="display-md mt-4">
              Six components.
              <br />
              One signed lifecycle.
            </h2>
          </div>
          <p className="col-span-12 md:col-span-5 md:col-start-8 text-foreground/70 self-end">
            The gateway never executes a tool until the verifier confirms the transfer on Solana.
            Receipts are signed and queryable.
          </p>
        </div>

        <div className="hairline p-8 md:p-12 bg-card/30">
          <div className="grid grid-cols-2 md:grid-cols-6 gap-px bg-hairline">
            {nodes.map((n, i) => (
              <div key={n.label} className="bg-background p-6 text-center relative">
                <div className="font-mono text-[0.65rem] text-muted-foreground">N0{i + 1}</div>
                <div className="mt-3 text-lg tracking-tight">{n.label}</div>
                <div className="mt-1 text-xs text-muted-foreground">{n.sub}</div>
                {i < nodes.length - 1 && (
                  <span className="hidden md:block absolute -right-3 top-1/2 -translate-y-1/2 text-muted-foreground font-mono">
                    →
                  </span>
                )}
              </div>
            ))}
          </div>
          <div className="mt-8 grid grid-cols-1 md:grid-cols-3 gap-4 text-xs text-muted-foreground">
            <div className="hairline p-3 font-mono">→ HTTP 402 + challenge</div>
            <div className="hairline p-3 font-mono">→ SPL transfer + signature</div>
            <div className="hairline p-3 font-mono">→ Verified · executed · stored</div>
          </div>
        </div>
      </div>
    </section>
  );
}

const useCases = [
  {
    tag: "01 / API",
    title: "Paid AI tool APIs",
    desc: "Charge agents per call. No keys, no Stripe, no monthly plan.",
  },
  {
    tag: "02 / Data",
    title: "Metered data sources",
    desc: "Sell access to embeddings, scrapers, indices — by the request.",
  },
  {
    tag: "03 / Skills",
    title: "Premium agent skills",
    desc: "Gate expensive tools (image gen, code exec) behind USDC.",
  },
];

export function UseCases() {
  return (
    <section className="hairline-b">
      <div className="container-editorial py-20">
        <MicroLabel number="006">Operator surfaces</MicroLabel>
        <h2 className="display-md mt-4 max-w-3xl">Who ships on x402/sol.</h2>
        <div className="mt-10 grid grid-cols-1 md:grid-cols-3 gap-px bg-hairline">
          {useCases.map((u) => (
            <div key={u.title} className="bg-background p-8">
              <div className="font-mono text-xs text-muted-foreground">{u.tag}</div>
              <h3 className="mt-6 text-2xl tracking-tight">{u.title}</h3>
              <p className="mt-3 text-sm text-muted-foreground">{u.desc}</p>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}

export function CodeShowcase() {
  return (
    <section className="hairline-b">
      <div className="container-editorial py-20 grid grid-cols-12 gap-8">
        <div className="col-span-12 md:col-span-4">
          <MicroLabel number="007">Wire format</MicroLabel>
          <h2 className="display-md mt-4">Plain HTTP. Signed transfers.</h2>
          <p className="mt-6 text-foreground/70">
            The 402 challenge is a normal HTTP response. The verify call takes a signature and a
            request id. That's the entire surface.
          </p>
          <div className="mt-6 flex flex-wrap gap-2">
            <StatusBadge status="challenged" />
            <StatusBadge status="paid" />
            <StatusBadge status="verified" />
            <StatusBadge status="executed" />
            <StatusBadge status="failed" />
          </div>
        </div>
        <div className="col-span-12 md:col-span-8 grid grid-cols-1 lg:grid-cols-2 gap-4">
          <CodeBlock
            filename="402 challenge"
            language="http"
            code={`HTTP/1.1 402 Payment Required
x-pay-scheme: x402.sol.usdc.v1
x-pay-to:     8KpL3xN…pQrSkX
x-pay-amount: 0.005
x-pay-mint:   EPjFW…UsdcZ
x-req-id:     req_8KpL3xN2

{ "challenge": "0x9f3a…" }`}
          />
          <CodeBlock
            filename="verify call"
            language="bash"
            code={`curl -X POST $GATEWAY/v1/verify \\
  -H "content-type: application/json" \\
  -d '{
    "requestId": "req_8KpL3xN2",
    "signature": "5KJp7z3X9mNqR8…"
  }'

→ 200 { "status": "verified" }`}
          />
        </div>
      </div>
    </section>
  );
}

export function FinalCta() {
  return (
    <section className="hairline-b relative overflow-hidden">
      <div className="container-editorial py-32 text-center">
        <MicroLabel className="justify-center">— Ready</MicroLabel>
        <h2 className="display-xl mt-8">
          SHIP A PAID
          <br />
          MCP TODAY.
        </h2>
        <div className="mt-12 flex flex-wrap gap-3 justify-center">
          <Link to="/dashboard" className="pill pill-solid">
            Open Dashboard ⊕
          </Link>
          <a href="#lifecycle" className="pill">
            Read the Spec ⊕
          </a>
        </div>
      </div>
    </section>
  );
}

export function LandingFooter() {
  return (
    <footer>
      <div className="container-editorial py-16 grid grid-cols-2 md:grid-cols-4 gap-8">
        <div className="col-span-2 md:col-span-1">
          <div className="flex items-center gap-2 font-mono text-sm">
            <span className="inline-block h-2 w-2 bg-foreground rounded-full" />
            x402<span className="text-muted-foreground">/sol</span>
          </div>
          <p className="mt-4 text-xs text-muted-foreground max-w-[18ch]">
            Wallet-native payment gateway for MCP servers on Solana.
          </p>
        </div>
        {[
          { h: "Protocol", l: ["x402 spec", "Schemes", "Verifier"] },
          { h: "Developers", l: ["Quickstart", "Go middleware", "TS SDK"] },
          { h: "Company", l: ["About", "Security", "Contact"] },
        ].map((c) => (
          <div key={c.h}>
            <div className="micro-label">— {c.h}</div>
            <ul className="mt-4 space-y-2 text-sm">
              {c.l.map((it) => (
                <li key={it}>
                  <a
                    href="#"
                    className="hover:text-foreground text-foreground/70 transition-colors"
                  >
                    {it}
                  </a>
                </li>
              ))}
            </ul>
          </div>
        ))}
      </div>
      <HairlineDivider />
      <div className="container-editorial py-6 flex flex-col sm:flex-row items-center justify-between gap-2 text-xs text-muted-foreground font-mono">
        <span>© 2025 x402/sol — built for operators</span>
        <span className="flex items-center gap-2">
          <span className="h-1.5 w-1.5 rounded-full bg-status-success" />
          devnet · build 2025.04.19
        </span>
      </div>
    </footer>
  );
}
