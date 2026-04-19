import { createFileRoute, Link } from "@tanstack/react-router";
import { useMemo, useState } from "react";
import { MicroLabel } from "@/components/Editorial";
import { api, fmtRelative, fmtUsdc, truncate } from "@/lib/api";
import { EmptyState } from "@/components/States";

export const Route = createFileRoute("/dashboard/receipts")({
  head: () => ({ meta: [{ title: "Receipts — x402/sol" }] }),
  loader: async () => api.getReceipts("srv_01HX3K"),
  component: ReceiptsPage,
});

function ReceiptsPage() {
  const initialReceipts = Route.useLoaderData();
  const [q, setQ] = useState("");
  const [tool, setTool] = useState<string>("all");

  const tools = useMemo(
    () => Array.from(new Set(initialReceipts.map((r) => r.toolName))),
    [initialReceipts],
  );

  const filtered = initialReceipts.filter((r) => {
    if (tool !== "all" && r.toolName !== tool) return false;
    if (!q) return true;
    return (
      r.requestId.toLowerCase().includes(q.toLowerCase()) ||
      r.txSignature.toLowerCase().includes(q.toLowerCase()) ||
      r.payerWallet.toLowerCase().includes(q.toLowerCase())
    );
  });

  return (
    <div className="space-y-8">
      <div>
        <MicroLabel number="03">Receipts</MicroLabel>
        <h1 className="display-md mt-3">Every settled tool call.</h1>
      </div>

      <div className="flex flex-wrap gap-3 items-center">
        <input
          value={q}
          onChange={(e) => setQ(e.target.value)}
          placeholder="Search request id, tx, wallet…"
          className="hairline bg-background px-3 py-2 text-sm font-mono w-full sm:w-80 focus-ring"
        />
        <select
          value={tool}
          onChange={(e) => setTool(e.target.value)}
          className="hairline bg-background px-3 py-2 text-sm font-mono focus-ring"
        >
          <option value="all">All tools</option>
          {tools.map((t) => (
            <option key={t} value={t}>
              {t}
            </option>
          ))}
        </select>
        <span className="ml-auto font-mono text-xs text-muted-foreground">
          {filtered.length} / {initialReceipts.length}
        </span>
      </div>

      {filtered.length === 0 ? (
        <EmptyState
          title="No receipts found"
          description="Adjust filters or wait for a tool call to settle."
        />
      ) : (
        <div className="hairline overflow-x-auto">
          <table className="w-full text-sm min-w-[900px]">
            <thead>
              <tr className="hairline-b text-muted-foreground">
                <th className="text-left px-4 py-3 font-normal text-xs uppercase tracking-wider">
                  Request
                </th>
                <th className="text-left px-4 py-3 font-normal text-xs uppercase tracking-wider">
                  Tool
                </th>
                <th className="text-left px-4 py-3 font-normal text-xs uppercase tracking-wider">
                  Payer
                </th>
                <th className="text-left px-4 py-3 font-normal text-xs uppercase tracking-wider">
                  Tx Signature
                </th>
                <th className="text-right px-4 py-3 font-normal text-xs uppercase tracking-wider">
                  Amount
                </th>
                <th className="text-right px-4 py-3 font-normal text-xs uppercase tracking-wider">
                  Block time
                </th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((r) => (
                <tr key={r.id} className="hairline-b last:border-b-0 hover:bg-card/50">
                  <td className="px-4 py-3 font-mono">
                    <Link
                      to="/dashboard/requests/$requestId"
                      params={{ requestId: r.requestId }}
                      className="text-foreground hover:underline"
                    >
                      {r.requestId}
                    </Link>
                  </td>
                  <td className="px-4 py-3 font-mono">{r.toolName}</td>
                  <td className="px-4 py-3 font-mono text-muted-foreground">
                    {truncate(r.payerWallet)}
                  </td>
                  <td className="px-4 py-3 font-mono text-muted-foreground">
                    <a
                      href={`https://explorer.solana.com/tx/${r.txSignature}?cluster=devnet`}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="hover:text-foreground"
                    >
                      {truncate(r.txSignature, 6, 6)} ↗
                    </a>
                  </td>
                  <td className="px-4 py-3 text-right font-mono">{fmtUsdc(r.amountUsdc)}</td>
                  <td className="px-4 py-3 text-right text-muted-foreground">
                    {fmtRelative(r.blockTime)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
