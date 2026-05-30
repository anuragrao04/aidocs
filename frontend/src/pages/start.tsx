import { useState } from "react";
import { Link } from "react-router-dom";
import {
  ArrowRight,
  Check,
  Circle,
  ExternalLink,
  PartyPopper,
  Rocket,
  Sparkles,
  X,
} from "lucide-react";
import { publicURL } from "@/lib/config";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { CodeBlock } from "@/components/ui/misc";
import { cn } from "@/lib/cn";
import {
  onboardingProgress,
  useOnboarding,
  type OnboardingState,
} from "@/lib/onboarding";

type StepKey = keyof OnboardingState["steps"];

const SKILL_REPO = "github:anuragrao04/aidocs";

const AGENT_TARGETS = [
  { id: "claude-code", label: "Claude Code" },
  { id: "codex", label: "Codex" },
  { id: "cursor", label: "Cursor" },
  { id: "pi", label: "Pi" },
  { id: "opencode", label: "OpenCode" },
  { id: "aider", label: "Aider" },
];

export function StartPage() {
  const { state, setStep, skip } = useOnboarding();
  const { done, total } = onboardingProgress(state);

  const allRequiredDone =
    state.steps.cli_installed &&
    state.steps.cli_authed &&
    state.steps.agent_configured;

  return (
    <div className="mx-auto max-w-3xl px-6 py-10">
      <div className="mb-8 flex items-start justify-between gap-4">
        <div>
          <div className="mb-2 inline-flex items-center gap-2 rounded-full border border-[var(--color-border)] bg-[var(--color-surface)] px-3 py-1 text-xs font-medium text-[var(--color-fg-muted)]">
            <Rocket className="h-3 w-3 text-[var(--color-accent)]" /> Setup
            guide
          </div>
          <h1 className="text-3xl font-semibold tracking-tight">
            Get your agent publishing to aidocs
          </h1>
          <p className="mt-2 max-w-xl text-sm text-[var(--color-fg-muted)]">
            aidocs is meant to be driven by your AI agent — Claude Code,
            Codex, Cursor, Pi, OpenCode, Aider, or anything else. These
            three steps wire it up. About 2 minutes.
          </p>
        </div>
        <button
          onClick={skip}
          className="flex shrink-0 items-center gap-1 rounded-md px-2 py-1 text-xs text-[var(--color-fg-muted)] hover:bg-[var(--color-surface-muted)] hover:text-[var(--color-fg)]"
        >
          Skip tutorial <X className="h-3 w-3" />
        </button>
      </div>

      {allRequiredDone && state.status !== "skipped" && (
        <Card className="mb-6 border-[var(--color-accent)]/30 bg-[var(--color-accent-muted)]/40">
          <CardContent className="flex items-center gap-3 pt-5">
            <PartyPopper className="h-5 w-5 text-[var(--color-accent)]" />
            <div className="flex-1">
              <div className="font-medium">You're all set.</div>
              <div className="text-xs text-[var(--color-fg-muted)]">
                Ask your agent to publish something to aidocs. It'll show
                up here.
              </div>
            </div>
            <Button asChild size="sm">
              <Link to="/app/documents">
                Open documents <ArrowRight className="h-4 w-4" />
              </Link>
            </Button>
          </CardContent>
        </Card>
      )}

      <div className="mb-3 flex items-center gap-2 text-xs text-[var(--color-fg-muted)]">
        <div className="h-1 flex-1 overflow-hidden rounded-full bg-[var(--color-surface-muted)]">
          <div
            className="h-full bg-[var(--color-accent)] transition-all"
            style={{ width: `${(done / total) * 100}%` }}
          />
        </div>
        <span className="font-mono">
          {done}/{total} complete
        </span>
      </div>

      <ol className="space-y-3">
        <Step
          n={1}
          title="Install the CLI on your machine"
          stepKey="cli_installed"
          state={state}
          setStep={setStep}
        >
          <p className="mb-3 text-sm text-[var(--color-fg-muted)]">
            One-time, on the machine where your agent runs. Your agent
            calls this CLI under the hood — you won't need to use it
            directly.
          </p>
          <CodeBlock>{`brew install anuragrao04/tap/aidocs`}</CodeBlock>
          <details className="mt-3 text-xs">
            <summary className="cursor-pointer text-[var(--color-fg-muted)]">
              Not on macOS?
            </summary>
            <div className="mt-2 text-[var(--color-fg-muted)]">
              See alternative install methods for Linux and Windows in
              the{" "}
              <a
                className="text-[var(--color-fg)] underline-offset-2 hover:underline"
                href="https://github.com/anuragrao04/aidocs"
                target="_blank"
                rel="noreferrer"
              >
                GitHub repo
              </a>
              .
            </div>
          </details>
        </Step>

        <Step
          n={2}
          title="Sign in once from your terminal"
          stepKey="cli_authed"
          state={state}
          setStep={setStep}
        >
          <p className="mb-3 text-sm text-[var(--color-fg-muted)]">
            One-time. After this, your agent can publish documents on
            your behalf — no keys or tokens to copy around.
          </p>
          <CodeBlock>{`aidocs auth login`}</CodeBlock>
        </Step>

        <Step
          n={3}
          title="Teach your agent the aidocs skill"
          stepKey="agent_configured"
          state={state}
          setStep={setStep}
        >
          <AgentConfigure />
        </Step>

        <Step
          n={4}
          title="Try it end-to-end (optional)"
          stepKey="first_doc_seen"
          state={state}
          setStep={setStep}
          optional
        >
          <TryItOut />
        </Step>
      </ol>
    </div>
  );
}

function Step({
  n,
  title,
  stepKey,
  state,
  setStep,
  optional,
  children,
}: {
  n: number;
  title: string;
  stepKey: StepKey;
  state: OnboardingState;
  setStep: (k: StepKey, v: boolean) => void;
  optional?: boolean;
  children: React.ReactNode;
}) {
  const done = state.steps[stepKey];
  // Find the first incomplete step among required ones to default-expand.
  const requiredOrder: StepKey[] = [
    "cli_installed",
    "cli_authed",
    "agent_configured",
    "first_doc_seen",
  ];
  const firstIncomplete = requiredOrder.find((k) => !state.steps[k]);
  const [forced, setForced] = useState<boolean | null>(null);
  const expanded = forced ?? (done ? false : firstIncomplete === stepKey);
  return (
    <li>
      <Card
        className={cn(
          "transition-colors",
          done && "bg-[var(--color-surface-muted)]/50",
        )}
      >
        <button
          type="button"
          onClick={() => setForced((v) => (v === null ? !expanded : !v))}
          className="flex w-full items-center gap-3 px-5 py-4 text-left"
        >
          <span
            className={cn(
              "flex h-6 w-6 shrink-0 items-center justify-center rounded-full text-xs font-semibold",
              done
                ? "bg-[var(--color-success)] text-white"
                : "border-2 border-[var(--color-border-strong)] text-[var(--color-fg-muted)]",
            )}
          >
            {done ? <Check className="h-3.5 w-3.5" /> : n}
          </span>
          <div className="flex-1">
            <div
              className={cn(
                "font-medium",
                done && "text-[var(--color-fg-muted)] line-through decoration-1",
              )}
            >
              {title}
              {optional && (
                <span className="ml-2 rounded-full bg-[var(--color-surface-muted)] px-1.5 py-px text-[10px] font-medium text-[var(--color-fg-muted)]">
                  optional
                </span>
              )}
            </div>
          </div>
          {!done && (
            <span className="text-xs text-[var(--color-fg-muted)]">
              {expanded ? "Collapse" : "Show"}
            </span>
          )}
        </button>
        {expanded && (
          <div className="border-t border-[var(--color-border)] px-5 pb-5 pt-4">
            {children}
            {!done && (
              <div className="mt-4 flex justify-end">
                <Button size="sm" onClick={() => setStep(stepKey, true)}>
                  <Check className="h-3.5 w-3.5" /> Mark done
                </Button>
              </div>
            )}
            {done && (
              <div className="mt-3 flex justify-end">
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => setStep(stepKey, false)}
                >
                  <Circle className="h-3.5 w-3.5" /> Mark incomplete
                </Button>
              </div>
            )}
          </div>
        )}
      </Card>
    </li>
  );
}

function AgentConfigure() {
  // `--all` installs the skill across every coding agent on the machine, so the
  // command is the same regardless of agent. The chips below are a static
  // compatibility list, not a selector.
  const cmd = `npx skills add ${SKILL_REPO} -s aidocs --all`;
  return (
    <div>
      <p className="mb-3 text-sm text-[var(--color-fg-muted)]">
        Run this once to install the aidocs skill into the agents you use.
        It tells them: "when the user asks for a doc / report / RCA / spec,
        publish it to aidocs". <code>--all</code> installs across every
        coding agent it finds on your machine.
      </p>
      <CodeBlock>{cmd}</CodeBlock>
      <div className="mt-4">
        <div className="mb-2 text-xs font-medium text-[var(--color-fg-muted)]">
          Works with
        </div>
        <div className="flex flex-wrap gap-1.5">
          {AGENT_TARGETS.map((t) => (
            <span
              key={t.id}
              className="rounded-full border border-[var(--color-border)] px-2.5 py-1 text-xs text-[var(--color-fg-muted)]"
            >
              {t.label}
            </span>
          ))}
        </div>
      </div>
      <p className="mt-4 text-xs text-[var(--color-fg-muted)]">
        Skill source:{" "}
        <a
          href="https://github.com/anuragrao04/aidocs/tree/main/skills/aidocs"
          target="_blank"
          rel="noreferrer"
          className="text-[var(--color-fg)] underline-offset-2 hover:underline"
        >
          skills/aidocs/SKILL.md
        </a>
        . Read it, fork it, change the trigger wording — it's just a
        Markdown file.
      </p>
    </div>
  );
}

function TryItOut() {
  const url = publicURL();
  const prompt = `Download the aidocs sample report and publish it as a reviewable document:

curl -sSL ${url}/onboarding/sample.html -o sample.html

Then use the aidocs CLI to publish sample.html to aidocs. Give me the document URL when you're done.`;
  return (
    <div>
      <p className="mb-3 text-sm text-[var(--color-fg-muted)]">
        Want to see the loop work without writing anything? Copy this
        prompt into your agent — it'll fetch a sample report and
        publish it to aidocs.
      </p>
      <CodeBlock>{prompt}</CodeBlock>
      <div className="mt-4">
        <Button asChild variant="ghost" size="sm">
          <Link to="/app/documents">
            <Sparkles className="h-4 w-4" /> Open documents
            <ExternalLink className="h-3.5 w-3.5" />
          </Link>
        </Button>
      </div>
    </div>
  );
}
