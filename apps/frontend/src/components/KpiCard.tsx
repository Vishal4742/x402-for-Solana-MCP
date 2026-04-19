import { cn } from "@/lib/utils";

export function KpiCard({
  label,
  value,
  delta,
  unit,
  className,
}: {
  label: string;
  value: string;
  delta?: { value: number; positive?: boolean };
  unit?: string;
  className?: string;
}) {
  return (
    <div className={cn("p-6 hairline-r last:border-r-0 hairline-b", className)}>
      <div className="micro-label mb-6">— {label}</div>
      <div className="flex items-baseline gap-2">
        <div className="text-5xl font-light tracking-tight">{value}</div>
        {unit && <div className="text-sm text-muted-foreground font-mono">{unit}</div>}
      </div>
      {delta && (
        <div
          className={cn(
            "mt-3 text-xs font-mono",
            delta.positive === false ? "text-status-fail" : "text-status-success",
          )}
        >
          {delta.value > 0 ? "+" : ""}
          {delta.value.toFixed(1)}% · 7d
        </div>
      )}
    </div>
  );
}
