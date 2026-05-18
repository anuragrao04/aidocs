export type Principal = {
  type: "user" | "service_account";
  id: string;
  email?: string;
  name?: string;
  picture_url?: string;
};
export type Document = {
  ID?: string;
  id?: string;
  Title?: string;
  title?: string;
  Visibility?: string;
  visibility?: string;
  Owner?: Principal;
  owner?: Principal;
  CurrentVersionID?: string;
  current_version_id?: string;
};
export type Version = {
  ID?: string;
  id?: string;
  Number?: number;
  number?: number;
  DocumentID?: string;
  document_id?: string;
  ChangeSummary?: string;
  change_summary?: string;
  SHA256?: string;
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
  ID?: string;
  id?: string;
  Principal?: Principal;
  principal?: Principal;
  Role?: string;
  role?: string;
};
export type ServiceAccount = {
  ID?: string;
  id?: string;
  Name?: string;
  name?: string;
  Disabled?: boolean;
  disabled?: boolean;
  Owner?: Principal;
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
  return (
    ct.includes("application/json") ? await res.json() : await res.text()
  ) as T;
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
  createComment: (doc: string, version: string, body: string, quote: string) =>
    request<Comment>(`/v1/documents/${doc}/comments`, {
      method: "POST",
      body: JSON.stringify({
        version_id: version,
        body,
        anchor: {
          quote,
          prefix: "",
          suffix: "",
          dom_path: "body",
          start_offset: 0,
          end_offset: quote.length,
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
  createGrant: (doc: string, principal: Principal, role: string) =>
    request<Grant>(`/v1/documents/${doc}/grants`, {
      method: "POST",
      body: JSON.stringify({ principal, role }),
    }),
  listServiceAccounts: () =>
    request<{ items: ServiceAccount[] }>("/v1/service-accounts"),
  createServiceAccount: (name: string) =>
    request<{ id: string; name: string }>("/v1/service-accounts", {
      method: "POST",
      body: JSON.stringify({ name }),
    }),
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

export const docID = (d: Document) => d.id || d.ID || "";
export const docTitle = (d: Document) => d.title || d.Title || "Untitled";
export const currentVersionID = (d: Document) =>
  d.current_version_id || d.CurrentVersionID || "";
export const versionID = (v: Version) => v.id || v.ID || "";
export const versionNumber = (v: Version) => v.number || v.Number || 0;
