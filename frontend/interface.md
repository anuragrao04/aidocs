# Web — Interfaces

The frontend has two external interfaces: the human (browser UI) and
the `api` server. It does not expose any server-side surface of its
own — static assets and OAuth callbacks are served by `api`.

## 1. Human inputs (UI)

| Action | Input | Output |
| --- | --- | --- |
| Sign in | Click "Sign in with Google" | Redirect to Google, then back to app with session cookie set by `api`; top bar shows Google profile avatar. |
| Upload document | HTML file (single self-contained file, ≤10 MB) | New document created, redirected to its view. |
| Upload new version | HTML file + parent document id + base version id | New version appended if caller has `editor` or `owner`. |
| Open document | Document id (URL) | Rendered HTML in sandboxed iframe. |
| Select text | Mouse/touch selection inside iframe | Floating popup with comment composer. |
| Save comment | Selection anchor + body text | Comment appears in sidebar if caller has `commenter` or above; persisted via `api`. |
| Resolve / reopen comment | Comment id | Status updated if caller is author, document `editor`, or document `owner`. |
| Manage access | User/service-account principal + role | Document grant created/updated/revoked if caller is document `owner`. |
| Manage account | Click profile avatar | Popover with full name, email, user id, and service-account management link. |
| Manage service accounts | Service-account name/key name | Create service accounts, mint one-time keys, enable/disable accounts. |
| Copy comments for agent | Document id | JSON payload copied to clipboard, identical shape to `aidocs docs comments list --json`. |

## 2. Inputs from `api` (consumed by frontend)

The frontend calls `api` over same-origin HTTPS with the session
cookie. Shapes match `api/interface.md`; the frontend treats these as
read-only contracts.

- `GET /v1/me` → current principal/user, including `user.picture_url` when available.
- `GET /v1/documents` → accessible documents including caller role.
- `GET /v1/documents/:id` → metadata + current version id + caller role.
- `GET /v1/documents/:id/versions` → version list.
- `GET /v1/versions/:id/html` → raw HTML for direct/CLI flows; browser rendering normally uses the render-origin wrapper.
- `GET /v1/documents/:id/comments` → comments with anchors and statuses.
- `POST /v1/documents` (multipart) → create doc; human user only in V1.
- `POST /v1/documents/:id/versions` (multipart) → new version; requires `editor` or `owner`.
- `POST /v1/documents/:id/comments` → create comment; requires `commenter` or above.
- `PATCH /v1/documents/:doc_id/comments/:comment_id` → update status/body.
- `GET /v1/documents/:id/grants` → list grants; owner only.
- `POST /v1/documents/:id/grants` → add user/service-account grant; owner only.
- `PATCH /v1/documents/:id/grants/:grant_id` → update grant role; owner only.
- `DELETE /v1/documents/:id/grants/:grant_id` → revoke grant; owner only.
- `GET /v1/service-accounts` → list owned service accounts.
- `POST /v1/service-accounts` → create service account.
- `PATCH /v1/service-accounts/:id` → rename/enable/disable owned service account.
- `GET /v1/service-accounts/:id/keys` → list service-account keys.
- `POST /v1/service-accounts/:id/keys` → mint one-time service-account key.
- `DELETE /v1/service-accounts/:id/keys/:key_id` → revoke service-account key.

## 3. Iframe ↔ parent messaging

The render origin serves the user's HTML plus an injected annotator
script. The annotator is built on `@apache-annotator/dom`
(W3C Web Annotation selectors) for capture and resolution, and
`mark.js` for painting; `rangy` is used only as a fallback for
browsers with quirky `Selection`/`Range` behaviour. The selector
objects produced by `@apache-annotator/dom` map directly onto the
`anchor` shape in `api/interface.md`.

The script communicates with the parent app via `postMessage`.

Messages from iframe → parent:

```json
{ "type": "selection",
  "quote": "higher payment success rates",
  "prefix": "increased due to ",
  "suffix": " and improved checkout latency",
  "domPath": "main/section[2]/p[1]",
  "startOffset": 24,
  "endOffset": 53 }
```

```json
{ "type": "selection-cleared" }
```

```json
{ "type": "ready", "documentId": "doc_123", "versionId": "v_7" }
```

Messages from parent → iframe:

```json
{ "type": "highlight", "commentId": "cmt_1", "anchor": { ... } }
```

```json
{ "type": "scroll-to", "commentId": "cmt_1" }
```

The parent never trusts iframe DOM directly. Origins are checked on
every `postMessage`.

## 4. Outputs to humans

- HTML document rendered as-is in the sandboxed iframe.
- Comments sidebar with author, body, status, timestamp.
- Version dropdown with timestamps and change summaries.
- Shareable document URL.
- "Copy for agent" JSON snippet.

## 5. Non-interfaces

Web does not expose:

- A public REST API.
- A WebSocket server.
- Any persistent storage.
- Any way to bypass `api` for document/comment data.
