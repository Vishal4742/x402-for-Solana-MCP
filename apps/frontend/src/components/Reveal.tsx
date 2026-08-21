import { useEffect, useRef, type ElementType, type ReactNode } from "react";

// One-shot scroll reveal: adds `is-in` when the element scrolls into view so the
// CSS `.reveal` / `.reveal-stagger` animation runs once. The hide-before-reveal
// is gated on `.js` (see styles.css + __root), so this degrades to plain content
// without JS and honors prefers-reduced-motion. Fires slightly early via rootMargin
// so the motion settles before the content is actually read.
export function Reveal({
  children,
  as: Tag = "div",
  stagger = false,
  className = "",
}: {
  children: ReactNode;
  as?: ElementType;
  stagger?: boolean;
  className?: string;
}) {
  const ref = useRef<HTMLElement | null>(null);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    if (typeof IntersectionObserver === "undefined") {
      el.classList.add("is-in");
      return;
    }
    const io = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting) {
            entry.target.classList.add("is-in");
            io.unobserve(entry.target);
          }
        }
      },
      { threshold: 0.12, rootMargin: "0px 0px -8% 0px" },
    );
    io.observe(el);
    return () => io.disconnect();
  }, []);

  return (
    <Tag ref={ref} className={`${stagger ? "reveal-stagger" : "reveal"} ${className}`.trim()}>
      {children}
    </Tag>
  );
}
