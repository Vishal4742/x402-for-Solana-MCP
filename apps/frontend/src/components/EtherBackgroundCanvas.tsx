import { useEffect, useRef } from "react";

type Rgb = [number, number, number];

const TOP: Rgb = [50, 70, 84];
const MID_HIGH: Rgb = [113, 133, 147];
const HORIZON: Rgb = [208, 209, 206];
const MID_LOW: Rgb = [237, 161, 103];
const BOTTOM: Rgb = [212, 97, 40];

const FRAME_MS = 1000 / 24;

function mix(a: Rgb, b: Rgb, t: number): Rgb {
  const clamped = Math.max(0, Math.min(1, t));
  return [
    a[0] + (b[0] - a[0]) * clamped,
    a[1] + (b[1] - a[1]) * clamped,
    a[2] + (b[2] - a[2]) * clamped,
  ];
}

function smoothstep(edge0: number, edge1: number, x: number) {
  const t = Math.max(0, Math.min(1, (x - edge0) / (edge1 - edge0)));
  return t * t * (3 - 2 * t);
}

function random(x: number, y: number, time: number) {
  const value = Math.sin(x * 12.9898 + y * 78.233 + time * 37.719) * 43758.5453123;
  return value - Math.floor(value);
}

function softNoise(x: number, y: number, time: number) {
  const ix = Math.floor(x);
  const iy = Math.floor(y);
  const fx = x - ix;
  const fy = y - iy;
  const ux = fx * fx * (3 - 2 * fx);
  const uy = fy * fy * (3 - 2 * fy);

  const a = random(ix, iy, time);
  const b = random(ix + 1, iy, time);
  const c = random(ix, iy + 1, time);
  const d = random(ix + 1, iy + 1, time);
  const top = a + (b - a) * ux;
  const bottom = c + (d - c) * ux;

  return top + (bottom - top) * uy;
}

function gradientAt(y: number): Rgb {
  if (y > 0.65) return mix(MID_HIGH, TOP, smoothstep(0.65, 1, y));
  if (y > 0.4) return mix(HORIZON, MID_HIGH, smoothstep(0.4, 0.65, y));
  if (y > 0.15) return mix(MID_LOW, HORIZON, smoothstep(0.15, 0.4, y));
  return mix(BOTTOM, MID_LOW, smoothstep(0, 0.15, y));
}

export function EtherBackgroundCanvas({ className }: { className?: string }) {
  const ref = useRef<HTMLCanvasElement>(null);

  useEffect(() => {
    const canvas = ref.current;
    if (!canvas) return;

    const ctx = canvas.getContext("2d", { alpha: false });
    if (!ctx) return;

    const reduced = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    const dpr = Math.min(window.devicePixelRatio || 1, 1.5);
    const renderScale = 0.42;
    let raf = 0;
    let lastDraw = 0;
    let visible = true;
    let inView = true;
    let width = 1;
    let height = 1;
    let imageData: ImageData | null = null;

    function resize() {
      const rect = canvas.getBoundingClientRect();
      width = Math.max(1, Math.floor(rect.width * dpr * renderScale));
      height = Math.max(1, Math.floor(rect.height * dpr * renderScale));
      canvas.width = width;
      canvas.height = height;
      canvas.style.imageRendering = "auto";
      imageData = ctx.createImageData(width, height);
      drawFrame(performance.now());
    }

    function drawFrame(now: number) {
      if (!imageData) return;

      const data = imageData.data;
      const time = now * 0.001;

      for (let y = 0; y < height; y++) {
        for (let x = 0; x < width; x++) {
          const nx = x / width;
          const ny = y / height;
          const drift = Math.sin(nx * 2 + time * 0.08) * 0.008;
          const color = gradientAt(1 - ny + drift);
          const mist = (softNoise(nx * 3.2, ny * 3.2 - time * 0.04, time * 0.15) - 0.5) * 10;
          const grain = 0;
          const vignette = Math.hypot(nx - 0.5, ny - 0.5) * 16;
          const i = (y * width + x) * 4;

          data[i] = Math.max(0, Math.min(255, color[0] + mist + grain - vignette));
          data[i + 1] = Math.max(0, Math.min(255, color[1] + mist + grain - vignette));
          data[i + 2] = Math.max(0, Math.min(255, color[2] + mist + grain - vignette));
          data[i + 3] = 255;
        }
      }

      ctx.putImageData(imageData, 0, 0);
    }

    function loop(now: number) {
      raf = 0;
      if (!visible || !inView) return;
      if (now - lastDraw >= FRAME_MS) {
        drawFrame(now);
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
    if (!reduced) start();

    const onVisibility = () => {
      visible = !document.hidden;
      if (visible && inView) start();
      else stop();
    };

    const observer = new IntersectionObserver(
      ([entry]) => {
        inView = entry.isIntersecting;
        if (visible && inView) start();
        else stop();
      },
      { threshold: 0.01 },
    );

    let resizeRaf = 0;
    const onResize = () => {
      if (resizeRaf) cancelAnimationFrame(resizeRaf);
      resizeRaf = requestAnimationFrame(resize);
    };

    observer.observe(canvas);
    window.addEventListener("resize", onResize);
    document.addEventListener("visibilitychange", onVisibility);

    return () => {
      stop();
      observer.disconnect();
      window.removeEventListener("resize", onResize);
      document.removeEventListener("visibilitychange", onVisibility);
      if (resizeRaf) cancelAnimationFrame(resizeRaf);
    };
  }, []);

  return <canvas ref={ref} className={className} aria-hidden />;
}
