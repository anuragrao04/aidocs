import { Link, useSearchParams } from "react-router-dom";
import { ArrowLeft, Sparkles } from "lucide-react";
import { Button } from "@/components/ui/button";

export function LoginPage() {
  const [params] = useSearchParams();
  const next = params.get("next") || "/app/documents";
  const url = `/v1/auth/google/start?mode=web&redirect=${encodeURIComponent(next)}`;
  return (
    <div className="flex min-h-full flex-col items-center justify-center bg-[var(--color-bg)] px-6 py-12">
      <Link
        to="/"
        className="absolute left-6 top-6 flex items-center gap-1 text-sm text-[var(--color-fg-muted)] hover:text-[var(--color-fg)]"
      >
        <ArrowLeft className="h-4 w-4" /> Home
      </Link>
      <div className="w-full max-w-sm">
        <div className="mb-8 flex flex-col items-center text-center">
          <Sparkles className="mb-3 h-7 w-7 text-[var(--color-accent)]" />
          <h1 className="text-2xl font-semibold tracking-tight">
            Sign in to aidocs
          </h1>
          <p className="mt-2 text-sm text-[var(--color-fg-muted)]">
            Review documents your agents publish.
          </p>
        </div>
        <Button asChild size="lg" className="w-full">
          <a href={url}>
            <GoogleIcon /> Continue with Google
          </a>
        </Button>
        <div className="mt-6 rounded-[12px] border border-[var(--color-border)] bg-[var(--color-surface)] p-4 text-xs text-[var(--color-fg-muted)]">
          <div className="mb-1 font-medium text-[var(--color-fg)]">
            Setting up a headless agent?
          </div>
          For things like n8n that can’t run OAuth interactively, sign in
          here first and mint a key from{" "}
          <span className="font-mono text-[var(--color-fg)]">
            Settings → Service accounts
          </span>
          .
        </div>
      </div>
    </div>
  );
}

function GoogleIcon() {
  return (
    <svg viewBox="0 0 24 24" className="h-4 w-4" aria-hidden>
      <path
        fill="#fff"
        d="M21.35 11.1H12v2.9h5.35c-.23 1.45-1.7 4.25-5.35 4.25-3.22 0-5.85-2.67-5.85-5.95s2.63-5.95 5.85-5.95c1.83 0 3.06.78 3.76 1.45l2.56-2.47C16.66 3.78 14.55 2.8 12 2.8 6.92 2.8 2.8 6.92 2.8 12s4.12 9.2 9.2 9.2c5.31 0 8.83-3.73 8.83-8.98 0-.6-.06-1.06-.15-1.51z"
      />
    </svg>
  );
}
