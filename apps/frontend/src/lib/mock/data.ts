import type { DashboardSummary, PaymentRequest, Receipt, Server, ToolPricing } from "./types";

// Realistic-looking devnet base58 addresses (mock — not real wallets).
const wallets = [
  "8KpL3xN2qR9vF4mT7sH1jY6cB3eW5zA2dG8nM9pQrSkX",
  "Hm7vP3xN2qR9vF4mT7sH1jY6cB3eW5zA2dG8nM9pQrSk",
  "3FxN2qR9vF4mT7sH1jY6cB3eW5zA2dG8nM9pQrSkXKpL",
  "2vF4mT7sH1jY6cB3eW5zA2dG8nM9pQrSkXKpL3xN2qR9",
  "9vF4mT7sH1jY6cB3eW5zA2dG8nM9pQrSkXKpL3xN2qRa",
  "Fb1jY6cB3eW5zA2dG8nM9pQrSkXKpL3xN2qR9vF4mT7s",
];

const sigs = [
  "5KJp7z3X9mNqR8vF4mT7sH1jY6cB3eW5zA2dG8nM9pQrSkXKpL3xN2qR9vF4mT7sH1jY6cB3eW5z",
  "3xN2qR9vF4mT7sH1jY6cB3eW5zA2dG8nM9pQrSkXKpL3xN2qR9vF4mT7sH1jY6cB3eW5zA2dG8n",
  "9pQrSkXKpL3xN2qR9vF4mT7sH1jY6cB3eW5zA2dG8nM9pQrSkXKpL3xN2qR9vF4mT7sH1jY6cB3",
  "Fb1jY6cB3eW5zA2dG8nM9pQrSkXKpL3xN2qR9vF4mT7sFb1jY6cB3eW5zA2dG8nM9pQrSkXKpL3",
  "2dG8nM9pQrSkXKpL3xN2qR9vF4mT7sH1jY6cB3eW5zA22dG8nM9pQrSkXKpL3xN2qR9vF4mT7sH",
];

export const servers: Server[] = [
  {
    id: "srv_01HX3K",
    name: "agent-tools-prod",
    endpoint: "https://gateway.x402sol.dev/mcp/srv_01HX3K",
    payoutWallet: wallets[0],
    webhookUrl: "https://api.acme.io/webhooks/x402",
    network: "devnet",
    createdAt: "2025-03-12T10:24:00Z",
    apiKey: "x402_sk_live_8f7d3b2c9a1e4f6h",
  },
  {
    id: "srv_02JK9P",
    name: "research-mcp",
    endpoint: "https://gateway.x402sol.dev/mcp/srv_02JK9P",
    payoutWallet: wallets[1],
    webhookUrl: "https://research.lab/hooks/x402",
    network: "devnet",
    createdAt: "2025-04-01T08:11:00Z",
    apiKey: "x402_sk_live_2a9c8f1b3e7d4h5j",
  },
];

export const tools: ToolPricing[] = [
  {
    id: "tool_01",
    serverId: "srv_01HX3K",
    toolName: "web.search",
    description: "Live web search with citations.",
    priceUsdc: 0.005,
    enabled: true,
    callsLast24h: 1247,
  },
  {
    id: "tool_02",
    serverId: "srv_01HX3K",
    toolName: "code.execute",
    description: "Sandboxed Python execution.",
    priceUsdc: 0.02,
    enabled: true,
    callsLast24h: 384,
  },
  {
    id: "tool_03",
    serverId: "srv_01HX3K",
    toolName: "image.generate",
    description: "Diffusion-based image rendering.",
    priceUsdc: 0.05,
    enabled: true,
    callsLast24h: 192,
  },
  {
    id: "tool_04",
    serverId: "srv_01HX3K",
    toolName: "ping",
    description: "Healthcheck endpoint.",
    priceUsdc: 0,
    enabled: true,
    callsLast24h: 8421,
  },
  {
    id: "tool_05",
    serverId: "srv_01HX3K",
    toolName: "vector.query",
    description: "Embedding similarity search.",
    priceUsdc: 0.01,
    enabled: true,
    callsLast24h: 612,
  },
  {
    id: "tool_06",
    serverId: "srv_01HX3K",
    toolName: "file.parse",
    description: "PDF / DOCX text extraction.",
    priceUsdc: 0.015,
    enabled: false,
    callsLast24h: 0,
  },
  {
    id: "tool_07",
    serverId: "srv_02JK9P",
    toolName: "scholar.lookup",
    description: "Academic paper retrieval.",
    priceUsdc: 0.025,
    enabled: true,
    callsLast24h: 89,
  },
];

const now = Date.now();
const minutesAgo = (m: number) => new Date(now - m * 60_000).toISOString();

function buildTimeline(
  status: PaymentRequest["status"],
  baseMinutes: number,
  failureReason?: PaymentRequest["failureReason"],
): PaymentRequest["timeline"] {
  const events: PaymentRequest["timeline"] = [
    {
      status: "challenged",
      timestamp: minutesAgo(baseMinutes),
      payload: { code: 402, scheme: "x402.sol.usdc.v1" },
      durationMs: 12,
    },
  ];
  if (status === "challenged") return events;

  events.push({
    status: "paid",
    timestamp: minutesAgo(baseMinutes - 0.05),
    payload: { tx: sigs[0].slice(0, 24) + "…", amount: "0.005 USDC" },
    signatureHash: sigs[0],
    durationMs: 480,
  });
  if (status === "paid") return events;

  if (status === "failed" && failureReason) {
    events.push({
      status: "failed",
      timestamp: minutesAgo(baseMinutes - 0.1),
      payload: { reason: failureReason, code: 402 },
      durationMs: 220,
    });
    return events;
  }

  events.push({
    status: "verified",
    timestamp: minutesAgo(baseMinutes - 0.1),
    payload: { rpc: "api.devnet.solana.com", confirmations: 1 },
    durationMs: 220,
  });
  if (status === "verified") return events;

  events.push({
    status: "executed",
    timestamp: minutesAgo(baseMinutes - 0.18),
    payload: { result: "ok", bytes: 4128 },
    durationMs: 312,
  });
  return events;
}

export const requests: PaymentRequest[] = [
  {
    id: "req_8KpL3xN2",
    serverId: "srv_01HX3K",
    toolName: "web.search",
    payerWallet: wallets[2],
    amountUsdc: 0.005,
    status: "executed",
    txSignature: sigs[0],
    createdAt: minutesAgo(2),
    settledAt: minutesAgo(1.7),
    timeline: buildTimeline("executed", 2),
    rawRequest: { tool: "web.search", args: { q: "solana mcp" } },
    rawResponse: { results: 8, latencyMs: 312 },
  },
  {
    id: "req_3FxN2qR9",
    serverId: "srv_01HX3K",
    toolName: "code.execute",
    payerWallet: wallets[3],
    amountUsdc: 0.02,
    status: "executed",
    txSignature: sigs[1],
    createdAt: minutesAgo(7),
    settledAt: minutesAgo(6.7),
    timeline: buildTimeline("executed", 7),
    rawRequest: { tool: "code.execute", args: { lang: "python" } },
    rawResponse: { stdout: "...", exit: 0 },
  },
  {
    id: "req_Hm7vP3xN",
    serverId: "srv_01HX3K",
    toolName: "image.generate",
    payerWallet: wallets[4],
    amountUsdc: 0.05,
    status: "failed",
    failureReason: "insufficient_funds",
    createdAt: minutesAgo(11),
    timeline: buildTimeline("failed", 11, "insufficient_funds"),
    rawRequest: { tool: "image.generate", args: { prompt: "…" } },
  },
  {
    id: "req_Fb1jY6cB",
    serverId: "srv_01HX3K",
    toolName: "vector.query",
    payerWallet: wallets[5],
    amountUsdc: 0.01,
    status: "verified",
    txSignature: sigs[3],
    createdAt: minutesAgo(14),
    settledAt: minutesAgo(13.8),
    timeline: buildTimeline("verified", 14),
    rawRequest: { tool: "vector.query", args: { topK: 5 } },
  },
  {
    id: "req_2dG8nM9p",
    serverId: "srv_01HX3K",
    toolName: "code.execute",
    payerWallet: wallets[2],
    amountUsdc: 0.02,
    status: "failed",
    failureReason: "signature_invalid",
    createdAt: minutesAgo(22),
    timeline: buildTimeline("failed", 22, "signature_invalid"),
    rawRequest: { tool: "code.execute", args: { lang: "node" } },
  },
  {
    id: "req_9pQrSkXK",
    serverId: "srv_01HX3K",
    toolName: "web.search",
    payerWallet: wallets[3],
    amountUsdc: 0.005,
    status: "executed",
    txSignature: sigs[2],
    createdAt: minutesAgo(28),
    settledAt: minutesAgo(27.7),
    timeline: buildTimeline("executed", 28),
    rawRequest: { tool: "web.search", args: { q: "x402 spec" } },
  },
  {
    id: "req_5KJp7z3X",
    serverId: "srv_01HX3K",
    toolName: "vector.query",
    payerWallet: wallets[4],
    amountUsdc: 0.01,
    status: "paid",
    txSignature: sigs[4],
    createdAt: minutesAgo(34),
    timeline: buildTimeline("paid", 34),
    rawRequest: { tool: "vector.query", args: { topK: 10 } },
  },
  {
    id: "req_KpL3xN2q",
    serverId: "srv_02JK9P",
    toolName: "scholar.lookup",
    payerWallet: wallets[5],
    amountUsdc: 0.025,
    status: "executed",
    txSignature: sigs[0],
    createdAt: minutesAgo(41),
    settledAt: minutesAgo(40.7),
    timeline: buildTimeline("executed", 41),
    rawRequest: { tool: "scholar.lookup", args: { doi: "10.1145/…" } },
  },
  {
    id: "req_xN2qR9vF",
    serverId: "srv_01HX3K",
    toolName: "image.generate",
    payerWallet: wallets[2],
    amountUsdc: 0.05,
    status: "failed",
    failureReason: "timeout",
    createdAt: minutesAgo(58),
    timeline: buildTimeline("failed", 58, "timeout"),
    rawRequest: { tool: "image.generate", args: { prompt: "…" } },
  },
  {
    id: "req_4mT7sH1j",
    serverId: "srv_01HX3K",
    toolName: "web.search",
    payerWallet: wallets[3],
    amountUsdc: 0.005,
    status: "executed",
    txSignature: sigs[1],
    createdAt: minutesAgo(72),
    settledAt: minutesAgo(71.7),
    timeline: buildTimeline("executed", 72),
    rawRequest: { tool: "web.search", args: { q: "usdc devnet" } },
  },
  {
    id: "req_Y6cB3eW5",
    serverId: "srv_01HX3K",
    toolName: "code.execute",
    payerWallet: wallets[4],
    amountUsdc: 0.02,
    status: "challenged",
    createdAt: minutesAgo(89),
    timeline: buildTimeline("challenged", 89),
    rawRequest: { tool: "code.execute", args: { lang: "python" } },
  },
  {
    id: "req_zA2dG8nM",
    serverId: "srv_01HX3K",
    toolName: "vector.query",
    payerWallet: wallets[5],
    amountUsdc: 0.01,
    status: "failed",
    failureReason: "verification_failed",
    createdAt: minutesAgo(104),
    timeline: buildTimeline("failed", 104, "verification_failed"),
    rawRequest: { tool: "vector.query", args: { topK: 3 } },
  },
];

export const receipts: Receipt[] = requests
  .filter((r) => r.txSignature && (r.status === "executed" || r.status === "verified"))
  .map((r) => ({
    id: `rcpt_${r.id.slice(4)}`,
    requestId: r.id,
    serverId: r.serverId,
    toolName: r.toolName,
    amountUsdc: r.amountUsdc,
    payerWallet: r.payerWallet,
    txSignature: r.txSignature!,
    blockTime: r.settledAt ?? r.createdAt,
    createdAt: r.createdAt,
  }));

export const summary: DashboardSummary = {
  totalRevenueUsdc: receipts.reduce((s, r) => s + r.amountUsdc, 0),
  paidRequests: requests.filter((r) => r.status !== "challenged" && r.status !== "failed").length,
  failedVerifications: requests.filter((r) => r.status === "failed").length,
  avgSettlementMs: 482,
  paidToolCount: tools.filter((t) => t.priceUsdc > 0).length,
  freeToolCount: tools.filter((t) => t.priceUsdc === 0).length,
  revenueDelta7d: 23.8,
  paidDelta7d: 14.2,
};
