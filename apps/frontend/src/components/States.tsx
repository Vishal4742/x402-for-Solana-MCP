import { cn } from "@/lib/utils";

export function EmptyState({
  title,
  description,
  action,
  className,
}: {
  title: string;
  description?: string;
  action?: React.ReactNode;
  className?: string;
}) {
  return (
    <div className={cn("hairline p-16 text-center", className)}>
      <div className="mx-auto mb-6 h-16 w-16 hairline rounded-full grid place-items-center font-mono text-xs text-muted-foreground">
        ∅
      </div>
      <h3 className="text-xl tracking-tight">{title}</h3>
      {description && (
        <p className="mt-2 text-sm text-muted-foreground max-w-md mx-auto">{description}</p>
      )}
      {action && <div className="mt-6">{action}</div>}
    </div>
  );
}

export function ErrorState({
  message,
  onRetry,
  className,
}: {
  message: string;
  onRetry?: () => void;
  className?: string;
}) {
  return (
    <div className={cn("hairline p-12 text-center", className)}>
      <div className="micro-label text-status-fail">— Error</div>
      <p className="mt-3 text-foreground">{message}</p>
      {onRetry && (
        <button onClick={onRetry} className="pill mt-6">
          Retry
        </button>
      )}
    </div>
  );
}

export function TableSkeleton({ rows = 6 }: { rows?: number }) {
  return (
    <div className="hairline">
      {Array.from({ length: rows }).map((_, i) => (
        <div key={i} className="hairline-b last:border-b-0 px-4 py-4 flex gap-4">
          <div className="h-3 w-24 bg-muted/40 animate-pulse rounded" />
          <div className="h-3 w-32 bg-muted/40 animate-pulse rounded" />
          <div className="h-3 w-16 bg-muted/40 animate-pulse rounded ml-auto" />
        </div>
      ))}
    </div>
  );
}
