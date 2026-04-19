import { cn } from "@/lib/utils";

export function CodeBlock({
  code,
  language = "json",
  className,
  filename,
}: {
  code: string;
  language?: string;
  className?: string;
  filename?: string;
}) {
  return (
    <div className={cn("hairline bg-card/40", className)}>
      {filename && (
        <div className="hairline-b px-4 py-2 flex items-center justify-between">
          <span className="font-mono text-xs text-muted-foreground">{filename}</span>
          <span className="micro-label">— {language}</span>
        </div>
      )}
      <pre className="p-4 overflow-x-auto text-xs font-mono text-foreground/85 leading-relaxed">
        <code>{code}</code>
      </pre>
    </div>
  );
}
