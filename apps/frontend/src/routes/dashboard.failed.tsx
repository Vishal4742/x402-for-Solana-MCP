import { createFileRoute, Link } from "@tanstack/react-router";
import { useState } from "react";
import { MicroLabel } from "@/components/Editorial";
import { api, fmtRelative, fmtUsdc, truncate } from "@/lib/api";
import { EmptyState } from "@/components/States";
import type { FailureReason } from "@/lib/mock/types";

export const Route = createFileRoute("/dashboard/failed")({
  head: () => ({ meta: [{ title: "Failed Payments — x402/sol" }] }),
  loader: async () => api.getRequests({ serverId: "srv_01HX3K", status: "failed" }),
  component: FailedPage,
});

const reasonLabels: Record<FailureReason, string> = {
  insufficient_funds: "Insufficient funds",
  signature_invalid: "Signature invalid",
  timeout: "Timeout",
  tool_error: "Tool error",
  verification_failed: "Verification failed",
};

function FailedPage() {
  const failed = Route.useLoaderData();
  const [reason, setReason] = useState<FailureReason | "all">("all");
  const filtered = reason === "all" ? failed : failed.filter((r) => r.failureReason === reason);

  return (
    <div className="space-y-8">
      <div>
        <MicroLabel number="04">Failed Payments</MicroLabel>
        <h1 className="display-md mt-3">Where settlement broke.</h1>
        <p className="mt-2 text-sm text-muted-foreground">
          Inspect the failure reason, payload, and timeline. Retry to re-issue a 402.
        </p>
      </div>

      <div className="flex flex-wrap gap-3">
        {(["all", ...Object.keys(reasonLabels)] as const).map((r) => (
          <button
            key={r}
            onClick={() => setReason(r as FailureReason | "all")}
            className={`pill text-xs ${reason === r ? "pill-solid" : ""}`}
          >
            {r === "all" ? "All" : reasonLabels[r as FailureReason]}
            <span className="ml-1 text-muted-foreground">
              {r === "all" ? failed.length : failed.filter((x) => x.failureReason === r).length}
            </span>
          </button>
        ))}
      </div>

      {filtered.length === 0 ? (
        <EmptyState
          title="No failed payments"
          description="Nothing broken in this filter. Settlement is clean."
        />
      ) : (
        <div className="hairline overflow-x-auto">
          <table className="w-full text-sm min-w-[820px]">
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
                  Reason
                </th>
                <th className="text-right px-4 py-3 font-normal text-xs uppercase tracking-wider">
                  Amount
                </th>
                <th className="text-right px-4 py-3 font-normal text-xs uppercase tracking-wider">
                  When
                </th>
                <th className="px-4 py-3"></th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((r) => (
                <tr key={r.id} className="hairline-b last:border-b-0 hover:bg-card/50">
                  <td className="px-4 py-3 font-mono">
                    <Link
                      to="/dashboard/requests/$requestId"
                      params={{ requestId: r.id }}
                      className="hover:underline"
                    >
                      {r.id}
                    </Link>
                  </td>
                  <td className="px-4 py-3 font-mono">{r.toolName}</td>
                  <td className="px-4 py-3 font-mono text-muted-foreground">
                    {truncate(r.payerWallet)}
                  </td>
                  <td className="px-4 py-3">
                    <span className="inline-flex items-center gap-1.5 text-status-fail text-xs font-mono">
                      <span className="h-1.5 w-1.5 rounded-full bg-status-fail" />
                      {r.failureReason ? reasonLabels[r.failureReason] : "Unknown"}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-right font-mono">{fmtUsdc(r.amountUsdc)}</td>
                  <td className="px-4 py-3 text-right text-muted-foreground">
                    {fmtRelative(r.createdAt)}
                  </td>
                  <td className="px-4 py-3 text-right">
                    <button className="pill text-[0.7rem]">Retry ↻</button>
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
