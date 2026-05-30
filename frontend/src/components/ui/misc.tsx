import * as React from "react";
import { Check, Copy } from "lucide-react";
import { cn } from "@/lib/cn";
import { Button } from "./button";

export function Skeleton({
  className,
  ...p
}: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn(
        "animate-pulse rounded-md bg-[var(--color-surface-muted)]",
        className,
      )}
      {...p}
    />
  );
}

export function Kbd({ children }: { children: React.ReactNode }) {
  return (
    <kbd className="inline-flex h-5 min-w-[20px] items-center justify-center rounded border border-[var(--color-border)] bg-[var(--color-surface-muted)] px-1.5 font-mono text-[10px] text-[var(--color-fg-muted)]">
      {children}
    </kbd>
  );
}

export function CodeBlock({
  children,
  copy = true,
  className,
}: {
  children: string;
  copy?: boolean;
  className?: string;
}) {
  const [copied, setCopied] = React.useState(false);
  const onCopy = async () => {
    try {
      await navigator.clipboard.writeText(children);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      // Clipboard is unavailable in insecure contexts or was denied; leave the
      // button state untouched so we don't falsely report success.
    }
  };
  return (
    <div
      className={cn(
        "group relative rounded-[10px] border border-[var(--color-border)] bg-[var(--color-surface-muted)] font-mono text-xs",
        className,
      )}
    >
      <pre className="overflow-x-auto p-3 pr-10 leading-relaxed text-[var(--color-fg)]">
        {children}
      </pre>
      {copy && (
        <>
          <button
            onClick={onCopy}
            className="absolute right-2 top-2 rounded-md p-1.5 text-[var(--color-fg-muted)] opacity-0 transition hover:bg-[var(--color-border)] hover:text-[var(--color-fg)] group-hover:opacity-100 focus:opacity-100"
            aria-label={copied ? "Copied" : "Copy"}
          >
            {copied ? <Check className="h-3.5 w-3.5" /> : <Copy className="h-3.5 w-3.5" />}
          </button>
          <span className="sr-only" role="status" aria-live="polite">
            {copied ? "Copied to clipboard" : ""}
          </span>
        </>
      )}
    </div>
  );
}

export function EmptyState({
  icon,
  title,
  description,
  action,
  className,
}: {
  icon?: React.ReactNode;
  title: string;
  description?: React.ReactNode;
  action?: React.ReactNode;
  className?: string;
}) {
  return (
    <div
      className={cn(
        "flex flex-col items-center justify-center rounded-[14px] border border-dashed border-[var(--color-border)] bg-[var(--color-surface)] p-10 text-center",
        className,
      )}
    >
      {icon && (
        <div className="mb-3 flex h-10 w-10 items-center justify-center rounded-full bg-[var(--color-surface-muted)] text-[var(--color-fg-muted)]">
          {icon}
        </div>
      )}
      <h3 className="text-base font-semibold">{title}</h3>
      {description && (
        <p className="mt-1 max-w-md text-sm text-[var(--color-fg-muted)]">
          {description}
        </p>
      )}
      {action && <div className="mt-4">{action}</div>}
    </div>
  );
}

export function Center({
  children,
  className,
}: {
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <div
      className={cn(
        "flex h-full w-full items-center justify-center p-10 text-sm text-[var(--color-fg-muted)]",
        className,
      )}
    >
      {children}
    </div>
  );
}

export function Avatar({
  name,
  src,
  size = 32,
}: {
  name?: string;
  src?: string;
  size?: number;
}) {
  const initials =
    (name || "?")
      .split(/\s+/)
      .map((x) => x[0])
      .filter(Boolean)
      .join("")
      .slice(0, 2)
      .toUpperCase() || "?";
  const [failed, setFailed] = React.useState(false);
  React.useEffect(() => {
    setFailed(false);
  }, [src]);
  const showImg = src && !failed;
  return (
    <div
      style={{
        width: size,
        height: size,
        fontSize: Math.max(10, Math.floor(size * 0.38)),
      }}
      className="flex shrink-0 items-center justify-center overflow-hidden rounded-full bg-[var(--color-accent-muted)] font-semibold text-[var(--color-accent)]"
    >
      {showImg ? (
        <img
          src={src}
          alt=""
          referrerPolicy="no-referrer"
          onError={() => setFailed(true)}
          className="h-full w-full object-cover"
        />
      ) : (
        initials
      )}
    </div>
  );
}

export { Button };
