import { Link, Outlet, useLocation } from "@tanstack/react-router";
import { useState } from "react";
import { cn } from "@/lib/utils";
import { truncate } from "@/lib/api";

const nav = [
  { to: "/dashboard", label: "Overview", code: "01" },
  { to: "/dashboard/tools", label: "Tools & Pricing", code: "02" },
  { to: "/dashboard/receipts", label: "Receipts", code: "03" },
  { to: "/dashboard/failed", label: "Failed Payments", code: "04" },
  { to: "/dashboard/settings", label: "Server Settings", code: "05" },
] as const;

export function DashboardShell() {
  const location = useLocation();
  const [mobileOpen, setMobileOpen] = useState(false);
  const wallet = "8KpL3xN2qR9vF4mT7sH1jY6cB3eW5zA2dG8nM9pQrSkX";

  const isActive = (to: string) =>
    to === "/dashboard" ? location.pathname === "/dashboard" : location.pathname.startsWith(to);

  return (
    <div className="min-h-screen bg-background text-foreground flex">
      {/* Sidebar */}
      <aside
        className={cn(
          "fixed lg:sticky top-0 left-0 z-40 h-screen w-64 hairline-r bg-sidebar shrink-0",
          "transition-transform duration-200",
          mobileOpen ? "translate-x-0" : "-translate-x-full lg:translate-x-0",
        )}
      >
        <div className="hairline-b px-5 h-14 flex items-center justify-between">
          <Link to="/" className="flex items-center gap-2 font-mono text-sm">
            <span className="inline-block h-2 w-2 bg-foreground rounded-full" />
            x402<span className="text-muted-foreground">/sol</span>
          </Link>
          <button
            className="lg:hidden text-muted-foreground"
            onClick={() => setMobileOpen(false)}
            aria-label="Close menu"
          >
            ✕
          </button>
        </div>

        <div className="p-5">
          <div className="micro-label mb-3">— Server</div>
          <button className="w-full hairline rounded-md px-3 py-2 text-left flex items-center justify-between hover:bg-sidebar-accent transition-colors">
            <span className="text-sm">agent-tools-prod</span>
            <span className="text-muted-foreground">⌄</span>
          </button>
          <div className="mt-1 font-mono text-[0.65rem] text-muted-foreground">
            srv_01HX3K · devnet
          </div>
        </div>

        <nav className="px-3 py-2 space-y-0.5">
          {nav.map((n) => {
            const active = isActive(n.to);
            return (
              <Link
                key={n.to}
                to={n.to}
                onClick={() => setMobileOpen(false)}
                className={cn(
                  "flex items-center gap-3 px-3 py-2 rounded-sm text-sm transition-colors focus-ring",
                  active
                    ? "bg-sidebar-accent text-foreground"
                    : "text-foreground/70 hover:text-foreground hover:bg-sidebar-accent/50",
                )}
              >
                <span className="font-mono text-[0.65rem] text-muted-foreground">{n.code}</span>
                <span>{n.label}</span>
                {active && <span className="ml-auto h-1 w-1 rounded-full bg-foreground" />}
              </Link>
            );
          })}
        </nav>

        <div className="absolute bottom-0 inset-x-0 hairline-t p-5">
          <div className="micro-label mb-2">— Network</div>
          <div className="flex items-center gap-2 text-xs font-mono">
            <span className="h-1.5 w-1.5 rounded-full bg-status-success" />
            Solana devnet · 482ms
          </div>
        </div>
      </aside>

      {/* Main */}
      <div className="flex-1 min-w-0 lg:pl-0">
        <header className="sticky top-0 z-30 hairline-b bg-background/85 backdrop-blur-md">
          <div className="h-14 px-5 lg:px-8 flex items-center gap-3">
            <button
              className="lg:hidden pill text-xs"
              onClick={() => setMobileOpen(true)}
              aria-label="Open menu"
            >
              ☰
            </button>
            <div className="font-mono text-[0.7rem] text-muted-foreground hidden sm:block">
              {location.pathname}
            </div>
            <div className="ml-auto flex items-center gap-2">
              <span className="pill text-[0.7rem]">
                <span className="h-1.5 w-1.5 rounded-full bg-status-success" />
                {truncate(wallet, 4, 4)}
              </span>
              <button className="pill pill-solid text-[0.7rem]">New Tool ⊕</button>
            </div>
          </div>
        </header>
        <main className="px-5 lg:px-8 py-8 lg:py-10 max-w-[1400px]">
          <Outlet />
        </main>
      </div>

      {mobileOpen && (
        <div
          className="fixed inset-0 bg-background/60 backdrop-blur-sm z-30 lg:hidden"
          onClick={() => setMobileOpen(false)}
        />
      )}
    </div>
  );
}
