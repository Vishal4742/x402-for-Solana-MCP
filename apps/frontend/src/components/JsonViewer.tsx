import { cn } from "@/lib/utils";

export function JsonViewer({ data, className }: { data: unknown; className?: string }) {
  return (
    <pre
      className={cn(
        "hairline bg-card/40 p-4 text-xs font-mono text-foreground/80 overflow-x-auto leading-relaxed",
        className,
      )}
    >
      <code>{JSON.stringify(data, null, 2)}</code>
    </pre>
  );
}
