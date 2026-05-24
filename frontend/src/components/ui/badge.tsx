import * as React from "react";
import { cn } from "@/lib/cn";

type Variant = "default" | "muted" | "accent" | "success" | "warning" | "danger";

export function Badge({
  className,
  variant = "default",
  ...p
}: React.HTMLAttributes<HTMLSpanElement> & { variant?: Variant }) {
  const map: Record<Variant, string> = {
    default:
      "bg-[var(--color-surface-muted)] text-[var(--color-fg)] border border-[var(--color-border)]",
    muted: "bg-transparent text-[var(--color-fg-muted)] border border-[var(--color-border)]",
    accent:
      "bg-[var(--color-accent-muted)] text-[var(--color-accent)] border border-transparent",
    success: "bg-green-100 text-green-800 dark:bg-green-950 dark:text-green-300",
    warning: "bg-amber-100 text-amber-800 dark:bg-amber-950 dark:text-amber-300",
    danger: "bg-red-100 text-red-800 dark:bg-red-950 dark:text-red-300",
  };
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-medium",
        map[variant],
        className,
      )}
      {...p}
    />
  );
}
