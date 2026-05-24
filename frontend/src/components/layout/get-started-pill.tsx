import { Link } from "react-router-dom";
import { Rocket } from "lucide-react";
import { onboardingProgress, shouldShowPill, useOnboarding } from "@/lib/onboarding";

export function GetStartedPill() {
  const { state } = useOnboarding();
  if (!shouldShowPill(state)) return null;
  const { done, total } = onboardingProgress(state);
  return (
    <Link
      to="/app/start"
      className="hidden items-center gap-2 rounded-full border border-[var(--color-accent)]/40 bg-[var(--color-accent-muted)] px-3 py-1 text-xs font-medium text-[var(--color-accent)] hover:bg-[var(--color-accent)]/15 md:inline-flex"
      aria-label={`Get started: ${done} of ${total} complete`}
    >
      <Rocket className="h-3.5 w-3.5" />
      Get started
      <span className="rounded-full bg-[var(--color-accent)]/15 px-1.5 py-px font-mono text-[10px]">
        {done}/{total}
      </span>
    </Link>
  );
}
