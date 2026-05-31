import { useTitle } from "@/lib/use-title";
import { Link, useSearchParams } from "react-router-dom";
import { ArrowLeft, MessageSquareText, Share2, Sparkles } from "lucide-react";
import { Button } from "@/components/ui/button";
import { api } from "@/api";

const HIGHLIGHTS = [
  {
    icon: Sparkles,
    title: "Built for AI output",
    body: "Your agents publish finished documents straight to aidocs.",
  },
  {
    icon: MessageSquareText,
    title: "Comment in context",
    body: "Highlight a passage and leave feedback, just like Google Docs.",
  },
  {
    icon: Share2,
    title: "Share and iterate",
    body: "Versioned, shareable, and ready for the next revision.",
  },
];

export function LoginPage() {
  useTitle("Sign in");
  const [params] = useSearchParams();
  const next = params.get("next") || "/app/documents";
  const url = api.loginURL(next);
  return (
    <div className="relative flex min-h-full flex-col items-center justify-center overflow-hidden bg-[var(--color-bg)] px-6 py-12">
      {/* Subtle accent glow behind the card. */}
      <div
        aria-hidden
        className="pointer-events-none absolute left-1/2 top-1/3 h-[28rem] w-[28rem] -translate-x-1/2 -translate-y-1/2 rounded-full bg-[var(--color-accent)] opacity-[0.07] blur-3xl"
      />
      <Link
        to="/"
        className="absolute left-6 top-6 flex items-center gap-1 text-sm text-[var(--color-fg-muted)] hover:text-[var(--color-fg)]"
      >
        <ArrowLeft className="h-4 w-4" /> Home
      </Link>
      <div className="relative w-full max-w-sm">
        <div className="rounded-2xl border border-[var(--color-border)] bg-[var(--color-surface)] p-8 shadow-sm">
          <div className="mb-7 flex flex-col items-center text-center">
            <img src="/favicon.svg" alt="" className="mb-4 h-14 w-14" />
            <h1 className="text-2xl font-semibold tracking-tight">
              Sign in to aidocs
            </h1>
            <p className="mt-2 text-sm text-[var(--color-fg-muted)]">
              The review layer for documents your agents write.
            </p>
          </div>
          <Button asChild size="lg" className="w-full">
            <a href={url}>
              <GoogleIcon /> Continue with Google
            </a>
          </Button>
        </div>
        <ul className="mt-6 space-y-3">
          {HIGHLIGHTS.map((h) => (
            <li key={h.title} className="flex items-start gap-3">
              <span className="mt-0.5 flex h-7 w-7 shrink-0 items-center justify-center rounded-md bg-[var(--color-accent-muted)] text-[var(--color-accent)]">
                <h.icon className="h-3.5 w-3.5" />
              </span>
              <div>
                <div className="text-sm font-medium">{h.title}</div>
                <div className="text-xs text-[var(--color-fg-muted)]">
                  {h.body}
                </div>
              </div>
            </li>
          ))}
        </ul>
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
