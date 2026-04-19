import { useEffect, useRef } from "react";

/**
 * Bayer-dithered animated payment-stream canvas.
 * Optimizations:
 *  - ImageData pixel writes (one putImageData/frame, no per-cell fillRect)
 *  - Capped to ~30fps (visual is grainy; higher fps is wasted CPU)
 *  - Pauses when offscreen (IntersectionObserver)
 *  - Pauses when tab is hidden (visibilitychange)
 *  - Honors prefers-reduced-motion (renders one static frame)
 *  - Caps DPR at 1 (canvas is decorative + already pixelated by design)
 */
const BAYER = [
  [0, 8, 2, 10],
  [12, 4, 14, 6],
  [3, 11, 1, 9],
  [15, 7, 13, 5],
];

const FRAME_MS = 1000 / 30;

export function DitherCanvas({ className }: { className?: string }) {
  const ref = useRef<HTMLCanvasElement>(null);

  useEffect(() => {
    const canvas = ref.current;
    if (!canvas) return;
    const ctx = canvas.getContext("2d", { alpha: false });
    if (!ctx) return;

    const reduced = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    const PIXEL = 4; // logical px — DPR fixed at 1, design intent is pixelated
    let raf = 0;
    let t = 0;
    let lastDraw = 0;
    let visible = true;
    let inView = true;
    let cols = 0;
    let rows = 0;
    let imageData: ImageData | null = null;
    let buf: Uint32Array | null = null;

    function resize() {
      if (!canvas || !ctx) return;
      const rect = canvas.getBoundingClientRect();
      const w = Math.max(1, Math.floor(rect.width / PIXEL));
      const h = Math.max(1, Math.floor(rect.height / PIXEL));
      cols = w;
      rows = h;
      canvas.width = w;
      canvas.height = h;
      // Scale up via CSS — gives the chunky pixel look for free
      canvas.style.imageRendering = "pixelated";
      imageData = ctx.createImageData(w, h);
      buf = new Uint32Array(imageData.data.buffer);
      drawFrame(true);
    }

    function drawFrame(force = false) {
      if (!ctx || !imageData || !buf) return;
      const w = cols;
      const h = rows;
      const sinT06 = t * 0.6;
      const sinTm04 = -t * 0.4;
      const cosT03 = t * 0.3;

      for (let y = 0; y < h; y++) {
        const ny = y / h;
        const nyEdge = Math.pow(Math.max(0, 1 - Math.abs(ny - 0.5) * 1.6), 1.5);
        const w3yFade = Math.exp(-Math.abs(ny - 0.5) * 3);
        const bayerRow = BAYER[y & 3];
        const rowOff = y * w;
        for (let x = 0; x < w; x++) {
          const nx = x / w;
          const w1 = Math.sin(nx * 6 + sinT06) * 0.5 + 0.5;
          const w2 = Math.sin(nx * 3 + sinTm04 + ny * 4) * 0.5 + 0.5;
          const w3 = Math.abs(Math.cos((nx - 0.5) * 8 + cosT03)) * w3yFade;
          let v = (w1 * 0.4 + w2 * 0.35 + w3 * 0.25) * nyEdge;
          v *= 0.4 + 0.6 * Math.sin(nx * Math.PI);

          const threshold = bayerRow[x & 3] / 16;
          const lit = v > threshold + 0.18;

          // ABGR little-endian: 0xAABBGGRR
          if (lit) {
            const a = Math.min(1, (v - threshold) * 1.4) * 0.55;
            const ai = (a * 255) | 0;
            buf[rowOff + x] = (ai << 24) | (250 << 16) | (250 << 8) | 250;
          } else {
            buf[rowOff + x] = (255 << 24) | (10 << 16) | (10 << 8) | 10; // bg #0a0a0a
          }
        }
      }
      ctx.putImageData(imageData, 0, 0);
      if (force) lastDraw = performance.now();
    }

    function loop(now: number) {
      raf = 0;
      if (!visible || !inView) return;
      if (now - lastDraw >= FRAME_MS) {
        t += 0.06; // larger step compensates for lower fps
        drawFrame();
        lastDraw = now;
      }
      raf = requestAnimationFrame(loop);
    }

    function start() {
      if (raf || reduced) return;
      lastDraw = performance.now();
      raf = requestAnimationFrame(loop);
    }
    function stop() {
      if (raf) cancelAnimationFrame(raf);
      raf = 0;
    }

    resize();
    if (reduced) {
      drawFrame(true);
    } else {
      start();
    }

    const onVis = () => {
      visible = !document.hidden;
      if (visible && inView) {
        start();
      } else {
        stop();
      }
    };
    const io = new IntersectionObserver(
      ([entry]) => {
        inView = entry.isIntersecting;
        if (visible && inView) {
          start();
        } else {
          stop();
        }
      },
      { threshold: 0.01 },
    );
    io.observe(canvas);

    let resizeRaf = 0;
    const onResize = () => {
      if (resizeRaf) cancelAnimationFrame(resizeRaf);
      resizeRaf = requestAnimationFrame(resize);
    };

    window.addEventListener("resize", onResize);
    document.addEventListener("visibilitychange", onVis);

    return () => {
      stop();
      io.disconnect();
      window.removeEventListener("resize", onResize);
      document.removeEventListener("visibilitychange", onVis);
      if (resizeRaf) cancelAnimationFrame(resizeRaf);
    };
  }, []);

  return <canvas ref={ref} className={className} aria-hidden />;
}
