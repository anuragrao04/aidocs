import { motion, AnimatePresence } from "framer-motion";
import { FileText } from "lucide-react";
import { cn } from "@/lib/cn";

export function MockDocsTable({
  hasRow,
  className,
}: {
  hasRow: boolean;
  className?: string;
}) {
  return (
    <div
      className={cn(
        "overflow-hidden rounded-[12px] border border-[var(--color-border)] bg-[var(--color-surface)] shadow-xl",
        className,
      )}
    >
      <div className="border-b border-[var(--color-border)] px-4 py-2.5 text-sm font-semibold">
        Documents
      </div>
      <div className="grid grid-cols-[1fr_80px_90px] gap-2 border-b border-[var(--color-border)] bg-[var(--color-surface-muted)]/50 px-4 py-2 text-[10px] font-medium uppercase tracking-wider text-[var(--color-fg-muted)]">
        <div>Title</div>
        <div>Updated</div>
        <div>Version</div>
      </div>
      <div className="min-h-[140px]">
        <AnimatePresence>
          {hasRow && (
            <motion.div
              initial={{ opacity: 0, y: -16, backgroundColor: "rgba(32,40,255,0.12)" }}
              animate={{ opacity: 1, y: 0, backgroundColor: "rgba(32,40,255,0)" }}
              transition={{ duration: 0.5 }}
              className="grid grid-cols-[1fr_80px_90px] items-center gap-2 px-4 py-3 text-[12.5px]"
            >
              <div className="flex items-center gap-2 truncate">
                <FileText className="h-3.5 w-3.5 text-[var(--color-fg-muted)]" />
                <span className="font-medium">Q3 incident review</span>
              </div>
              <div className="text-[var(--color-fg-muted)]">just now</div>
              <div>
                <span className="rounded-full bg-[var(--color-accent-muted)] px-2 py-0.5 text-[10px] font-medium text-[var(--color-accent)]">
                  v1
                </span>
              </div>
            </motion.div>
          )}
        </AnimatePresence>
        {!hasRow && (
          <div className="flex h-[140px] items-center justify-center text-xs text-[var(--color-fg-muted)]">
            No documents yet
          </div>
        )}
      </div>
    </div>
  );
}
