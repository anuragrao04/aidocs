import { useEffect, useRef, useState } from "react";
import { cn } from "@/lib/cn";

type Line = { kind: "cmd" | "out"; text: string };

export function Terminal({
  lines,
  active,
  speed = 18,
  className,
  height = 200,
}: {
  lines: Line[];
  active: boolean;
  speed?: number;
  className?: string;
  height?: number;
}) {
  const [shown, setShown] = useState<{ line: number; col: number }>({
    line: 0,
    col: 0,
  });
  const startedRef = useRef(false);
  // Always read the latest lines via a ref so prop-identity changes
  // on the parent don't cancel the in-flight typewriter.
  const linesRef = useRef(lines);
  linesRef.current = lines;
  const speedRef = useRef(speed);
  speedRef.current = speed;
  const reduced = usePrefersReducedMotion();

  useEffect(() => {
    if (!active || startedRef.current) return;
    startedRef.current = true;
    if (reduced) {
      setShown({ line: linesRef.current.length, col: 0 });
      return;
    }
    let li = 0;
    let ci = 0;
    const tick = () => {
      const ls = linesRef.current;
      const line = ls[li];
      if (!line) return;
      if (ci < line.text.length) {
        ci++;
        setShown({ line: li, col: ci });
        const delay = line.kind === "cmd" ? speedRef.current : 4;
        setTimeout(tick, delay);
      } else {
        li++;
        ci = 0;
        setShown({ line: li, col: 0 });
        if (li < ls.length) setTimeout(tick, ls[li].kind === "out" ? 120 : 280);
      }
    };
    tick();
  }, [active, reduced]);

  return (
    <div
      className={cn(
        "overflow-hidden rounded-[12px] border border-[var(--color-border)] bg-[#0b0d12] font-mono text-[12.5px] leading-[1.7] text-zinc-100 shadow-xl",
        className,
      )}
    >
      <div className="flex items-center gap-1.5 border-b border-white/5 px-3 py-2">
        <span className="h-2.5 w-2.5 rounded-full bg-red-400/80" />
        <span className="h-2.5 w-2.5 rounded-full bg-yellow-400/80" />
        <span className="h-2.5 w-2.5 rounded-full bg-green-400/80" />
        <span className="ml-2 text-[11px] text-zinc-500">~ aidocs</span>
      </div>
      <div
        className="p-4"
        style={{ height, overflow: "hidden" }}
        aria-live="polite"
      >
        {lines.map((l, i) => {
          const isCurrent = i === shown.line;
          const reveal = i < shown.line ? l.text.length : isCurrent ? shown.col : 0;
          if (reveal === 0 && !isCurrent) return null;
          return (
            <div key={i} className="whitespace-pre-wrap">
              {l.kind === "cmd" ? (
                <span>
                  <span className="text-[var(--color-accent)]">$ </span>
                  <span>{l.text.slice(0, reveal)}</span>
                  {isCurrent && reveal < l.text.length && (
                    <span className="caret">▋</span>
                  )}
                </span>
              ) : (
                <span className="text-zinc-400">{l.text.slice(0, reveal)}</span>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}

export function usePrefersReducedMotion() {
  const [reduced, setReduced] = useState(false);
  useEffect(() => {
    const mq = window.matchMedia("(prefers-reduced-motion: reduce)");
    setReduced(mq.matches);
    const fn = () => setReduced(mq.matches);
    mq.addEventListener("change", fn);
    return () => mq.removeEventListener("change", fn);
  }, []);
  return reduced;
}
