import { createFileRoute, Link, notFound } from "@tanstack/react-router";
import { MicroLabel } from "@/components/Editorial";
import { StatusBadge } from "@/components/StatusBadge";
import { JsonViewer } from "@/components/JsonViewer";
import { api, fmtRelative, fmtUsdc, truncate } from "@/lib/api";
import { cn } from "@/lib/utils";
import type { PaymentRequest, TimelineEvent } from "@/lib/mock/types";

export const Route = createFileRoute("/dashboard/requests/$requestId")({
  head: ({ params }) => ({
    meta: [{ title: `Request ${params.requestId} — x402/sol` }],
  }),
  loader: async ({ params }) => {
    const req = await api.getRequest(params.requestId);
    if (!req) throw notFound();
    return req;
  },
  notFoundComponent: () => (
    <div className="p-12 text-center">
      <MicroLabel className="justify-center">— 404</MicroLabel>
      <h2 className="display-md mt-4">Request not found</h2>
      <Link to="/dashboard/receipts" className="pill mt-6 inline-flex">
        Back to receipts
      </Link>
    </div>
  ),
  errorComponent: ({ error }) => (
    <div className="p-12 text-center">
      <MicroLabel className="justify-center text-status-fail">— Error</MicroLabel>
      <p className="mt-4">{error.message}</p>
    </div>
  ),
  component: RequestDetail,
});

function RequestDetail() {
  const req = Route.useLoaderData() as PaymentRequest;

  return (
    <div className="space-y-10">
      <div>
        <Link
          to="/dashboard/receipts"
          className="text-xs text-muted-foreground hover:text-foreground"
        >
          ← Receipts
        </Link>
        <div className="mt-4 flex items-start justify-between flex-wrap gap-4">
          <div>
            <MicroLabel number="06">Request</MicroLabel>
            <h1 className="display-md mt-3 font-mono">{req.id}</h1>
            <div className="mt-3 flex items-center gap-3 text-sm text-muted-foreground">
              <span className="font-mono">{req.toolName}</span>
              <span>·</span>
              <span>{fmtRelative(req.createdAt)}</span>
            </div>
          </div>
          <StatusBadge status={req.status} />
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
        {/* Timeline */}
        <section className="lg:col-span-2">
          <MicroLabel className="mb-6">Timeline</MicroLabel>
          <ol className="relative">
            {req.timeline.map((ev: TimelineEvent, i: number) => {
              const isLast = i === req.timeline.length - 1;
              const failed = ev.status === "failed";
              return (
                <li key={i} className="pl-8 pb-8 relative">
                  {!isLast && (
                    <span
                      className={cn(
                        "absolute left-2 top-3 bottom-0 w-px",
                        failed ? "bg-status-fail/40" : "bg-hairline",
                      )}
                    />
                  )}
                  <span
                    className={cn(
                      "absolute left-0 top-1.5 h-4 w-4 rounded-full border-2 border-background",
                      failed
                        ? "bg-status-fail"
                        : ev.status === "executed"
                          ? "bg-status-success"
                          : ev.status === "paid"
                            ? "bg-status-paid"
                            : "bg-status-info",
                    )}
                  />
                  <div className="flex items-center justify-between gap-4">
                    <div className="flex items-center gap-3">
                      <span className="font-mono text-xs text-muted-foreground">
                        {String(i + 1).padStart(2, "0")}
                      </span>
                      <StatusBadge status={ev.status} />
                    </div>
                    <div className="font-mono text-xs text-muted-foreground">
                      {new Date(ev.timestamp).toLocaleTimeString()}
                      {ev.durationMs != null && (
                        <span className="ml-2 text-foreground/60">+{ev.durationMs}ms</span>
                      )}
                    </div>
                  </div>
                  {ev.payload && (
                    <pre className="mt-3 hairline bg-card/40 p-3 text-[0.7rem] font-mono text-foreground/75 overflow-x-auto">
                      {JSON.stringify(ev.payload, null, 2)}
                    </pre>
                  )}
                  {ev.signatureHash && (
                    <div className="mt-2 font-mono text-[0.65rem] text-muted-foreground">
                      sig: {truncate(ev.signatureHash, 10, 10)}
                    </div>
                  )}
                </li>
              );
            })}
          </ol>
        </section>

        {/* Side panel */}
        <aside className="space-y-6">
          <div className="hairline">
            <Field label="Payer">
              <span className="font-mono text-xs">{truncate(req.payerWallet, 6, 6)}</span>
            </Field>
            <Field label="Amount">
              <span className="font-mono">{fmtUsdc(req.amountUsdc)}</span>
            </Field>
            <Field label="Tool">
              <span className="font-mono text-xs">{req.toolName}</span>
            </Field>
            <Field label="Server">
              <span className="font-mono text-xs">{req.serverId}</span>
            </Field>
            {req.txSignature && (
              <Field label="Tx signature">
                <a
                  href={`https://explorer.solana.com/tx/${req.txSignature}?cluster=devnet`}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="font-mono text-xs hover:text-foreground text-foreground/80"
                >
                  {truncate(req.txSignature, 6, 6)} ↗
                </a>
              </Field>
            )}
            {req.failureReason && (
              <Field label="Failure">
                <span className="font-mono text-xs text-status-fail">{req.failureReason}</span>
              </Field>
            )}
          </div>

          <div>
            <MicroLabel className="mb-3">Raw request</MicroLabel>
            <JsonViewer data={req.rawRequest} />
          </div>
          {req.rawResponse && (
            <div>
              <MicroLabel className="mb-3">Raw response</MicroLabel>
              <JsonViewer data={req.rawResponse} />
            </div>
          )}
        </aside>
      </div>
    </div>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="hairline-b last:border-b-0 px-4 py-3 flex items-center justify-between gap-4">
      <span className="micro-label">— {label}</span>
      <span className="text-right">{children}</span>
    </div>
  );
}
