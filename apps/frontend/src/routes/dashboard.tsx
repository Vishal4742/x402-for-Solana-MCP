import { createFileRoute } from "@tanstack/react-router";
import { DashboardShell } from "@/components/dashboard/DashboardShell";

export const Route = createFileRoute("/dashboard")({
  head: () => ({
    meta: [
      { title: "Dashboard — x402/sol" },
      { name: "description", content: "Operator control plane for x402/sol MCP servers." },
    ],
  }),
  component: DashboardShell,
});
