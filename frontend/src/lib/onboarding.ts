import { useCallback, useEffect, useSyncExternalStore } from "react";

export type OnboardingState = {
  status: "active" | "skipped" | "complete";
  steps: {
    cli_installed: boolean;
    cli_authed: boolean;
    agent_configured: boolean;
    first_doc_seen: boolean;
  };
};

const KEY = "aidocs.onboarding";
const EVT = "aidocs:onboarding-change";

const initial: OnboardingState = {
  status: "active",
  steps: {
    cli_installed: false,
    cli_authed: false,
    agent_configured: false,
    first_doc_seen: false,
  },
};

// Cache the snapshot so useSyncExternalStore sees a stable reference
// between renders unless the underlying string actually changes.
let cachedRaw: string | null = null;
let cachedState: OnboardingState = initial;

function read(): OnboardingState {
  let raw: string | null = null;
  try {
    raw = localStorage.getItem(KEY);
  } catch {
    return initial;
  }
  if (raw === cachedRaw) return cachedState;
  cachedRaw = raw;
  if (!raw) {
    cachedState = initial;
    return cachedState;
  }
  try {
    const parsed = JSON.parse(raw);
    cachedState = {
      status: parsed.status || "active",
      steps: { ...initial.steps, ...(parsed.steps || {}) },
    };
  } catch {
    cachedState = initial;
  }
  return cachedState;
}

function write(next: OnboardingState) {
  localStorage.setItem(KEY, JSON.stringify(next));
  window.dispatchEvent(new Event(EVT));
}

function subscribe(cb: () => void) {
  const onStorage = (e: StorageEvent) => {
    if (e.key === KEY) cb();
  };
  window.addEventListener(EVT, cb);
  window.addEventListener("storage", onStorage);
  return () => {
    window.removeEventListener(EVT, cb);
    window.removeEventListener("storage", onStorage);
  };
}

export function useOnboarding() {
  const state = useSyncExternalStore(subscribe, read, () => initial);
  const setStep = useCallback(
    (key: keyof OnboardingState["steps"], value: boolean) => {
      const cur = read();
      const next: OnboardingState = {
        ...cur,
        steps: { ...cur.steps, [key]: value },
      };
      // auto-complete when the three required steps are done
      const required = ["cli_installed", "cli_authed", "agent_configured"] as const;
      if (cur.status === "active" && required.every((k) => next.steps[k])) {
        next.status = "complete";
      }
      write(next);
    },
    [],
  );
  const skip = useCallback(() => write({ ...read(), status: "skipped" }), []);
  const reset = useCallback(() => write(initial), []);
  return { state, setStep, skip, reset };
}

export function onboardingProgress(s: OnboardingState) {
  const required = ["cli_installed", "cli_authed", "agent_configured"] as const;
  const done = required.filter((k) => s.steps[k]).length;
  return { done, total: required.length };
}

export function shouldShowPill(s: OnboardingState) {
  return s.status === "active";
}

// Used by the auto-detection on the start page when documents exist.
export function autoMarkFirstDoc() {
  const cur = read();
  if (cur.steps.first_doc_seen) return;
  write({ ...cur, steps: { ...cur.steps, first_doc_seen: true } });
}
