import { useEffect } from "react";
import { cn } from "@/lib/utils";

export function DetailDrawer({
  open,
  onClose,
  title,
  children,
  className,
}: {
  open: boolean;
  onClose: () => void;
  title: string;
  children: React.ReactNode;
  className?: string;
}) {
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") onClose();
    }
    if (open) document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [open, onClose]);

  if (!open) return null;
  return (
    <div className="fixed inset-0 z-50">
      <div
        className="absolute inset-0 bg-background/70 backdrop-blur-sm"
        onClick={onClose}
        aria-hidden
      />
      <aside
        className={cn(
          "absolute right-0 top-0 h-full w-full max-w-xl bg-background hairline-l overflow-y-auto",
          className,
        )}
      >
        <div className="hairline-b px-6 py-4 flex items-center justify-between sticky top-0 bg-background">
          <div className="micro-label">— {title}</div>
          <button onClick={onClose} className="pill text-xs" aria-label="Close">
            Close ✕
          </button>
        </div>
        <div className="p-6">{children}</div>
      </aside>
    </div>
  );
}
