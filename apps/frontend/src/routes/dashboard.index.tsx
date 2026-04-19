import { createFileRoute } from "@tanstack/react-router";
import { KpiCard } from "@/components/KpiCard";
import { MicroLabel } from "@/components/Editorial";
import { StatusBadge } from "@/components/StatusBadge";
import { api, fmtRelative, fmtUsdc, truncate } from "@/lib/api";
import { Link } from "@tanstack/react-router";

export const Route = createFileRoute("/dashboard/")({
  head: () => ({
    meta: [{ title: "Overview — x402/sol Dashboard" }],
  }),
  loader: async () => {
    const [summary, tools, receipts, requests] = await Promise.all([
      api.getDashboardSummary(),
      api.getTools("srv_01HX3K"),
      api.getReceipts("srv_01HX3K"),
      api.getRequests({ serverId: "srv_01HX3K" }),
    ]);

    return { summary, tools, receipts, requests };
  },
  component: Overview,
});

function Overview() {
  const { receipts, requests, summary, tools } = Route.useLoaderData();
  const recentReceipts = receipts.slice(0, 6);
  const activity = requests.slice(0, 8);
  const paidPct = (summary.paidToolCount / (summary.paidToolCount + summary.freeToolCount)) * 100;

  return (
    <div className="space-y-10">
      <div className="flex flex-col md:flex-row md:items-end justify-between gap-4">
        <div>
          <MicroLabel number="01">Overview</MicroLabel>
          <h1 className="display-md mt-3">agent-tools-prod</h1>
          <p className="mt-2 text-sm text-muted-foreground font-mono">
            srv_01HX3K · gateway.x402sol.dev · devnet
          </p>
        </div>
        <div className="flex gap-2">
          <button className="pill text-xs">Last 24h ⌄</button>
          <button className="pill text-xs">Export ⊕</button>
        </div>
      </div>

      {/* KPIs */}
      <div className="hairline grid grid-cols-2 lg:grid-cols-4">
        <KpiCard
          label="Total revenue"
          value={summary.totalRevenueUsdc.toFixed(3)}
          unit="USDC"
          delta={{ value: summary.revenueDelta7d }}
        />
        <KpiCard
          label="Paid requests"
          value={summary.paidRequests.toString()}
          unit="calls"
          delta={{ value: summary.paidDelta7d }}
        />
        <KpiCard
          label="Failed verifications"
          value={summary.failedVerifications.toString()}
          unit="errors"
          delta={{ value: -8.2, positive: false }}
        />
        <KpiCard label="Avg settlement" value={summary.avgSettlementMs.toString()} unit="ms" />
      </div>

      {/* Paid vs free */}
      <section>
        <div className="flex items-baseline justify-between mb-4">
          <MicroLabel>Paid vs free tools</MicroLabel>
          <span className="font-mono text-xs text-muted-foreground">
            {summary.paidToolCount} paid · {summary.freeToolCount} free
          </span>
        </div>
        <div className="hairline h-10 flex overflow-hidden">
          <div
            className="bg-foreground/90 flex items-center px-3 text-[0.65rem] font-mono text-background"
            style={{ width: `${paidPct}%` }}
          >
            PAID {paidPct.toFixed(0)}%
          </div>
          <div className="flex-1 bg-card flex items-center px-3 text-[0.65rem] font-mono text-muted-foreground">
            FREE {(100 - paidPct).toFixed(0)}%
          </div>
        </div>
      </section>

      <div className="grid grid-cols-1 xl:grid-cols-3 gap-8">
        {/* Recent receipts */}
        <section className="xl:col-span-2">
          <div className="flex items-baseline justify-between mb-4">
            <MicroLabel>Recent receipts</MicroLabel>
            <Link
              to="/dashboard/receipts"
              className="text-xs text-muted-foreground hover:text-foreground"
            >
              View all →
            </Link>
          </div>
          <div className="hairline">
            <table className="w-full text-sm">
              <thead>
                <tr className="hairline-b text-muted-foreground">
                  <th className="text-left px-4 py-3 font-normal text-xs uppercase tracking-wider">
                    Tool
                  </th>
                  <th className="text-left px-4 py-3 font-normal text-xs uppercase tracking-wider">
                    Payer
                  </th>
                  <th className="text-right px-4 py-3 font-normal text-xs uppercase tracking-wider">
                    Amount
                  </th>
                  <th className="text-right px-4 py-3 font-normal text-xs uppercase tracking-wider">
                    When
                  </th>
                </tr>
              </thead>
              <tbody>
                {recentReceipts.map((r) => (
                  <tr key={r.id} className="hairline-b last:border-b-0 hover:bg-card/50">
                    <td className="px-4 py-3 font-mono">{r.toolName}</td>
                    <td className="px-4 py-3 font-mono text-muted-foreground">
                      {truncate(r.payerWallet)}
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
        </section>

        {/* Activity */}
        <section>
          <div className="flex items-baseline justify-between mb-4">
            <MicroLabel>Latest activity</MicroLabel>
          </div>
          <ul className="hairline divide-y divide-hairline">
            {activity.map((r) => (
              <li key={r.id}>
                <Link
                  to="/dashboard/requests/$requestId"
                  params={{ requestId: r.id }}
                  className="px-4 py-3 flex items-center gap-3 hover:bg-card/50 transition-colors"
                >
                  <StatusBadge status={r.status} />
                  <div className="flex-1 min-w-0">
                    <div className="font-mono text-xs truncate">{r.toolName}</div>
                    <div className="text-[0.65rem] text-muted-foreground font-mono">{r.id}</div>
                  </div>
                  <div className="text-[0.65rem] text-muted-foreground font-mono whitespace-nowrap">
                    {fmtRelative(r.createdAt)}
                  </div>
                </Link>
              </li>
            ))}
          </ul>
        </section>
      </div>

      {/* Tool stats */}
      <section>
        <MicroLabel>Top tools · last 24h</MicroLabel>
        <div className="hairline mt-4 grid grid-cols-1 md:grid-cols-3 gap-px bg-hairline">
          {tools
            .filter((t) => t.enabled)
            .slice(0, 6)
            .map((t) => (
              <div key={t.id} className="bg-background p-5">
                <div className="flex items-center justify-between">
                  <span className="font-mono text-sm">{t.toolName}</span>
                  {t.priceUsdc > 0 ? (
                    <span className="text-[0.65rem] font-mono text-status-success">PAID</span>
                  ) : (
                    <span className="text-[0.65rem] font-mono text-muted-foreground">FREE</span>
                  )}
                </div>
                <div className="mt-3 flex items-baseline justify-between">
                  <span className="text-3xl font-light">{t.callsLast24h.toLocaleString()}</span>
                  <span className="text-xs text-muted-foreground font-mono">
                    {t.priceUsdc > 0 ? fmtUsdc(t.priceUsdc) : "—"}
                  </span>
                </div>
              </div>
            ))}
        </div>
      </section>
    </div>
  );
}
