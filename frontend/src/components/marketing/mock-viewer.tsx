import { motion, AnimatePresence } from "framer-motion";
import { CheckCircle2 } from "lucide-react";
import { cn } from "@/lib/cn";

type CommentState = "none" | "selecting" | "open" | "resolved";

export function MockDocViewer({
  version,
  commentState,
  highlight,
  className,
}: {
  version: 1 | 2;
  commentState: CommentState;
  highlight?: boolean;
  className?: string;
}) {
  return (
    <div
      className={cn(
        "relative overflow-hidden rounded-[12px] border border-[var(--color-border)] bg-[var(--color-surface)] shadow-xl",
        className,
      )}
    >
      {/* doc bar */}
      <div className="flex items-center justify-between border-b border-[var(--color-border)] px-4 py-2.5">
        <div className="flex items-center gap-2 text-sm">
          <span className="font-semibold">Q3 incident review</span>
          <motion.span
            key={version}
            initial={{ scale: 0.6, opacity: 0 }}
            animate={{ scale: 1, opacity: 1 }}
            transition={{ type: "spring", stiffness: 400, damping: 18 }}
            className="rounded-full bg-[var(--color-accent-muted)] px-2 py-0.5 text-[10px] font-medium text-[var(--color-accent)]"
          >
            v{version}
          </motion.span>
        </div>
        <div className="flex gap-1.5">
          <span className="h-2 w-2 rounded-full bg-[var(--color-border-strong)]" />
          <span className="h-2 w-2 rounded-full bg-[var(--color-border-strong)]" />
          <span className="h-2 w-2 rounded-full bg-[var(--color-border-strong)]" />
        </div>
      </div>
      {/* body */}
      <div className="flex">
        <div className="relative flex-1 p-5 text-[11.5px] leading-relaxed text-[var(--color-fg)]">
          <RichDoc highlight={!!highlight} />
          {/* selection bubble overlay positioned over the highlighted phrase */}
          <AnimatePresence>
            {commentState === "selecting" && (
              <motion.div
                initial={{ opacity: 0, y: -4 }}
                animate={{ opacity: 1, y: 0 }}
                exit={{ opacity: 0 }}
                className="pointer-events-none absolute left-[22%] top-[58%] flex items-center gap-1 rounded-md bg-zinc-900 px-2 py-1 text-[10px] font-medium text-white shadow-lg ring-1 ring-black/20"
              >
                <span className="inline-block h-1.5 w-1.5 rounded-full bg-[var(--color-accent)]" />
                Comment
              </motion.div>
            )}
          </AnimatePresence>
        </div>
        {/* comment rail */}
        <div className="w-[180px] shrink-0 border-l border-[var(--color-border)] bg-[var(--color-surface-muted)]/40 p-2.5">
          <div className="mb-2 text-[10px] font-medium uppercase tracking-wider text-[var(--color-fg-muted)]">
            Comments
          </div>
          <AnimatePresence>
            {(commentState === "open" || commentState === "resolved") && (
              <motion.div
                initial={{ opacity: 0, y: 8 }}
                animate={{ opacity: 1, y: 0 }}
                exit={{ opacity: 0 }}
                className={cn(
                  "rounded-md border border-[var(--color-border)] bg-[var(--color-surface)] p-2 text-[10px]",
                  commentState === "resolved" && "opacity-60",
                )}
              >
                <div className="mb-1 flex items-center justify-between">
                  <span className="font-semibold">alice</span>
                  {commentState === "resolved" ? (
                    <CheckCircle2 className="h-3 w-3 text-[var(--color-success)]" />
                  ) : (
                    <span className="rounded-full bg-[var(--color-accent-muted)] px-1.5 py-px text-[8px] font-medium text-[var(--color-accent)]">
                      open
                    </span>
                  )}
                </div>
                <div className="mb-1 border-l-2 border-[var(--color-border-strong)] pl-1 text-[var(--color-fg-muted)]">
                  …impact window…
                </div>
                <div className="text-[var(--color-fg)]">
                  Clarify the impact window.
                </div>
              </motion.div>
            )}
          </AnimatePresence>
        </div>
      </div>
    </div>
  );
}

function RichDoc({ highlight }: { highlight: boolean }) {
  return (
    <>
      <h3 className="mb-1 text-[15px] font-semibold tracking-tight">
        Q3 payments incident — postmortem
      </h3>
      <div className="mb-3 flex items-center gap-2 text-[10px] text-[var(--color-fg-muted)]">
        <span>Sept 14, 2025</span>
        <span>·</span>
        <span className="rounded bg-red-500/15 px-1.5 py-px text-red-500">
          Sev-1
        </span>
        <span>·</span>
        <span>owner: payments-platform</span>
      </div>
      <p className="mb-3">
        On Sept 14, the payments API returned <code className="rounded bg-[var(--color-surface-muted)] px-1 font-mono text-[10px]">5xx</code> errors for{" "}
        <span
          className={cn(
            "rounded px-0.5 transition-colors duration-300",
            highlight ? "bg-yellow-200/80 dark:bg-yellow-400/30" : "",
          )}
        >
          roughly 12 minutes during the impact window
        </span>
        , affecting 3.2% of merchants.
      </p>

      <h4 className="mb-1 mt-3 text-[12px] font-semibold">Impact</h4>
      <table className="mb-3 w-full text-[10px]">
        <thead className="text-left text-[var(--color-fg-muted)]">
          <tr>
            <th className="border-b border-[var(--color-border)] pb-1 font-medium">
              Metric
            </th>
            <th className="border-b border-[var(--color-border)] pb-1 font-medium">
              Value
            </th>
          </tr>
        </thead>
        <tbody>
          <tr>
            <td className="py-1">Failed requests</td>
            <td className="py-1 font-mono">14,203</td>
          </tr>
          <tr>
            <td className="py-1">Merchants affected</td>
            <td className="py-1 font-mono">3.2%</td>
          </tr>
          <tr>
            <td className="py-1">Revenue at risk</td>
            <td className="py-1 font-mono">$48,200</td>
          </tr>
        </tbody>
      </table>

      <h4 className="mb-1 mt-3 text-[12px] font-semibold">Error rate</h4>
      <div className="mb-3 flex h-14 items-end gap-1 rounded-md bg-[var(--color-surface-muted)]/60 p-2">
        {[12, 18, 14, 22, 88, 95, 76, 30, 14, 11, 9, 10].map((h, i) => (
          <div
            key={i}
            style={{ height: `${h}%` }}
            className={cn(
              "flex-1 rounded-sm",
              h > 50 ? "bg-red-400/80" : "bg-[var(--color-accent)]/70",
            )}
          />
        ))}
      </div>

      <h4 className="mb-1 text-[12px] font-semibold">Root cause</h4>
      <pre className="overflow-hidden rounded-md bg-[var(--color-surface-muted)] p-2 font-mono text-[9.5px] leading-relaxed text-[var(--color-fg)]">
        {`if conn.IdleTimeout < ctx.Deadline() {
  // pool returned a stale conn → 5xx
  return ErrUpstream
}`}
      </pre>
    </>
  );
}
