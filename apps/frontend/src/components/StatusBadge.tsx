import { cn } from "@/lib/utils";
import type { RequestStatus } from "@/lib/mock/types";

const styles: Record<RequestStatus | "info", { dot: string; text: string; label: string }> = {
  challenged: { dot: "bg-status-info", text: "text-status-info", label: "Challenged" },
  paid: { dot: "bg-status-paid", text: "text-status-paid", label: "Paid" },
  verified: { dot: "bg-status-info", text: "text-status-info", label: "Verified" },
  executed: { dot: "bg-status-success", text: "text-status-success", label: "Executed" },
  failed: { dot: "bg-status-fail", text: "text-status-fail", label: "Failed" },
  info: { dot: "bg-muted-foreground", text: "text-muted-foreground", label: "Info" },
};

export function StatusBadge({ status, className }: { status: RequestStatus; className?: string }) {
  const s = styles[status] ?? styles.info;
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 border border-hairline rounded-full px-2.5 py-0.5 text-[0.7rem] font-mono uppercase tracking-wider",
        s.text,
        className,
      )}
    >
      <span className={cn("h-1.5 w-1.5 rounded-full", s.dot)} />
      {s.label}
    </span>
  );
}
