import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { Bot, KeyRound, Plus, Power, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { CodeBlock, EmptyState, Skeleton } from "@/components/ui/misc";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import { api, type ServiceAccount } from "@/api";

export function ServiceAccountsPage() {
  const q = useQueryClient();
  const sas = useQuery({
    queryKey: ["service-accounts"],
    queryFn: api.listServiceAccounts,
  });
  const [name, setName] = useState("");
  const create = useMutation({
    mutationFn: () => api.createServiceAccount(name),
    onSuccess: (r) => {
      setName("");
      toast.success(`Created ${r.name}`);
      q.invalidateQueries({ queryKey: ["service-accounts"] });
      setSelectedId(r.id);
    },
    onError: (e) => toast.error(e instanceof Error ? e.message : "Failed"),
  });
  const items = sas.data?.items || [];
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const selected =
    items.find((s) => (s.id || s.ID) === selectedId) || items[0] || null;

  return (
    <div className="mx-auto max-w-6xl px-6 py-10">
      <div className="mb-6">
        <h1 className="text-2xl font-semibold tracking-tight">
          Service accounts
        </h1>
        <p className="mt-1 max-w-2xl text-sm text-[var(--color-fg-muted)]">
          For headless agents that can’t run an interactive OAuth flow —
          n8n workflows, scheduled jobs, CI pipelines. If you’re using the
          CLI from a terminal, run{" "}
          <code className="rounded bg-[var(--color-surface-muted)] px-1 font-mono text-xs">
            aidocs auth login
          </code>{" "}
          instead.
        </p>
      </div>

      <div className="grid gap-6 md:grid-cols-[280px_1fr]">
        <div className="space-y-2">
          <form
            className="flex gap-2"
            onSubmit={(e) => {
              e.preventDefault();
              if (name) create.mutate();
            }}
          >
            <Input
              placeholder="report-writer-bot"
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
            <Button type="submit" size="icon" disabled={!name || create.isPending}>
              <Plus className="h-4 w-4" />
            </Button>
          </form>
          {sas.isLoading ? (
            <>
              <Skeleton className="h-12 w-full" />
              <Skeleton className="h-12 w-full" />
            </>
          ) : items.length === 0 ? (
            <div className="rounded-[12px] border border-dashed border-[var(--color-border)] p-6 text-center text-sm text-[var(--color-fg-muted)]">
              No service accounts yet.
            </div>
          ) : (
            <ul className="space-y-1">
              {items.map((sa) => {
                const id = sa.id || sa.ID || "";
                const active = (selected?.id || selected?.ID) === id;
                const disabled = sa.disabled || sa.Disabled;
                return (
                  <li key={id}>
                    <button
                      onClick={() => setSelectedId(id)}
                      className={`flex w-full items-center gap-2 rounded-md border px-3 py-2 text-left text-sm transition-colors ${active ? "border-[var(--color-accent)] bg-[var(--color-accent-muted)]" : "border-[var(--color-border)] bg-[var(--color-surface)] hover:bg-[var(--color-surface-muted)]"}`}
                    >
                      <Bot className="h-4 w-4 shrink-0 text-[var(--color-fg-muted)]" />
                      <div className="min-w-0 flex-1">
                        <div className="truncate font-medium">
                          {sa.name || sa.Name}
                        </div>
                        <div className="truncate font-mono text-[10px] text-[var(--color-fg-muted)]">
                          {id}
                        </div>
                      </div>
                      {disabled && <Badge variant="warning">disabled</Badge>}
                    </button>
                  </li>
                );
              })}
            </ul>
          )}
        </div>

        <div>
          {selected ? (
            <ServiceAccountDetail sa={selected} />
          ) : (
            <EmptyState
              icon={<Bot className="h-5 w-5" />}
              title="No service account selected"
              description="Create one on the left to get started."
            />
          )}
        </div>
      </div>
    </div>
  );
}

function ServiceAccountDetail({ sa }: { sa: ServiceAccount }) {
  const id = sa.id || sa.ID || "";
  const name = sa.name || sa.Name || "";
  const disabled = sa.disabled || sa.Disabled || false;
  const q = useQueryClient();
  const keys = useQuery({
    queryKey: ["service-account-keys", id],
    queryFn: () => api.listServiceAccountKeys(id),
  });
  const [revealToken, setRevealToken] = useState<{
    token: string;
    keyName: string;
  } | null>(null);
  const [keyName, setKeyName] = useState("default");
  const createKey = useMutation({
    mutationFn: () => api.createServiceAccountKey(id, keyName),
    onSuccess: (r) => {
      setRevealToken({ token: r.token, keyName });
      setKeyName("default");
      q.invalidateQueries({ queryKey: ["service-account-keys", id] });
    },
    onError: (e) => toast.error(e instanceof Error ? e.message : "Failed"),
  });
  const revokeKey = useMutation({
    mutationFn: (keyID: string) => api.revokeServiceAccountKey(id, keyID),
    onSuccess: () => {
      toast.success("Key revoked.");
      q.invalidateQueries({ queryKey: ["service-account-keys", id] });
    },
  });
  const toggle = useMutation({
    mutationFn: () => api.updateServiceAccount(id, name, !disabled),
    onSuccess: () => {
      toast.success(disabled ? "Enabled." : "Disabled.");
      q.invalidateQueries({ queryKey: ["service-accounts"] });
    },
  });

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader className="flex flex-row items-start justify-between">
          <div>
            <CardTitle className="flex items-center gap-2">
              <Bot className="h-4 w-4" /> {name}
            </CardTitle>
            <div className="mt-1 font-mono text-xs text-[var(--color-fg-muted)]">
              {id}
            </div>
          </div>
          <AlertDialog>
            <AlertDialogTrigger asChild>
              <Button variant="outline" size="sm">
                <Power className="h-3.5 w-3.5" />
                {disabled ? "Enable" : "Disable"}
              </Button>
            </AlertDialogTrigger>
            <AlertDialogContent>
              <AlertDialogTitle>
                {disabled ? "Enable" : "Disable"} “{name}”?
              </AlertDialogTitle>
              <AlertDialogDescription>
                {disabled
                  ? "Agents using active keys will be able to authenticate again."
                  : "All agents using this service account will immediately lose access. Keys and grants are retained."}
              </AlertDialogDescription>
              <AlertDialogFooter>
                <AlertDialogCancel>Cancel</AlertDialogCancel>
                <AlertDialogAction onClick={() => toggle.mutate()}>
                  {disabled ? "Enable" : "Disable"}
                </AlertDialogAction>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
        </CardHeader>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-sm">
            <KeyRound className="h-4 w-4" /> API keys
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          <form
            className="flex gap-2"
            onSubmit={(e) => {
              e.preventDefault();
              createKey.mutate();
            }}
          >
            <Input
              value={keyName}
              onChange={(e) => setKeyName(e.target.value)}
              placeholder="Key name"
            />
            <Button type="submit" disabled={!keyName || createKey.isPending}>
              <Plus className="h-4 w-4" /> New key
            </Button>
          </form>
          <div className="space-y-1">
            {keys.data?.items?.length ? (
              keys.data.items.map((k) => (
                <div
                  key={k.id}
                  className="flex items-center gap-3 rounded-md border border-[var(--color-border)] bg-[var(--color-surface)] px-3 py-2 text-sm"
                >
                  <KeyRound className="h-3.5 w-3.5 text-[var(--color-fg-muted)]" />
                  <span className="font-medium">{k.name}</span>
                  <span className="flex-1 truncate font-mono text-xs text-[var(--color-fg-muted)]">
                    {k.id}
                  </span>
                  <AlertDialog>
                    <AlertDialogTrigger asChild>
                      <Button variant="ghost" size="icon" aria-label="Revoke">
                        <Trash2 className="h-3.5 w-3.5 text-[var(--color-danger)]" />
                      </Button>
                    </AlertDialogTrigger>
                    <AlertDialogContent>
                      <AlertDialogTitle>Revoke key “{k.name}”?</AlertDialogTitle>
                      <AlertDialogDescription>
                        Agents using this key will immediately lose access.
                      </AlertDialogDescription>
                      <AlertDialogFooter>
                        <AlertDialogCancel>Cancel</AlertDialogCancel>
                        <AlertDialogAction
                          variant="danger"
                          onClick={() => revokeKey.mutate(k.id)}
                        >
                          Revoke
                        </AlertDialogAction>
                      </AlertDialogFooter>
                    </AlertDialogContent>
                  </AlertDialog>
                </div>
              ))
            ) : (
              <div className="rounded-md border border-dashed border-[var(--color-border)] p-4 text-center text-xs text-[var(--color-fg-muted)]">
                No keys yet. Mint one to start using the CLI.
              </div>
            )}
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-sm">Headless agent quickstart</CardTitle>
        </CardHeader>
        <CardContent>
          <p className="mb-2 text-xs text-[var(--color-fg-muted)]">
            Use the minted key as a bearer token from any HTTP client, or
            with the CLI in non-interactive mode:
          </p>
          <CodeBlock>{`aidocs auth login --token <YOUR_KEY>
aidocs docs create report.html`}</CodeBlock>
          <p className="mt-3 text-xs text-[var(--color-fg-muted)]">
            For raw HTTP:
          </p>
          <CodeBlock>{`curl -H "Authorization: Bearer <YOUR_KEY>" \\
  ${typeof window !== "undefined" ? window.location.origin : "https://your-host"}/v1/documents`}</CodeBlock>
        </CardContent>
      </Card>

      <Dialog
        open={!!revealToken}
        onOpenChange={(o) => !o && setRevealToken(null)}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Copy this key now</DialogTitle>
            <DialogDescription>
              This is the only time the secret token will be shown. Store it
              somewhere safe.
            </DialogDescription>
          </DialogHeader>
          {revealToken && (
            <>
              <div className="mb-1 text-xs font-medium text-[var(--color-fg-muted)]">
                {revealToken.keyName}
              </div>
              <CodeBlock>{revealToken.token}</CodeBlock>
              <div className="mt-4 text-xs text-[var(--color-fg-muted)]">
                Use it as a bearer token from your headless agent, or:
              </div>
              <CodeBlock>{`aidocs auth login --token ${revealToken.token}`}</CodeBlock>
            </>
          )}
          <DialogFooter>
            <Button onClick={() => setRevealToken(null)}>Done</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
