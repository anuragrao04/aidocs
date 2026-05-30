export type Principal = {
  type: "user" | "service_account";
  id: string;
  email?: string;
  name?: string;
  picture_url?: string;
};
export type Document = {
  id: string;
  title: string;
  visibility: string;
  owner?: Principal;
  current_version_id?: string;
};
export type Version = {
  id: string;
  number: number;
  document_id?: string;
  change_summary?: string;
  sha256?: string;
};
export type Anchor = {
  quote: string;
  prefix?: string;
  suffix?: string;
  dom_path?: string;
  start_offset?: number;
  end_offset?: number;
};
export type Comment = {
  id: string;
  body: string;
  selected_text: string;
  status: string;
  author: Principal;
  created_on_version_id: string;
  anchor: Anchor;
  current_placement?: {
    status: string;
    confidence: number;
    matched_text: string;
  };
};
export type Grant = {
  id: string;
  principal?: Principal;
  role: string;
};
export type ServiceAccount = {
  id: string;
  name: string;
  disabled?: boolean;
  owner?: Principal;
};
export type ServiceAccountKey = { id: string; name: string };

export class APIError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
  ) {
    super(message);
  }
}

// API responses mix PascalCase (Go struct JSON) and snake_case across
// endpoints. Normalize every response to a single snake_case shape so call
// sites never need `d.id || d.ID` style fallbacks (web-02/web-03).
function toSnakeKey(key: string): string {
  return key
    .replace(/([a-z0-9])([A-Z])/g, "$1_$2")
    .replace(/([A-Z]+)([A-Z][a-z])/g, "$1_$2")
    .toLowerCase();
}

function normalizeKeys(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(normalizeKeys);
  if (value && typeof value === "object") {
    const out: Record<string, unknown> = {};
    for (const [k, v] of Object.entries(value as Record<string, unknown>)) {
      out[toSnakeKey(k)] = normalizeKeys(v);
    }
    return out;
  }
  return value;
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const res = await fetch(path, {
    credentials: "include",
    ...init,
    headers: {
      ...(init.body instanceof FormData
        ? {}
        : { "Content-Type": "application/json" }),
      ...(init.headers || {}),
    },
  });
  if (!res.ok) {
    let msg = res.statusText;
    let code = "http_error";
    try {
      const payload = await res.json();
      msg = payload.error?.message || msg;
      code = payload.error?.code || code;
    } catch {}
    throw new APIError(res.status, code, msg);
  }
  if (res.status === 204) return undefined as T;
  const ct = res.headers.get("content-type") || "";
  if (ct.includes("application/json")) {
    return normalizeKeys(await res.json()) as T;
  }
  return (await res.text()) as T;
}

export const api = {
  me: () =>
    request<{
      principal: Principal;
      user?: Principal;
      service_account?: unknown;
    }>("/v1/me"),
  loginURL: () =>
    `/v1/auth/google/start?mode=web&redirect=${encodeURIComponent(location.pathname + location.search)}`,
  listDocuments: () => request<{ items: Document[] }>("/v1/documents"),
  getDocument: (id: string) => request<Document>(`/v1/documents/${id}`),
  createDocument: (title: string, visibility: string, file: File) => {
    const fd = new FormData();
    fd.set("title", title);
    fd.set("visibility", visibility);
    fd.set("file", file);
    return request<{ id: string; current_version_id: string }>(
      "/v1/documents",
      { method: "POST", body: fd },
    );
  },
  listVersions: (doc: string) =>
    request<{ items: Version[] }>(`/v1/documents/${doc}/versions`),
  createVersion: (doc: string, base: string, summary: string, file: File) => {
    const fd = new FormData();
    fd.set("base_version_id", base);
    fd.set("change_summary", summary);
    fd.set("file", file);
    return request<Version>(`/v1/documents/${doc}/versions`, {
      method: "POST",
      body: fd,
    });
  },
  renderToken: (version: string) =>
    request<{ token: string; url: string }>(
      `/v1/versions/${version}/render-token`,
      { method: "POST" },
    ),
  listComments: (doc: string, version?: string) =>
    request<{ items: Comment[] }>(
      `/v1/documents/${doc}/comments${version ? `?version_id=${encodeURIComponent(version)}` : ""}`,
    ),
  createComment: (
    doc: string,
    version: string,
    body: string,
    quote: string,
    anchor?: Partial<Anchor>,
  ) =>
    request<Comment>(`/v1/documents/${doc}/comments`, {
      method: "POST",
      body: JSON.stringify({
        version_id: version,
        body,
        // Prefer a real anchor captured from the render bridge; fall back to a
        // best-effort placeholder when the bridge did not supply selection
        // offsets. See web-12 and the iframe-bridge contract (server-07).
        anchor: {
          quote,
          prefix: anchor?.prefix ?? "",
          suffix: anchor?.suffix ?? "",
          dom_path: anchor?.dom_path ?? "body",
          start_offset: anchor?.start_offset ?? 0,
          end_offset: anchor?.end_offset ?? quote.length,
        },
      }),
    }),
  patchComment: (doc: string, id: string, body: string, status: string) =>
    request<Comment>(`/v1/documents/${doc}/comments/${id}`, {
      method: "PATCH",
      body: JSON.stringify({ body, status }),
    }),
  listGrants: (doc: string) =>
    request<{ items: Grant[] }>(`/v1/documents/${doc}/grants`),
  createGrant: (doc: string, address: string, role: string) =>
    request<Grant>(`/v1/documents/${doc}/grants`, {
      method: "POST",
      body: JSON.stringify({ address, role }),
    }),
  listServiceAccounts: () =>
    request<{ items: ServiceAccount[] }>("/v1/service-accounts"),
  createServiceAccount: (label: string, domain?: string) =>
    request<{
      id: string;
      label: string;
      name: string;
      key: { id: string; token: string };
    }>("/v1/service-accounts", {
      method: "POST",
      body: JSON.stringify(domain ? { label, domain } : { label }),
    }),
  deleteDocument: (id: string) =>
    request<void>(`/v1/documents/${id}`, { method: "DELETE" }),
  updateServiceAccount: (id: string, name: string, disabled: boolean) =>
    request<ServiceAccount>(`/v1/service-accounts/${id}`, {
      method: "PATCH",
      body: JSON.stringify({ name, disabled }),
    }),
  listServiceAccountKeys: (id: string) =>
    request<{ items: ServiceAccountKey[] }>(`/v1/service-accounts/${id}/keys`),
  createServiceAccountKey: (id: string, name: string) =>
    request<{ id: string; token: string }>(`/v1/service-accounts/${id}/keys`, {
      method: "POST",
      body: JSON.stringify({ name }),
    }),
  revokeServiceAccountKey: (id: string, key: string) =>
    request<void>(`/v1/service-accounts/${id}/keys/${key}`, {
      method: "DELETE",
    }),
};

export const docID = (d: Partial<Document>) => d.id || "";
export const docTitle = (d: Partial<Document>) => d.title || "Untitled";
export const currentVersionID = (d: Partial<Document>) =>
  d.current_version_id || "";
export const versionID = (v: Partial<Version>) => v.id || "";
export const versionNumber = (v: Partial<Version>) => v.number || 0;
