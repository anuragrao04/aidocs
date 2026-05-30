import { useEffect, useMemo, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { useOnboarding } from "@/lib/onboarding";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import {
  FileText,
  MoreHorizontal,
  Search,
  Sparkles,
  Trash2,
  Upload,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Center, Skeleton, EmptyState } from "@/components/ui/misc";
import {
  Dropdown,
  DropdownContent,
  DropdownItem,
  DropdownTrigger,
} from "@/components/ui/dropdown";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { HtmlFileInput, useStagedFile } from "@/components/ui/upload";
import { DeleteDocumentDialog } from "@/components/delete-document-dialog";
import { api, docID, docTitle, type Document } from "@/api";
import { queryKeys } from "@/lib/queryKeys";
import { VISIBILITIES, DEFAULT_VISIBILITY } from "@/lib/constants";

export function DocumentsPage() {
  const docs = useQuery({
    queryKey: queryKeys.documents(),
    queryFn: api.listDocuments,
  });
  const [q, setQ] = useState("");
  const { state: onb } = useOnboarding();
  const nav = useNavigate();
  useEffect(() => {
    // First-time visitor with no documents → send to setup guide.
    if (
      onb.status === "active" &&
      !onb.steps.cli_installed &&
      !onb.steps.cli_authed &&
      !onb.steps.agent_configured &&
      docs.data &&
      (docs.data.items || []).length === 0
    ) {
      nav("/app/start", { replace: true });
    }
  }, [onb, docs.data, nav]);
  const items = useMemo(() => {
    const list = docs.data?.items || [];
    if (!q) return list;
    return list.filter((d) =>
      docTitle(d).toLowerCase().includes(q.toLowerCase()),
    );
  }, [docs.data, q]);

  if (docs.isLoading) {
    return (
      <div className="mx-auto max-w-6xl px-6 py-10">
        <Skeleton className="mb-3 h-9 w-48" />
        <Skeleton className="h-64 w-full" />
      </div>
    );
  }
  if (docs.error) return <Center>Could not load documents.</Center>;

  const empty = (docs.data?.items || []).length === 0;

  return (
    <div className="mx-auto max-w-6xl px-6 py-10">
      <div className="mb-6 flex items-end justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Documents</h1>
          <p className="mt-1 text-sm text-[var(--color-fg-muted)]">
            Documents your agents have published.
          </p>
        </div>
        <div className="flex items-center gap-2">
          <div className="relative">
            <Search className="pointer-events-none absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-[var(--color-fg-muted)]" />
            <Input
              value={q}
              onChange={(e) => setQ(e.target.value)}
              placeholder="Search…"
              className="w-56 pl-8"
            />
          </div>
          <Dropdown>
            <DropdownTrigger asChild>
              <Button variant="outline" size="icon" aria-label="More">
                <MoreHorizontal className="h-4 w-4" />
              </Button>
            </DropdownTrigger>
            <DropdownContent>
              <UploadBackupItem />
            </DropdownContent>
          </Dropdown>
        </div>
      </div>

      {empty ? (
        <EmptyDocs />
      ) : items.length === 0 ? (
        <EmptyState title="No matches" description={`Nothing matches “${q}”.`} />
      ) : (
        <DocTable items={items} />
      )}
    </div>
  );
}

function DocTable({ items }: { items: Document[] }) {
  return (
    <div className="overflow-hidden rounded-[12px] border border-[var(--color-border)] bg-[var(--color-surface)]">
      <table className="w-full text-sm">
        <thead className="bg-[var(--color-surface-muted)]/50 text-[11px] uppercase tracking-wider text-[var(--color-fg-muted)]">
          <tr>
            <th className="px-4 py-2.5 text-left font-medium">Title</th>
            <th className="px-4 py-2.5 text-left font-medium">ID</th>
            <th className="px-4 py-2.5 text-left font-medium">Visibility</th>
            <th className="w-12 px-2 py-2.5"></th>
          </tr>
        </thead>
        <tbody>
          {items.map((d) => (
            <DocRow key={docID(d)} doc={d} />
          ))}
        </tbody>
      </table>
    </div>
  );
}

function DocRow({ doc }: { doc: Document }) {
  const q = useQueryClient();
  const del = useMutation({
    mutationFn: () => api.deleteDocument(docID(doc)),
    onSuccess: () => {
      toast.success("Deleted.");
      q.invalidateQueries({ queryKey: queryKeys.documents() });
    },
    onError: (e) =>
      toast.error(e instanceof Error ? e.message : "Delete failed"),
  });
  return (
    <tr className="group border-t border-[var(--color-border)] transition-colors hover:bg-[var(--color-surface-muted)]/40">
      <td className="px-4 py-3">
        <Link
          to={`/app/d/${docID(doc)}`}
          className="flex items-center gap-2 font-medium hover:underline"
        >
          <FileText className="h-4 w-4 text-[var(--color-fg-muted)]" />
          {docTitle(doc)}
        </Link>
      </td>
      <td className="px-4 py-3 font-mono text-xs text-[var(--color-fg-muted)]">
        {docID(doc)}
      </td>
      <td className="px-4 py-3">
        <Badge variant="muted">
          {(doc.visibility || DEFAULT_VISIBILITY).toLowerCase()}
        </Badge>
      </td>
      <td className="px-2 py-3">
        <DeleteDocumentDialog
          title={docTitle(doc)}
          onConfirm={() => del.mutate()}
          trigger={
            <Button
              variant="ghost"
              size="icon"
              className="opacity-0 transition-opacity group-hover:opacity-100"
              aria-label="Delete document"
            >
              <Trash2 className="h-3.5 w-3.5 text-[var(--color-danger)]" />
            </Button>
          }
        />
      </td>
    </tr>
  );
}

function EmptyDocs() {
  return (
    <div className="mx-auto max-w-xl space-y-4">
      <div className="rounded-[14px] border border-[var(--color-border)] bg-[var(--color-surface)] p-6">
        <div className="mb-3 flex h-10 w-10 items-center justify-center rounded-md bg-[var(--color-accent-muted)] text-[var(--color-accent)]">
          <Sparkles className="h-5 w-5" />
        </div>
        <h3 className="text-base font-semibold">Set up your agent</h3>
        <p className="mt-1 text-sm text-[var(--color-fg-muted)]">
          Documents are published by your AI agent. Walk through the
          three-step setup so it knows how to use aidocs.
        </p>
        <div className="mt-4">
          <Button asChild>
            <Link to="/app/start">Open setup guide</Link>
          </Button>
        </div>
      </div>
      <UploadBackupCard inline />
    </div>
  );
}

function UploadBackupItem() {
  return (
    <UploadBackupDialog
      trigger={
        <DropdownItem onSelect={(e) => e.preventDefault()}>
          <Upload className="h-4 w-4" /> Upload HTML (backup)
        </DropdownItem>
      }
    />
  );
}

function UploadBackupCard({ inline }: { inline?: boolean }) {
  return (
    <div
      className={`rounded-[14px] border border-dashed border-[var(--color-border)] p-4 text-center text-sm text-[var(--color-fg-muted)] ${inline ? "" : ""}`}
    >
      Manual fallback —{" "}
      <UploadBackupDialog
        trigger={
          <button className="font-medium text-[var(--color-fg)] underline-offset-2 hover:underline">
            upload an HTML file manually
          </button>
        }
      />
      .
    </div>
  );
}

function UploadBackupDialog({ trigger }: { trigger: React.ReactNode }) {
  const q = useQueryClient();
  const nav = useNavigate();
  const [title, setTitle] = useState("");
  const [visibility, setVisibility] = useState(DEFAULT_VISIBILITY);
  const { file, setFile, reset } = useStagedFile();
  const [open, setOpen] = useState(false);
  const m = useMutation({
    mutationFn: () =>
      api.createDocument(title || file?.name || "Untitled", visibility, file!),
    onSuccess: (r) => {
      q.invalidateQueries({ queryKey: queryKeys.documents() });
      toast.success("Document uploaded.");
      setOpen(false);
      reset();
      nav(`/app/d/${r.id}`);
    },
    onError: (e) =>
      toast.error(e instanceof Error ? e.message : "Upload failed"),
  });
  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>{trigger}</DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Upload HTML (backup)</DialogTitle>
          <DialogDescription>
            Agents normally publish via the CLI. Use this only as a manual
            fallback.
          </DialogDescription>
        </DialogHeader>
        <form
          className="space-y-3"
          onSubmit={(e) => {
            e.preventDefault();
            if (file) m.mutate();
          }}
        >
          <Input
            placeholder="Title"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
          />
          <select
            value={visibility}
            onChange={(e) => setVisibility(e.target.value)}
            aria-label="Document visibility"
            className="h-9 w-full rounded-[10px] border border-[var(--color-border)] bg-[var(--color-surface)] px-3 text-sm"
          >
            {VISIBILITIES.map((v) => (
              <option key={v.value} value={v.value}>
                {v.label}
              </option>
            ))}
          </select>
          <HtmlFileInput onFile={setFile} />
          <DialogFooter>
            <Button
              type="button"
              variant="ghost"
              onClick={() => setOpen(false)}
            >
              Cancel
            </Button>
            <Button type="submit" disabled={!file || m.isPending}>
              {m.isPending ? "Uploading…" : "Upload"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
