import React, { useEffect, useMemo, useRef, useState } from "react";
import { createRoot } from "react-dom/client";
import {
  BrowserRouter,
  Link,
  Navigate,
  Route,
  Routes,
  useNavigate,
  useParams,
} from "react-router-dom";
import {
  QueryClient,
  QueryClientProvider,
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import {
  Bot,
  FileText,
  KeyRound,
  MessageSquare,
  Plus,
  Share2,
  Upload,
  Sparkles,
  ExternalLink,
} from "lucide-react";
import {
  api,
  APIError,
  currentVersionID,
  docID,
  docTitle,
  versionID,
  versionNumber,
  type Comment,
  type Document,
} from "./api";
import "./styles.css";

const qc = new QueryClient();

function App() {
  return (
    <QueryClientProvider client={qc}>
      <BrowserRouter>
        <Shell />
      </BrowserRouter>
    </QueryClientProvider>
  );
}

function Shell() {
  const me = useQuery({ queryKey: ["me"], queryFn: api.me, retry: false });
  const unauth = me.error instanceof APIError && me.error.status === 401;
  return (
    <div className="app">
      <header className="topbar">
        <Link to="/" className="brand">
          <Sparkles size={22} /> aidocs
        </Link>
        <nav>
          <a href="/openapi.json">OpenAPI</a>
          <a href="/api-docs">API docs</a>
          {me.data && <UserMenu me={me.data} />}
        </nav>
      </header>
      {unauth ? (
        <Login />
      ) : me.isLoading ? (
        <Center text="Loading workspace…" />
      ) : (
        <Routes>
          <Route path="/" element={<Home />} />
          <Route path="/service-accounts" element={<ServiceAccounts />} />
          <Route path="/documents/:id" element={<DocumentPage />} />
          <Route path="*" element={<Navigate to="/" />} />
        </Routes>
      )}
    </div>
  );
}

function UserMenu({ me }: { me: Awaited<ReturnType<typeof api.me>> }) {
  const [open, setOpen] = useState(false);
  const user = me.user || me.principal;
  const label = user.name || user.email || user.id;
  const initials = label
    .split(/\s+/)
    .map((x) => x[0])
    .join("")
    .slice(0, 2)
    .toUpperCase();
  return (
    <div className="userMenu">
      <button className="avatarButton" onClick={() => setOpen((v) => !v)}>
        {user.picture_url ? <img src={user.picture_url} alt="" /> : initials}
      </button>
      {open && (
        <div className="userPopover">
          <div className="userHeader">
            <div className="avatarLarge">
              {user.picture_url ? (
                <img src={user.picture_url} alt="" />
              ) : (
                initials
              )}
            </div>
            <div>
              <b>{user.name || "Signed-in user"}</b>
              <p>{user.email || user.id}</p>
            </div>
          </div>
          <Link
            className="popoverAction"
            to="/service-accounts"
            onClick={() => setOpen(false)}
          >
            <Bot size={16} /> Manage service accounts
          </Link>
        </div>
      )}
    </div>
  );
}

function Login() {
  return (
    <main className="login">
      <div className="hero">
        <div className="pill">HTML review for AI agents</div>
        <h1>Review generated HTML like a doc.</h1>
        <p>
          Upload a self-contained HTML file, render it safely, and keep review
          comments anchored to the exact text they refer to.
        </p>
        <a className="primary" href={api.loginURL()}>
          Sign in with Google
        </a>
      </div>
    </main>
  );
}
function Center({ text }: { text: string }) {
  return <main className="center">{text}</main>;
}

function Home() {
  const docs = useQuery({
    queryKey: ["documents"],
    queryFn: api.listDocuments,
  });
  return (
    <main className="page">
      <section className="pageHead">
        <div>
          <h1>Documents</h1>
          <p>Upload and review single-file HTML documents.</p>
        </div>
        <UploadCard />
      </section>
      {docs.isLoading ? (
        <Center text="Loading documents…" />
      ) : (
        <div className="grid">
          {docs.data?.items?.map((d) => (
            <DocCard key={docID(d)} doc={d} />
          ))}
          {!docs.data?.items?.length && <Empty />}
        </div>
      )}
    </main>
  );
}

function UploadCard() {
  const nav = useNavigate();
  const q = useQueryClient();
  const [title, setTitle] = useState("");
  const [visibility, setVisibility] = useState("private");
  const [file, setFile] = useState<File | null>(null);
  const m = useMutation({
    mutationFn: () =>
      api.createDocument(title || file?.name || "Untitled", visibility, file!),
    onSuccess: (r) => {
      q.invalidateQueries({ queryKey: ["documents"] });
      nav(`/documents/${r.id}`);
    },
  });
  return (
    <form
      className="uploadCard"
      onSubmit={(e) => {
        e.preventDefault();
        if (file) m.mutate();
      }}
    >
      <h2>
        <Plus size={18} /> New document
      </h2>
      <input
        placeholder="Title"
        value={title}
        onChange={(e) => setTitle(e.target.value)}
      />
      <select
        value={visibility}
        onChange={(e) => setVisibility(e.target.value)}
      >
        <option value="private">Private</option>
        <option value="org">Org visible</option>
        <option value="link">Anyone with link</option>
      </select>
      <input
        type="file"
        accept=".html,text/html"
        onChange={(e) => setFile(e.target.files?.[0] || null)}
      />
      <button disabled={!file || m.isPending}>
        {m.isPending ? "Uploading…" : "Upload HTML"}
      </button>
      {m.error && <ErrorText err={m.error} />}
    </form>
  );
}

function DocCard({ doc }: { doc: Document }) {
  return (
    <Link className="docCard" to={`/documents/${docID(doc)}`}>
      <FileText />
      <h3>{docTitle(doc)}</h3>
      <p>{docID(doc)}</p>
      <span>{doc.visibility || doc.Visibility || "private"}</span>
    </Link>
  );
}
function Empty() {
  return (
    <div className="empty">
      <FileText size={36} />
      <h2>No documents yet</h2>
      <p>Upload an HTML file to start reviewing.</p>
    </div>
  );
}

function DocumentPage() {
  const { id = "" } = useParams();
  const [tab, setTab] = useState<"comments" | "versions" | "share">("comments");
  const [selection, setSelection] = useState("");
  const [activeComment, setActiveComment] = useState<string>();
  const doc = useQuery({
    queryKey: ["document", id],
    queryFn: () => api.getDocument(id),
  });
  const versions = useQuery({
    queryKey: ["versions", id],
    queryFn: () => api.listVersions(id),
  });
  const selected =
    currentVersionID(doc.data || {}) ||
    versionID(versions.data?.items?.[0] || {});
  if (doc.isLoading) return <Center text="Loading document…" />;
  if (doc.error) return <Center text="Could not load document." />;
  return (
    <main className="review">
      <aside className="side">
        <Link to="/" className="back">
          ← Documents
        </Link>
        <h1>{docTitle(doc.data!)}</h1>
        <p className="muted">{id}</p>
        <div className="hint">
          Select text inside the render to start a pinned comment.
        </div>
        <div className="tabs">
          <button
            className={tab === "comments" ? "active" : ""}
            onClick={() => setTab("comments")}
          >
            <MessageSquare size={16} />
            Comments
          </button>
          <button
            className={tab === "versions" ? "active" : ""}
            onClick={() => setTab("versions")}
          >
            <Upload size={16} />
            Versions
          </button>
          <button
            className={tab === "share" ? "active" : ""}
            onClick={() => setTab("share")}
          >
            <Share2 size={16} />
            Share
          </button>
        </div>
        {tab === "comments" && (
          <Comments
            doc={id}
            version={selected}
            selectedText={selection}
            onHover={setActiveComment}
          />
        )}
        {tab === "versions" && <Versions doc={id} current={selected} />}
        {tab === "share" && <Share doc={id} />}
      </aside>
      <section className="canvas">
        {selected ? (
          <RenderFrame
            version={selected}
            doc={id}
            onSelect={(q) => {
              setSelection(q);
              setTab("comments");
            }}
            activeComment={activeComment}
          />
        ) : (
          <Center text="No version available." />
        )}
      </section>
    </main>
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
  onSelect: (quote: string) => void;
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
  useEffect(() => {
    const onMsg = (e: MessageEvent) => {
      if (e.data?.type === "aidocs:selection") onSelect(e.data.quote);
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
  if (token.isLoading) return <Center text="Preparing secure render…" />;
  if (token.error) return <Center text="Could not create render token." />;
  return (
    <>
      <div className="frameBar">
        <span>
          Sandboxed render · {comments.data?.items?.length || 0} highlights
        </span>
        <a href={token.data!.url} target="_blank">
          <ExternalLink size={14} /> Open
        </a>
      </div>
      <iframe
        ref={ref}
        className="docFrame"
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
    </>
  );
}

function Comments({
  doc,
  version,
  selectedText,
  onHover,
}: {
  doc: string;
  version: string;
  selectedText: string;
  onHover: (id?: string) => void;
}) {
  const q = useQueryClient();
  const [quote, setQuote] = useState("");
  const [body, setBody] = useState("");
  useEffect(() => {
    if (selectedText) setQuote(selectedText);
  }, [selectedText]);
  const comments = useQuery({
    queryKey: ["comments", doc, version],
    queryFn: () => api.listComments(doc, version),
    enabled: !!version,
  });
  const create = useMutation({
    mutationFn: () => api.createComment(doc, version, body, quote),
    onSuccess: () => {
      setBody("");
      setQuote("");
      q.invalidateQueries({ queryKey: ["comments", doc, version] });
    },
  });
  return (
    <div className="panel">
      <form
        className="composer"
        onSubmit={(e) => {
          e.preventDefault();
          create.mutate();
        }}
      >
        <label>Selected text</label>
        <textarea
          rows={2}
          value={quote}
          onChange={(e) => setQuote(e.target.value)}
          placeholder="Paste or type the text you are commenting on"
        />
        <label>Comment</label>
        <textarea
          rows={3}
          value={body}
          onChange={(e) => setBody(e.target.value)}
          placeholder="What should change?"
        />
        <button disabled={!quote || !body || create.isPending}>
          Add comment
        </button>
        {create.error && <ErrorText err={create.error} />}
      </form>
      <div className="commentList">
        {comments.data?.items?.map((c) => (
          <CommentCard
            key={c.id}
            c={c}
            doc={doc}
            version={version}
            onHover={onHover}
          />
        ))}
      </div>
    </div>
  );
}
function CommentCard({
  c,
  doc,
  version,
  onHover,
}: {
  c: Comment;
  doc: string;
  version: string;
  onHover: (id?: string) => void;
}) {
  const q = useQueryClient();
  const m = useMutation({
    mutationFn: (status: string) => api.patchComment(doc, c.id, c.body, status),
    onSuccess: () =>
      q.invalidateQueries({ queryKey: ["comments", doc, version] }),
  });
  return (
    <article
      className="comment"
      onMouseEnter={() => onHover(c.id)}
      onMouseLeave={() => onHover(undefined)}
    >
      <div>
        <b>{c.author.name || c.author.email || c.author.id}</b>
        <span>{c.status}</span>
      </div>
      <blockquote>{c.selected_text}</blockquote>
      <p>{c.body}</p>
      <button
        onClick={() => m.mutate(c.status === "resolved" ? "open" : "resolved")}
      >
        {c.status === "resolved" ? "Reopen" : "Resolve"}
      </button>
    </article>
  );
}

function Versions({ doc, current }: { doc: string; current: string }) {
  const q = useQueryClient();
  const versions = useQuery({
    queryKey: ["versions", doc],
    queryFn: () => api.listVersions(doc),
  });
  const [summary, setSummary] = useState("");
  const [file, setFile] = useState<File | null>(null);
  const m = useMutation({
    mutationFn: () => api.createVersion(doc, current, summary, file!),
    onSuccess: () => {
      setSummary("");
      setFile(null);
      q.invalidateQueries({ queryKey: ["versions", doc] });
      q.invalidateQueries({ queryKey: ["document", doc] });
    },
  });
  return (
    <div className="panel">
      <form
        className="composer"
        onSubmit={(e) => {
          e.preventDefault();
          if (file) m.mutate();
        }}
      >
        <h3>Upload new version</h3>
        <input
          placeholder="Change summary"
          value={summary}
          onChange={(e) => setSummary(e.target.value)}
        />
        <input
          type="file"
          accept=".html,text/html"
          onChange={(e) => setFile(e.target.files?.[0] || null)}
        />
        <button disabled={!file || m.isPending}>Upload version</button>
        {m.error && <ErrorText err={m.error} />}
      </form>
      {versions.data?.items?.map((v) => (
        <div className="version" key={versionID(v)}>
          <b>Version {versionNumber(v)}</b>
          <span>{versionID(v)}</span>
          <p>{v.change_summary || v.ChangeSummary}</p>
        </div>
      ))}
    </div>
  );
}

function Share({ doc }: { doc: string }) {
  const q = useQueryClient();
  const grants = useQuery({
    queryKey: ["grants", doc],
    queryFn: () => api.listGrants(doc),
  });
  const sas = useQuery({
    queryKey: ["service-accounts"],
    queryFn: api.listServiceAccounts,
  });
  const [mode, setMode] = useState<"user" | "service_account">("user");
  const [email, setEmail] = useState("");
  const [sa, setSa] = useState("");
  const [role, setRole] = useState("commenter");
  const m = useMutation({
    mutationFn: () =>
      api.createGrant(
        doc,
        mode === "user"
          ? { type: "user", id: "", email }
          : { type: "service_account", id: sa },
        role,
      ),
    onSuccess: () => {
      setEmail("");
      q.invalidateQueries({ queryKey: ["grants", doc] });
    },
  });
  return (
    <div className="panel">
      <form
        className="composer"
        onSubmit={(e) => {
          e.preventDefault();
          m.mutate();
        }}
      >
        <h3>Grant access</h3>
        <select value={mode} onChange={(e) => setMode(e.target.value as any)}>
          <option value="user">User by email</option>
          <option value="service_account">Service account</option>
        </select>
        {mode === "user" ? (
          <input
            placeholder="user@example.com"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
          />
        ) : (
          <select value={sa} onChange={(e) => setSa(e.target.value)}>
            <option value="">Choose service account</option>
            {sas.data?.items?.map((x: any) => (
              <option key={x.id || x.ID} value={x.id || x.ID}>
                {x.name || x.Name}
              </option>
            ))}
          </select>
        )}
        <select value={role} onChange={(e) => setRole(e.target.value)}>
          <option value="viewer">Viewer</option>
          <option value="commenter">Commenter</option>
          <option value="editor">Editor</option>
        </select>
        <button disabled={(mode === "user" ? !email : !sa) || m.isPending}>
          Share
        </button>
        {m.error && <ErrorText err={m.error} />}
      </form>
      {grants.data?.items?.map((g: any) => (
        <div className="grant" key={g.id || g.ID}>
          <b>
            {g.principal?.email ||
              g.Principal?.Email ||
              g.principal?.id ||
              g.Principal?.ID}
          </b>
          <span>{g.role || g.Role}</span>
        </div>
      ))}
    </div>
  );
}

function ServiceAccounts() {
  const q = useQueryClient();
  const [name, setName] = useState("");
  const [token, setToken] = useState("");
  const sas = useQuery({
    queryKey: ["service-accounts"],
    queryFn: api.listServiceAccounts,
  });
  const create = useMutation({
    mutationFn: () => api.createServiceAccount(name),
    onSuccess: () => {
      setName("");
      q.invalidateQueries({ queryKey: ["service-accounts"] });
    },
  });
  return (
    <main className="page">
      <section className="pageHead">
        <div>
          <h1>Service accounts</h1>
          <p>
            Create headless identities for agents. Grant them document access
            from each document's Share tab.
          </p>
        </div>
        <form
          className="uploadCard"
          onSubmit={(e) => {
            e.preventDefault();
            create.mutate();
          }}
        >
          <h2>
            <Bot size={18} /> New service account
          </h2>
          <input
            placeholder="report-writer-bot"
            value={name}
            onChange={(e) => setName(e.target.value)}
          />
          <button disabled={!name || create.isPending}>Create</button>
          {create.error && <ErrorText err={create.error} />}
        </form>
      </section>
      {token && (
        <div className="tokenBox">
          <b>Copy this key now</b>
          <p>It will not be shown again.</p>
          <code>{token}</code>
        </div>
      )}
      <div className="grid serviceAccountGrid">
        {sas.data?.items?.map((sa: any) => (
          <ServiceAccountCard key={sa.id || sa.ID} sa={sa} onToken={setToken} />
        ))}
      </div>
    </main>
  );
}
function ServiceAccountCard({
  sa,
  onToken,
}: {
  sa: any;
  onToken: (t: string) => void;
}) {
  const id = sa.id || sa.ID;
  const q = useQueryClient();
  const keys = useQuery({
    queryKey: ["service-account-keys", id],
    queryFn: () => api.listServiceAccountKeys(id),
  });
  const [keyName, setKeyName] = useState("default");
  const key = useMutation({
    mutationFn: () => api.createServiceAccountKey(id, keyName),
    onSuccess: (r) => {
      onToken(r.token);
      q.invalidateQueries({ queryKey: ["service-account-keys", id] });
    },
  });
  const revoke = useMutation({
    mutationFn: (keyID: string) => api.revokeServiceAccountKey(id, keyID),
    onSuccess: () =>
      q.invalidateQueries({ queryKey: ["service-account-keys", id] }),
  });
  const upd = useMutation({
    mutationFn: () =>
      api.updateServiceAccount(
        id,
        sa.name || sa.Name,
        !(sa.disabled || sa.Disabled),
      ),
    onSuccess: () => q.invalidateQueries({ queryKey: ["service-accounts"] }),
  });
  return (
    <article className="saCard">
      <div>
        <Bot />
        <h3>{sa.name || sa.Name}</h3>
        <span>{id}</span>
      </div>
      <button
        className={sa.disabled || sa.Disabled ? "stateEnable" : "stateDisable"}
        disabled={upd.isPending}
        onClick={() => {
          const disabled = sa.disabled || sa.Disabled;
          const action = disabled ? "enable" : "disable";
          const consequence = disabled
            ? "Agents using active keys will be able to authenticate again."
            : "All agents using this service account will immediately lose access, but keys and grants are retained.";
          if (
            window.confirm(
              `${action[0].toUpperCase() + action.slice(1)} service account "${sa.name || sa.Name}"? ${consequence}`,
            )
          ) {
            upd.mutate();
          }
        }}
      >
        {sa.disabled || sa.Disabled ? "Enable" : "Disable"}
      </button>
      <div className="keyForm">
        <input value={keyName} onChange={(e) => setKeyName(e.target.value)} />
        <button onClick={() => key.mutate()}>
          <KeyRound size={15} /> New key
        </button>
      </div>
      {keys.data?.items?.map((k) => (
        <div className="keyRow" key={k.id}>
          <span>{k.name}</span>
          <code>{k.id}</code>
          <button
            className="dangerGhost"
            disabled={revoke.isPending}
            onClick={() => {
              if (
                window.confirm(
                  `Revoke key "${k.name}"? Agents using it will lose access immediately.`,
                )
              ) {
                revoke.mutate(k.id);
              }
            }}
          >
            Revoke
          </button>
        </div>
      ))}
      {revoke.error && <ErrorText err={revoke.error} />}
    </article>
  );
}

function ErrorText({ err }: { err: unknown }) {
  if (err instanceof APIError && err.code === "blob_storage_failed") {
    return (
      <div className="errorBox">
        <b>Upload failed</b>
        <p>
          The server could not upload this HTML file to blob storage. Check the
          S3 bucket, region, and credentials configured on the backend.
        </p>
      </div>
    );
  }
  return (
    <p className="error">
      {err instanceof Error ? err.message : "Something went wrong"}
    </p>
  );
}

createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
);
