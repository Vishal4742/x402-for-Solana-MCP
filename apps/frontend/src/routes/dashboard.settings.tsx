import { createFileRoute } from "@tanstack/react-router";
import { useState } from "react";
import { MicroLabel } from "@/components/Editorial";
import { api } from "@/lib/api";
import { notFound } from "@tanstack/react-router";

export const Route = createFileRoute("/dashboard/settings")({
  head: () => ({ meta: [{ title: "Server Settings — x402/sol" }] }),
  loader: async () => {
    const server = await api.getServer("srv_01HX3K");
    if (!server) throw notFound();
    return server;
  },
  component: SettingsPage,
});

function SettingsPage() {
  const server = Route.useLoaderData();
  const [revealed, setRevealed] = useState(false);
  const [copied, setCopied] = useState<string | null>(null);

  const copy = async (label: string, value: string) => {
    await navigator.clipboard.writeText(value);
    setCopied(label);
    setTimeout(() => setCopied(null), 1500);
  };

  return (
    <div className="space-y-10 max-w-3xl">
      <div>
        <MicroLabel number="05">Server Settings</MicroLabel>
        <h1 className="display-md mt-3">{server.name}</h1>
      </div>

      <Section label="Identity">
        <Row label="Server ID" value={server.id} mono />
        <Row label="Network" value={server.network} mono />
        <Row label="Created" value={new Date(server.createdAt).toLocaleString()} />
      </Section>

      <Section label="Endpoints">
        <Row
          label="Gateway endpoint"
          value={server.endpoint}
          mono
          onCopy={() => copy("endpoint", server.endpoint)}
          copied={copied === "endpoint"}
        />
        <Row
          label="Webhook URL"
          value={server.webhookUrl}
          mono
          onCopy={() => copy("webhook", server.webhookUrl)}
          copied={copied === "webhook"}
        />
      </Section>

      <Section label="Payout">
        <Row
          label="Payout wallet"
          value={server.payoutWallet}
          mono
          onCopy={() => copy("wallet", server.payoutWallet)}
          copied={copied === "wallet"}
        />
      </Section>

      <Section label="API key">
        <div className="hairline-b py-4 flex flex-col gap-2">
          <div className="flex items-center justify-between gap-4">
            <div className="micro-label shrink-0">— Secret key</div>
            <div className="font-mono text-sm text-foreground/85 flex-1 text-right truncate">
              {revealed ? server.apiKey : "•".repeat(server.apiKey.length)}
            </div>
          </div>
          <div className="flex gap-2 justify-end">
            <button onClick={() => setRevealed((v) => !v)} className="pill text-[0.7rem]">
              {revealed ? "Hide" : "Reveal"}
            </button>
            <button onClick={() => copy("api", server.apiKey)} className="pill text-[0.7rem]">
              {copied === "api" ? "Copied ✓" : "Copy"}
            </button>
            <button className="pill text-[0.7rem]">Rotate ↻</button>
          </div>
        </div>
      </Section>

      <Section label="Danger zone">
        <div className="hairline p-5 flex items-center justify-between">
          <div>
            <div className="text-sm">Pause this server</div>
            <div className="text-xs text-muted-foreground mt-1">
              All paid tool calls will return 503 until resumed.
            </div>
          </div>
          <button className="pill text-xs border-status-fail text-status-fail hover:bg-status-fail hover:text-background">
            Pause Server
          </button>
        </div>
      </Section>
    </div>
  );
}

function Section({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <section>
      <MicroLabel className="mb-4">{label}</MicroLabel>
      <div className="hairline-t">{children}</div>
    </section>
  );
}

function Row({
  label,
  value,
  mono,
  onCopy,
  copied,
}: {
  label: string;
  value: string;
  mono?: boolean;
  onCopy?: () => void;
  copied?: boolean;
}) {
  return (
    <div className="hairline-b py-4 flex items-center justify-between gap-4">
      <div className="micro-label shrink-0">— {label}</div>
      <div className="flex items-center gap-3 min-w-0 flex-1 justify-end">
        <span className={`text-sm truncate ${mono ? "font-mono" : ""}`}>{value}</span>
        {onCopy && (
          <button onClick={onCopy} className="pill text-[0.65rem] shrink-0">
            {copied ? "Copied ✓" : "Copy"}
          </button>
        )}
      </div>
    </div>
  );
}
