import { useCallback } from "react";
import { createPersistentStore } from "./persistent-store";

export type OnboardingState = {
  status: "active" | "skipped" | "complete";
  steps: {
    cli_installed: boolean;
    cli_authed: boolean;
    agent_configured: boolean;
    first_doc_seen: boolean;
  };
};

const initial: OnboardingState = {
  status: "active",
  steps: {
    cli_installed: false,
    cli_authed: false,
    agent_configured: false,
    first_doc_seen: false,
  },
};

const store = createPersistentStore<OnboardingState>(
  "aidocs.onboarding",
  initial,
  (parsed) => {
    const p = parsed as Partial<OnboardingState>;
    return {
      status: p.status || "active",
      steps: { ...initial.steps, ...(p.steps || {}) },
    };
  },
);

const requiredSteps = ["cli_installed", "cli_authed", "agent_configured"] as const;

export function useOnboarding() {
  const state = store.useStore();
  const setStep = useCallback(
    (key: keyof OnboardingState["steps"], value: boolean) => {
      const cur = store.read();
      const next: OnboardingState = {
        ...cur,
        steps: { ...cur.steps, [key]: value },
      };
      // Auto-complete once the required steps are all done.
      if (cur.status === "active" && requiredSteps.every((k) => next.steps[k])) {
        next.status = "complete";
      }
      store.write(next);
    },
    [],
  );
  const skip = useCallback(() => store.write({ ...store.read(), status: "skipped" }), []);
  const reset = useCallback(() => store.write(initial), []);
  return { state, setStep, skip, reset };
}

export function onboardingProgress(s: OnboardingState) {
  const done = requiredSteps.filter((k) => s.steps[k]).length;
  return { done, total: requiredSteps.length };
}

export function shouldShowPill(s: OnboardingState) {
  return s.status === "active";
}

// Marks the first-document step from the start page once documents exist.
export function autoMarkFirstDoc() {
  const cur = store.read();
  if (cur.steps.first_doc_seen) return;
  store.write({ ...cur, steps: { ...cur.steps, first_doc_seen: true } });
}
