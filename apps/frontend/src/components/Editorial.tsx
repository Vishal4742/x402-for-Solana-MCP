import { cn } from "@/lib/utils";

export function MicroLabel({
  children,
  className,
  number,
}: {
  children: React.ReactNode;
  className?: string;
  number?: string;
}) {
  return (
    <div className={cn("micro-label flex items-center gap-2", className)}>
      {number && <span className="font-mono text-foreground/60">{number}</span>}
      <span>—</span>
      <span>{children}</span>
    </div>
  );
}

export function HairlineDivider({ className }: { className?: string }) {
  return <div className={cn("h-px w-full bg-hairline", className)} />;
}

export function Pill({
  children,
  variant = "outline",
  as: As = "button",
  className,
  ...props
}: {
  children: React.ReactNode;
  variant?: "outline" | "solid";
  as?: "button" | "a" | "span";
  className?: string;
} & React.HTMLAttributes<HTMLElement>) {
  return (
    <As
      className={cn("pill focus-ring", variant === "solid" && "pill-solid", className)}
      {...(props as object)}
    >
      {children}
    </As>
  );
}
