import { createFileRoute } from "@tanstack/react-router";
import { useState } from "react";
import { MicroLabel } from "@/components/Editorial";
import { DetailDrawer } from "@/components/DetailDrawer";
import { api, fmtUsdc } from "@/lib/api";
import type { ToolPricing } from "@/lib/mock/types";

export const Route = createFileRoute("/dashboard/tools")({
  head: () => ({ meta: [{ title: "Tools & Pricing — x402/sol" }] }),
  loader: async () => api.getTools("srv_01HX3K"),
  component: ToolsPage,
});

function ToolsPage() {
  const initialTools = Route.useLoaderData();
  const [tools, setTools] = useState<ToolPricing[]>(initialTools);
  const [selected, setSelected] = useState<ToolPricing | null>(null);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [draftPrice, setDraftPrice] = useState("");

  const updatePrice = (id: string, price: number) => {
    setTools((ts) => ts.map((t) => (t.id === id ? { ...t, priceUsdc: price } : t)));
  };
  const toggle = (id: string) => {
    setTools((ts) => ts.map((t) => (t.id === id ? { ...t, enabled: !t.enabled } : t)));
  };

  return (
    <div className="space-y-8">
      <div>
        <MicroLabel number="02">Tools & Pricing</MicroLabel>
        <h1 className="display-md mt-3">Price each tool independently.</h1>
        <p className="mt-2 text-sm text-muted-foreground max-w-xl">
          Set a USDC price per MCP tool. Free tools stay open. Paid tools issue a 402 challenge
          before execution.
        </p>
      </div>

      <div className="hairline overflow-x-auto">
        <table className="w-full text-sm min-w-[720px]">
          <thead>
            <tr className="hairline-b text-muted-foreground">
              <th className="text-left px-4 py-3 font-normal text-xs uppercase tracking-wider">
                Tool
              </th>
              <th className="text-left px-4 py-3 font-normal text-xs uppercase tracking-wider">
                Description
              </th>
              <th className="text-right px-4 py-3 font-normal text-xs uppercase tracking-wider">
                Price
              </th>
              <th className="text-right px-4 py-3 font-normal text-xs uppercase tracking-wider">
                24h calls
              </th>
              <th className="text-center px-4 py-3 font-normal text-xs uppercase tracking-wider">
                Status
              </th>
              <th className="px-4 py-3"></th>
            </tr>
          </thead>
          <tbody>
            {tools.map((t) => (
              <tr key={t.id} className="hairline-b last:border-b-0 hover:bg-card/40">
                <td className="px-4 py-3 font-mono">{t.toolName}</td>
                <td className="px-4 py-3 text-muted-foreground">{t.description}</td>
                <td className="px-4 py-3 text-right font-mono">
                  {editingId === t.id ? (
                    <form
                      onSubmit={(e) => {
                        e.preventDefault();
                        const v = parseFloat(draftPrice);
                        if (!isNaN(v) && v >= 0) updatePrice(t.id, v);
                        setEditingId(null);
                      }}
                      className="inline-flex items-center gap-1"
                    >
                      <input
                        autoFocus
                        value={draftPrice}
                        onChange={(e) => setDraftPrice(e.target.value)}
                        onBlur={() => setEditingId(null)}
                        className="w-20 hairline bg-background px-2 py-1 text-right font-mono text-xs"
                      />
                      <span className="text-muted-foreground text-xs">USDC</span>
                    </form>
                  ) : (
                    <button
                      onClick={() => {
                        setEditingId(t.id);
                        setDraftPrice(t.priceUsdc.toString());
                      }}
                      className="hover:text-foreground text-foreground/85"
                    >
                      {t.priceUsdc === 0 ? (
                        <span className="text-muted-foreground">free</span>
                      ) : (
                        fmtUsdc(t.priceUsdc)
                      )}
                    </button>
                  )}
                </td>
                <td className="px-4 py-3 text-right font-mono text-muted-foreground">
                  {t.callsLast24h.toLocaleString()}
                </td>
                <td className="px-4 py-3 text-center">
                  <button
                    onClick={() => toggle(t.id)}
                    className={`relative inline-flex h-5 w-9 items-center rounded-full hairline transition-colors ${
                      t.enabled ? "bg-foreground" : "bg-card"
                    }`}
                    aria-label={t.enabled ? "Disable" : "Enable"}
                  >
                    <span
                      className={`inline-block h-3 w-3 rounded-full bg-background transition-transform ${
                        t.enabled ? "translate-x-5" : "translate-x-1"
                      }`}
                    />
                  </button>
                </td>
                <td className="px-4 py-3 text-right">
                  <button
                    onClick={() => setSelected(t)}
                    className="text-xs text-muted-foreground hover:text-foreground"
                  >
                    Inspect →
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <DetailDrawer
        open={!!selected}
        onClose={() => setSelected(null)}
        title={selected?.toolName ?? ""}
      >
        {selected && (
          <div className="space-y-6">
            <div>
              <div className="micro-label mb-2">— Tool</div>
              <div className="font-mono text-2xl">{selected.toolName}</div>
              <p className="mt-2 text-muted-foreground text-sm">{selected.description}</p>
            </div>
            <div className="grid grid-cols-2 gap-px bg-hairline hairline">
              <div className="bg-background p-4">
                <div className="micro-label">— Price</div>
                <div className="mt-2 font-mono text-lg">
                  {selected.priceUsdc === 0 ? "free" : fmtUsdc(selected.priceUsdc)}
                </div>
              </div>
              <div className="bg-background p-4">
                <div className="micro-label">— 24h calls</div>
                <div className="mt-2 font-mono text-lg">
                  {selected.callsLast24h.toLocaleString()}
                </div>
              </div>
              <div className="bg-background p-4">
                <div className="micro-label">— Status</div>
                <div className="mt-2 font-mono text-sm">
                  {selected.enabled ? "Enabled" : "Disabled"}
                </div>
              </div>
              <div className="bg-background p-4">
                <div className="micro-label">— Server</div>
                <div className="mt-2 font-mono text-sm">{selected.serverId}</div>
              </div>
            </div>
            <div>
              <div className="micro-label mb-2">— Endpoint</div>
              <pre className="hairline bg-card/40 p-4 font-mono text-xs overflow-x-auto">
                {`POST /mcp/${selected.serverId}\n{ "tool": "${selected.toolName}" }`}
              </pre>
            </div>
          </div>
        )}
      </DetailDrawer>
    </div>
  );
}
