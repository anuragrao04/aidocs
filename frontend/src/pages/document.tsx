import { useEffect, useMemo, useRef, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import {
  ArrowLeft,
  CheckCircle2,
  ExternalLink,
  MoreHorizontal,
  Share2,
  Trash2,
  Upload,
} from "lucide-react";
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
import { useNavigate } from "react-router-dom";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Center, CodeBlock, EmptyState, Skeleton } from "@/components/ui/misc";
import { Input, Textarea } from "@/components/ui/input";
import {
  Dropdown,
  DropdownContent,
  DropdownItem,
  DropdownSeparator,
  DropdownTrigger,
} from "@/components/ui/dropdown";
import {
  Sheet,
  SheetBody,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet";
import {
  api,
  type Comment,
  currentVersionID,
  docTitle,
  versionID,
} from "@/api";

export function DocumentPage() {
  const { id = "" } = useParams();
  const doc = useQuery({
    queryKey: ["document", id],
    queryFn: () => api.getDocument(id),
  });
  const versions = useQuery({
    queryKey: ["versions", id],
    queryFn: () => api.listVersions(id),
  });
  const [selection, setSelection] = useState("");
  const [activeComment, setActiveComment] = useState<string>();

  const current =
    currentVersionID(doc.data || {}) ||
    versionID(versions.data?.items?.[0] || {});

  if (doc.isLoading) {
    return (
      <div className="p-6">
        <Skeleton className="mb-3 h-7 w-72" />
        <Skeleton className="h-[60vh] w-full" />
      </div>
    );
  }
  if (doc.error) return <Center>Could not load document.</Center>;
  if (!doc.data) return null;

  return (
    <div className="flex h-full flex-col">
      <DocBar
        title={docTitle(doc.data)}
        docId={id}
        current={current}
      />
      <div className="flex flex-1 overflow-hidden">
        <section className="relative flex-1 bg-[var(--color-surface-muted)]">
          {current ? (
            <RenderFrame
              version={current}
              doc={id}
              onSelect={setSelection}
              activeComment={activeComment}
            />
          ) : (
            <Center>No version available.</Center>
          )}
        </section>
        <aside className="hidden w-[380px] shrink-0 flex-col border-l border-[var(--color-border)] bg-[var(--color-surface)] md:flex">
          <Comments
            doc={id}
            version={current}
            selectedText={selection}
            clearSelection={() => setSelection("")}
            onHover={setActiveComment}
          />
        </aside>
      </div>
    </div>
  );
}

function DocBar({
  title,
  docId,
  current,
}: {
  title: string;
  docId: string;
  current: string;
}) {
  const nav = useNavigate();
  const q = useQueryClient();
  const del = useMutation({
    mutationFn: () => api.deleteDocument(docId),
    onSuccess: () => {
      toast.success("Document deleted.");
      q.invalidateQueries({ queryKey: ["documents"] });
      nav("/app/documents");
    },
    onError: (e) => toast.error(e instanceof Error ? e.message : "Delete failed"),
  });
  return (
    <div className="flex h-14 items-center gap-3 border-b border-[var(--color-border)] bg-[var(--color-surface)] px-4">
      <Button asChild variant="ghost" size="icon">
        <Link to="/app/documents" aria-label="Back to documents">
          <ArrowLeft className="h-4 w-4" />
        </Link>
      </Button>
      <div className="min-w-0 flex-1">
        <div className="truncate text-sm font-semibold">{title}</div>
        <div className="truncate font-mono text-[11px] text-[var(--color-fg-muted)]">
          {docId}
        </div>
      </div>
      <Sheet>
        <SheetTrigger asChild>
          <Button variant="outline" size="sm">
            <Share2 className="h-4 w-4" /> Share
          </Button>
        </SheetTrigger>
        <ShareSheet docId={docId} />
      </Sheet>
      <Dropdown>
        <DropdownTrigger asChild>
          <Button variant="ghost" size="icon" aria-label="More">
            <MoreHorizontal className="h-4 w-4" />
          </Button>
        </DropdownTrigger>
        <DropdownContent>
          <DropdownItem
            onSelect={() =>
              window.open(`/v1/documents/${docId}/html`, "_blank")
            }
          >
            <ExternalLink className="h-4 w-4" /> Open raw HTML
          </DropdownItem>
          <UploadVersionItem docId={docId} baseVersion={current} />
          <DropdownSeparator />
          <AlertDialog>
            <AlertDialogTrigger asChild>
              <DropdownItem
                onSelect={(e) => e.preventDefault()}
                className="text-[var(--color-danger)] data-[highlighted]:bg-red-50 dark:data-[highlighted]:bg-red-950/40"
              >
                <Trash2 className="h-4 w-4" /> Delete document
              </DropdownItem>
            </AlertDialogTrigger>
            <AlertDialogContent>
              <AlertDialogTitle>Delete “{title}”?</AlertDialogTitle>
              <AlertDialogDescription>
                This permanently removes the document, all versions, and
                all comments. This cannot be undone.
              </AlertDialogDescription>
              <AlertDialogFooter>
                <AlertDialogCancel>Cancel</AlertDialogCancel>
                <AlertDialogAction
                  variant="danger"
                  onClick={() => del.mutate()}
                >
                  Delete
                </AlertDialogAction>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
        </DropdownContent>
      </Dropdown>
    </div>
  );
}

function UploadVersionItem({
  docId,
  baseVersion,
}: {
  docId: string;
  baseVersion: string;
}) {
  const q = useQueryClient();
  const [summary, setSummary] = useState("");
  const [file, setFile] = useState<File | null>(null);
  const [open, setOpen] = useState(false);
  const m = useMutation({
    mutationFn: () =>
      api.createVersion(docId, baseVersion, summary, file!),
    onSuccess: () => {
      toast.success("Version uploaded.");
      setOpen(false);
      setSummary("");
      setFile(null);
      q.invalidateQueries({ queryKey: ["versions", docId] });
      q.invalidateQueries({ queryKey: ["document", docId] });
      q.invalidateQueries({ queryKey: ["comments", docId] });
    },
    onError: (e) => toast.error(e instanceof Error ? e.message : "Upload failed"),
  });
  return (
    <Sheet open={open} onOpenChange={setOpen}>
      <SheetTrigger asChild>
        <DropdownItem
          onSelect={(e) => {
            e.preventDefault();
            setOpen(true);
          }}
          className="text-[var(--color-fg-muted)]"
        >
          <Upload className="h-3.5 w-3.5" /> Upload new version (backup)
        </DropdownItem>
      </SheetTrigger>
      <SheetContent>
        <SheetHeader>
          <SheetTitle>Upload new version</SheetTitle>
        </SheetHeader>
        <SheetBody>
          <form
            className="space-y-3"
            onSubmit={(e) => {
              e.preventDefault();
              if (file) m.mutate();
            }}
          >
            <Input
              placeholder="Change summary"
              value={summary}
              onChange={(e) => setSummary(e.target.value)}
            />
            <input
              type="file"
              accept=".html,text/html"
              onChange={(e) => setFile(e.target.files?.[0] || null)}
              className="text-sm"
            />
            <Button type="submit" disabled={!file || m.isPending}>
              {m.isPending ? "Uploading…" : "Upload version"}
            </Button>
          </form>
        </SheetBody>
      </SheetContent>
    </Sheet>
  );
}

function RenderFrame({
  version,
  doc,
  onSelect,
  activeComment,
}: {
  version: string;
  doc: string;
  onSelect: (q: string) => void;
  activeComment?: string;
}) {
  const ref = useRef<HTMLIFrameElement>(null);
  const token = useQuery({
    queryKey: ["render", version],
    queryFn: () => api.renderToken(version),
    staleTime: 4 * 60_000,
  });
  const comments = useQuery({
    queryKey: ["comments", doc, version],
    queryFn: () => api.listComments(doc, version),
    enabled: !!version,
  });
  const paintPayload = useMemo(
    () =>
      comments.data?.items?.map((c) => ({
        id: c.id,
        quote: c.selected_text || c.anchor?.quote,
      })) || [],
    [comments.data],
  );
  const paintRef = useRef({ comments: paintPayload, active: activeComment });
  paintRef.current = { comments: paintPayload, active: activeComment };
  useEffect(() => {
    const onMsg = (e: MessageEvent) => {
      if (e.data?.type === "aidocs:selection") onSelect(e.data.quote);
      if (e.data?.type === "aidocs:ready") {
        ref.current?.contentWindow?.postMessage(
          {
            type: "aidocs:paint",
            comments: paintRef.current.comments,
            active: paintRef.current.active,
          },
          "*",
        );
      }
    };
    window.addEventListener("message", onMsg);
    return () => window.removeEventListener("message", onMsg);
  }, [onSelect]);
  useEffect(() => {
    ref.current?.contentWindow?.postMessage(
      { type: "aidocs:paint", comments: paintPayload, active: activeComment },
      "*",
    );
  }, [paintPayload, activeComment, token.data]);
  if (token.isLoading) return <Center>Preparing secure render…</Center>;
  if (token.error) return <Center>Could not create render token.</Center>;
  return (
    <iframe
      ref={ref}
      className="h-full w-full bg-white"
      src={token.data!.url}
      sandbox="allow-scripts allow-same-origin"
      onLoad={() =>
        ref.current?.contentWindow?.postMessage(
          {
            type: "aidocs:paint",
            comments: paintPayload,
            active: activeComment,
          },
          "*",
        )
      }
    />
  );
}

function Comments({
  doc,
  version,
  selectedText,
  clearSelection,
  onHover,
}: {
  doc: string;
  version: string;
  selectedText: string;
  clearSelection: () => void;
  onHover: (id?: string) => void;
}) {
  const q = useQueryClient();
  const [filter, setFilter] = useState<"all" | "open" | "resolved">("open");
  const [quote, setQuote] = useState("");
  const [body, setBody] = useState("");
  useEffect(() => {
    if (selectedText) setQuote(selectedText);
  }, [selectedText]);
  const comments = useQuery({
    queryKey: ["comments", doc, "all"],
    queryFn: () => api.listComments(doc),
    enabled: !!doc,
  });
  const create = useMutation({
    mutationFn: () => api.createComment(doc, version, body, quote),
    onSuccess: () => {
      setBody("");
      setQuote("");
      clearSelection();
      toast.success("Comment added.");
      q.invalidateQueries({ queryKey: ["comments", doc] });
    },
    onError: (e) =>
      toast.error(e instanceof Error ? e.message : "Could not add comment"),
  });
  const items = (comments.data?.items || []).filter((c) => {
    if (filter === "all") return true;
    if (filter === "resolved") return c.status === "resolved";
    return c.status !== "resolved";
  });
  return (
    <div className="flex h-full flex-col">
      <div className="border-b border-[var(--color-border)] px-4 py-3">
        <div className="mb-3 flex items-center justify-between">
          <h2 className="text-sm font-semibold">Comments</h2>
          <Badge variant="muted">{comments.data?.items?.length || 0}</Badge>
        </div>
        <div className="flex gap-1 rounded-md bg-[var(--color-surface-muted)] p-0.5 text-xs">
          {(["all", "open", "resolved"] as const).map((f) => (
            <button
              key={f}
              onClick={() => setFilter(f)}
              className={`flex-1 rounded px-2 py-1 capitalize transition-colors ${filter === f ? "bg-[var(--color-surface)] text-[var(--color-fg)] shadow-sm" : "text-[var(--color-fg-muted)]"}`}
            >
              {f}
            </button>
          ))}
        </div>
      </div>
      <div className="flex-1 overflow-y-auto p-4">
        {quote && (
          <form
            className="mb-4 space-y-2 rounded-[12px] border border-[var(--color-border)] bg-[var(--color-surface-muted)]/50 p-3"
            onSubmit={(e) => {
              e.preventDefault();
              create.mutate();
            }}
          >
            <div className="border-l-2 border-[var(--color-accent)] pl-2 text-xs italic text-[var(--color-fg-muted)]">
              “{quote.length > 80 ? quote.slice(0, 80) + "…" : quote}”
            </div>
            <Textarea
              autoFocus
              rows={3}
              value={body}
              onChange={(e) => setBody(e.target.value)}
              placeholder="Add a comment…"
            />
            <div className="flex justify-end gap-2">
              <Button
                type="button"
                variant="ghost"
                size="sm"
                onClick={() => {
                  setQuote("");
                  setBody("");
                  clearSelection();
                }}
              >
                Cancel
              </Button>
              <Button
                size="sm"
                type="submit"
                disabled={!body || create.isPending}
              >
                Comment
              </Button>
            </div>
          </form>
        )}
        {items.length === 0 ? (
          <EmptyState
            title={filter === "open" ? "No open comments" : "Nothing here"}
            description="Select text inside the rendered document to start a thread."
          />
        ) : (
          <div className="space-y-3">
            {items.map((c) => (
              <CommentCard
                key={c.id}
                c={c}
                doc={doc}
                onHover={onHover}
              />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

function CommentCard({
  c,
  doc,
  onHover,
}: {
  c: Comment;
  doc: string;
  onHover: (id?: string) => void;
}) {
  const q = useQueryClient();
  const m = useMutation({
    mutationFn: (status: string) => api.patchComment(doc, c.id, c.body, status),
    onSuccess: () => q.invalidateQueries({ queryKey: ["comments", doc] }),
  });
  const resolved = c.status === "resolved";
  return (
    <article
      className={`rounded-[12px] border border-[var(--color-border)] bg-[var(--color-surface)] p-3 transition-colors hover:border-[var(--color-border-strong)] ${resolved ? "opacity-70" : ""}`}
      onMouseEnter={() => onHover(c.id)}
      onMouseLeave={() => onHover(undefined)}
    >
      <div className="mb-1 flex items-center justify-between">
        <span className="text-xs font-semibold">
          {c.author.name || c.author.email || c.author.id}
        </span>
        {resolved ? (
          <Badge variant="success">
            <CheckCircle2 className="h-3 w-3" /> resolved
          </Badge>
        ) : (
          <Badge variant="accent">open</Badge>
        )}
      </div>
      {c.current_placement?.status &&
        c.current_placement.status !== "attached" && (
          <Badge variant="warning" className="mb-2">
            {c.current_placement.status}
          </Badge>
        )}
      <blockquote className="mb-2 border-l-2 border-[var(--color-border-strong)] pl-2 text-xs italic text-[var(--color-fg-muted)]">
        {c.selected_text}
      </blockquote>
      <p className="text-sm">{c.body}</p>
      <div className="mt-2 flex justify-end">
        <Button
          variant="ghost"
          size="sm"
          onClick={() => m.mutate(resolved ? "open" : "resolved")}
        >
          {resolved ? "Reopen" : "Resolve"}
        </Button>
      </div>
    </article>
  );
}

function ShareSheet({ docId }: { docId: string }) {
  const q = useQueryClient();
  const grants = useQuery({
    queryKey: ["grants", docId],
    queryFn: () => api.listGrants(docId),
  });
  const [address, setAddress] = useState("");
  const [role, setRole] = useState("commenter");
  const m = useMutation({
    mutationFn: () => api.createGrant(docId, address.trim(), role),
    onSuccess: () => {
      setAddress("");
      toast.success("Access granted.");
      q.invalidateQueries({ queryKey: ["grants", docId] });
    },
    onError: (e) =>
      toast.error(e instanceof Error ? e.message : "Couldn't share."),
  });
  return (
    <SheetContent>
      <SheetHeader>
        <SheetTitle>Share document</SheetTitle>
      </SheetHeader>
      <SheetBody>
        <form
          className="space-y-3"
          onSubmit={(e) => {
            e.preventDefault();
            m.mutate();
          }}
        >
          <div>
            <Input
              placeholder="anurag@razorpay.com  or  n8n-prod@brave.otter.bot"
              value={address}
              onChange={(e) => setAddress(e.target.value)}
              autoCapitalize="off"
              autoCorrect="off"
              spellCheck={false}
            />
            <p className="mt-1.5 text-[11px] text-[var(--color-fg-muted)]">
              Add a person by their email, or a bot by its address.
            </p>
          </div>
          <select
            value={role}
            onChange={(e) => setRole(e.target.value)}
            className="h-9 w-full rounded-[10px] border border-[var(--color-border)] bg-[var(--color-surface)] px-3 text-sm"
          >
            <option value="viewer">Viewer</option>
            <option value="commenter">Commenter</option>
            <option value="editor">Editor</option>
          </select>
          <Button
            className="w-full"
            type="submit"
            disabled={!address.trim() || m.isPending}
          >
            Share
          </Button>
        </form>
        <div className="mt-6">
          <h4 className="mb-2 text-xs font-medium uppercase tracking-wider text-[var(--color-fg-muted)]">
            People with access
          </h4>
          <div className="space-y-1">
            {grants.data?.items?.map((g: any) => (
              <div
                key={g.id || g.ID}
                className="flex items-center justify-between rounded-md border border-[var(--color-border)] bg-[var(--color-surface)] px-3 py-2 text-sm"
              >
                <span className="truncate">
                  {g.principal?.email ||
                    g.Principal?.Email ||
                    g.principal?.id ||
                    g.Principal?.ID}
                </span>
                <Badge variant="muted">{g.role || g.Role}</Badge>
              </div>
            ))}
            {(!grants.data?.items || grants.data.items.length === 0) && (
              <div className="text-xs text-[var(--color-fg-muted)]">
                No grants yet.
              </div>
            )}
          </div>
        </div>
      </SheetBody>
    </SheetContent>
  );
}
