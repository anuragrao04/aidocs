# API — Interfaces

All endpoints are HTTPS, JSON, versioned under `/v1`. Authentication is
one of:

- session cookie (from the frontend, set after browser Google OAuth);
- `Authorization: Bearer <token>` where `<token>` is either an
  aidocs CLI credential (issued after `aidocs login`) or a service
  account key.

Responses include a stable `id` for every entity. The acting principal
for any authenticated request is either a `user` or a `service_account`.
`api` does not distinguish between them at the endpoint level except
for the service-account management routes themselves.

## Auth

Two entry points share the same Google OAuth code-exchange, but end
differently: the web flow sets a session cookie, the CLI flow returns
an aidocs-issued bearer token to a loopback redirect.

### `GET /v1/auth/google/start`
- **In:** `mode=web|cli` (default `web`), `redirect` (web only),
  `cli_redirect` (loopback URL like `http://127.0.0.1:54321/cb`,
  required when `mode=cli`), `code_challenge`, `code_challenge_method=S256`
  (PKCE, required when `mode=cli`), `state`.
- **Out:** 302 to Google OAuth consent screen. `mode`, `cli_redirect`,
  `code_challenge`, and `state` are remembered server-side keyed by
  `state`.

### `GET /v1/auth/google/callback`
- **In:** `code`, `state` from Google.
- **Behaviour:**
  - `mode=web` → sets session cookie, 302 to the original `redirect`.
  - `mode=cli` → mints a one-time `cli_code`, 302 to
    `<cli_redirect>?code=<cli_code>&state=<state>`.

### `POST /v1/auth/cli/exchange`
- **Auth:** none (PKCE-protected).
- **In:** `{ "code": "<cli_code>", "code_verifier": "...", "name": "my-laptop" }`
- **Out:** `{ "id": "cred_...", "token": "aidocs_cli_...", "principal": { "type": "user", "id": "usr_...", "email": "..." } }`
- **Notes:** `token` is shown once. V1 returns a single long-lived
  credential; the response shape is forward-compatible with a future
  `{ access_token, refresh_token, expires_in }` upgrade.

### `GET /v1/auth/cli/credentials` / `DELETE /v1/auth/cli/credentials/:id`
- List or revoke CLI credentials belonging to the calling user.

### `GET /v1/me`
- **Out:** `{ "principal": { "type": "user|service_account", "id": "..." }, "user": { "id", "email", "name", "picture_url" }? }`
  `picture_url` is the Google profile image URL when available and may
  be empty/omitted for placeholder users. When acting as a service
  account, `user` is omitted and the service account's `id`, `name`,
  and current `owner` are returned instead.

## Service accounts

Service accounts are aidocs-managed non-human principals owned by a
user. They authenticate with keys minted by aidocs (no Google involved).

Ownership is administrative only: it lets the owner rename/disable the
service account, mint/revoke keys, and transfer ownership. It does
**not** let the service account inherit the owner's document access.
New service accounts start with no document permissions; access is
assigned explicitly through document grants.

### `POST /v1/service-accounts`
- **Auth:** user session or CLI credential.
- **In:** `{ "name": "razorpay-report-bot" }`
- **Out:**
  ```json
  { "id": "sa_...",
    "name": "razorpay-report-bot",
    "owner": { "id": "usr_...", "email": "owner@example.com" },
    "disabled": false,
    "created_at": "..." }
  ```
- Caller becomes the initial `owner_user_id`.
- The service account receives no document grants by default.

### `GET /v1/service-accounts`
- **Out:** service accounts the caller owns plus those visible in their
  org (subject to visibility rules).

### `PATCH /v1/service-accounts/:id`
- **In:** `{ "name"?, "disabled"? }`
- **Auth:** owner only.

### `POST /v1/service-accounts/:id/keys`
- **In:** `{ "name": "prod-ci" }`
- **Out:** `{ "id": "sak_...", "token": "aidocs_sa_..." }` (shown once).

### `GET /v1/service-accounts/:id/keys` / `DELETE /v1/service-accounts/:id/keys/:key_id`
- List or revoke keys. Revocation is immediate.

### Ownership transfer

Used when the current owner leaves the company. The transfer is
two-sided so a bot cannot be silently reassigned.

#### `POST /v1/service-accounts/:id/transfer`
- **Auth:** current owner, or an admin acting on behalf of a
  deactivated user.
- **In:** `{ "to_user_email": "new-owner@example.com" }`
- **Out:** `{ "id": "xfer_...", "status": "pending" }`
- The target user sees a pending transfer in `GET /v1/service-accounts/transfers`.
- Keys keep working throughout. Nothing rotates.

#### `POST /v1/service-accounts/transfers/:id/accept`
- **Auth:** target user.
- **Out:** `{ "status": "accepted", "service_account": { ... } }`
- On acceptance, `owner_user_id` flips and an entry is written to
  `ownership_transfers` for audit.

#### `POST /v1/service-accounts/transfers/:id/decline`
- **Auth:** target user or initiator.

#### Admin override

#### `POST /v1/admin/service-accounts/:id/reassign`
- **Auth:** org admin. Used when the previous owner is already
  deactivated and cannot click accept. Writes the same audit row with
  `status=admin_reassigned` and the acting admin recorded.

## Authorization and grants

Document access is role-based. The acting principal is either a user or
a service account.

Roles, from least to most privileged:

- `viewer` — read document metadata, versions, HTML, and comments.
- `commenter` — `viewer` + create comments, resolve/reopen comments
  where allowed.
- `editor` — `commenter` + upload new versions.
- `owner` — `editor` + update document settings, manage grants,
  delete the document.

V1 rule: service accounts may be granted `viewer`, `commenter`, or
`editor`, but not `owner`. Documents must have a human owner.

Grant object:

```json
{ "id": "gr_...",
  "resource": { "type": "document", "id": "doc_..." },
  "principal": { "type": "user|service_account", "id": "...", "email": "..." },
  "role": "viewer|commenter|editor|owner",
  "granted_by": { "id": "usr_...", "email": "..." },
  "created_at": "..." }
```

### `GET /v1/documents/:id/grants`
- **Auth:** document `owner`.
- **Out:** `{ "items": [ <grant object> ] }`

### `POST /v1/documents/:id/grants`
- **Auth:** document `owner`.
- **In:**
  ```json
  { "principal": { "type": "user|service_account", "id": "..." },
    "role": "viewer|commenter|editor" }
  ```
  For users, callers may provide email instead of id:
  ```json
  { "principal": { "type": "user", "email": "reviewer@example.com" },
    "role": "commenter" }
  ```
- **Out:** grant object.
- **Notes:** `owner` is not accepted here in V1. If a user email has
  not logged in yet, aidocs creates a placeholder user row and links it
  to their Google login later. Granting a service account is how bots
  get access; service-account ownership alone is insufficient.

### `PATCH /v1/documents/:id/grants/:grant_id`
- **Auth:** document `owner`.
- **In:** `{ "role": "viewer|commenter|editor" }`
- **Out:** updated grant object.

### `DELETE /v1/documents/:id/grants/:grant_id`
- **Auth:** document `owner`.
- **Out:** 204.
- **Notes:** cannot delete the document owner's implicit owner grant.

## Documents

### `POST /v1/documents`
- **Auth:** user session or CLI credential. Service accounts cannot own
  documents in V1.
- **In (multipart):** `title`, `visibility`, `file` (HTML, ≤10 MB).
- **Out:** `{ "id": "doc_...", "current_version_id": "ver_..." }`
- **Behaviour:** caller becomes the document owner and receives the
  implicit `owner` role.

### `GET /v1/documents`
- **Auth:** any principal.
- **Out:** `{ "items": [ { id, title, updated_at, visibility, role } ] }`
- Lists documents where the principal has at least `viewer` access.

### `GET /v1/documents/:id`
- **Auth:** `viewer` or above.
- **Out:** `{ id, title, owner, visibility, current_version_id, created_at, updated_at, role }`

### `PATCH /v1/documents/:id`
- **Auth:** document `owner`.
- **In:** `{ "title"?, "visibility"? }`
- **Out:** updated document.

### `DELETE /v1/documents/:id`
- **Auth:** document `owner`.
- **Out:** 204.

## Versions

### `POST /v1/documents/:id/versions`
- **Auth:** `editor` or `owner`.
- **In (multipart):** `file` (HTML), `base_version_id`, optional `change_summary`.
- **Behaviour:** if `base_version_id` is not the current version,
  returns `409 Conflict` with the current version id.
- **Out:** `{ "id": "ver_...", "number": 4, "sha256": "..." }`

### `GET /v1/documents/:id/versions`
- **Auth:** `viewer` or above.
- **Out:** `{ "items": [ { id, number, created_by, created_at, change_summary, sha256 } ] }`

### `GET /v1/versions/:id`
- **Auth:** `viewer` or above on the parent document.
- **Out:** version metadata.

### `GET /v1/versions/:id/html`
- **Auth:** `viewer` or above on the parent document.
- **Out:** `text/html` body. Served from render origin in browser flows;
  CLI can fetch it directly.

## Comments

### `POST /v1/documents/:id/comments`
- **Auth:** `commenter` or above.
- **In:**
  ```json
  { "version_id": "ver_...",
    "body": "Add a number here.",
    "anchor": {
      "quote": "higher payment success rates",
      "prefix": "increased due to ",
      "suffix": " and improved checkout latency",
      "dom_path": "main/section[2]/p[1]",
      "start_offset": 24,
      "end_offset": 53
    } }
  ```
- **Out:** comment object.

### `GET /v1/documents/:id/comments`
- **Auth:** `viewer` or above.
- **Query:** `status` (`open|resolved|stale|orphaned|all`), `version_id`.
- **Out:**
  ```json
  { "items": [ {
      "id": "cmt_...",
      "author": { "id", "email", "name" },
      "body": "...",
      "selected_text": "...",
      "anchor": { ... },
      "status": "open",
      "created_on_version_id": "ver_...",
      "current_placement": {
        "version_id": "ver_...",
        "status": "attached|stale|orphaned",
        "anchor": { ... },
        "matched_text": "..."
      },
      "created_at": "...",
      "resolved_at": null
  } ] }
  ```

### `PATCH /v1/documents/:doc_id/comments/:comment_id`
- **Auth:** comment author, document `owner`, or document `editor`.
- **In:** `{ "status"?: "open|resolved", "body"? }`
- **Out:** updated comment.

### `DELETE /v1/documents/:doc_id/comments/:comment_id`
- **Auth:** comment author or document `owner`.
- **Out:** 204.

## Errors

Standard JSON shape:

```json
{ "error": { "code": "version_conflict",
             "message": "base_version_id is stale",
             "details": { "current_version_id": "ver_..." } } }
```

Common codes: `unauthorized`, `forbidden`, `not_found`,
`version_conflict`, `payload_too_large`, `invalid_html`,
`rate_limited`.

## Render origin endpoints

Served from a separate host, e.g. `doc.aidocs.example.com`:

- `GET /v/:version_id` → wrapper HTML page that embeds the stored HTML
  and injects the annotation bridge. Requires a short-lived signed
  token issued by `api`.

## Non-interfaces

- No GraphQL endpoint.
- No WebSocket in V1.
- No bulk import endpoints.
- No admin endpoints exposed publicly.
