import { createFileRoute } from "@tanstack/react-router";
import {
  Architecture,
  BigCopy,
  CodeShowcase,
  Features,
  FinalCta,
  Hero,
  LandingFooter,
  LandingHeader,
  Lifecycle,
  StatStrip,
  UseCases,
} from "@/components/landing/LandingSections";

export const Route = createFileRoute("/")({
  head: () => ({
    meta: [
      { title: "x402/sol — USDC billing for MCP servers on Solana" },
      {
        name: "description",
        content:
          "Wallet-native payment gateway for MCP servers. Charge per tool call in USDC. Return a 402 challenge, verify on Solana, execute after settlement.",
      },
      { property: "og:title", content: "x402/sol — Charge per MCP tool call" },
      {
        property: "og:description",
        content:
          "Wrap an existing MCP server in minutes. Price each tool, settle in USDC on Solana, inspect every receipt.",
      },
    ],
  }),
  component: LandingPage,
});

function LandingPage() {
  return (
    <div className="min-h-screen bg-background text-foreground">
      <LandingHeader />
      <main>
        <Hero />
        <StatStrip />
        <Lifecycle />
        <BigCopy />
        <Features />
        <Architecture />
        <UseCases />
        <CodeShowcase />
        <FinalCta />
      </main>
      <LandingFooter />
    </div>
  );
}
