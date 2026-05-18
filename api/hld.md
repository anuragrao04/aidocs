# API — High-Level Design

## Role in the system

`api` is the single source of truth for aidocs. Both `frontend`
(humans, via the browser) and `cli` (agents) talk to it. It owns
identity verification, storage, authorization, versioning, and the
comment lifecycle.

```
     frontend (browser)         cli (agent)
            \                     /
             \                   /
              +----> api <------+
                      |
              +-------+-------+
              |               |
          postgres        blob store
                               |
                       (HTML versions, immutable)
```

If `api` is down, nothing else works. If `api` is up, `frontend` and
`cli` can be rebuilt independently. In production `api` and
`frontend` ship as one Go binary; the split is logical, not
process-level.

## Responsibilities

- **Auth**
  - Google OAuth code exchange (server side; client secret lives here).
  - Issue session cookies for the frontend.
  - Issue long-lived aidocs credentials to `cli` after an interactive
    Google login via the OAuth loopback flow (PKCE). The credential is
    a single long-lived token in V1; the wire format leaves room to
    upgrade to a refresh + short-lived access token pair later without
    breaking clients.
  - Manage **service accounts** as first-class non-human principals
    owned by an aidocs user. Service-account keys are minted by aidocs
    (not by Google) and used for headless/bot workflows.
  - Allow **ownership transfer** of service accounts (e.g. when an
    employee leaves) so bots keep working without rotating their keys.
  - Verify every request and resolve the acting principal (user or
    service account).
- **Documents and versions**
  - Create documents.
  - Append immutable versions (HTML blob + sha256 + parent version).
  - Enforce optimistic concurrency via `base_version_id`.
  - Serve HTML through a separate render origin host.
- **Comments**
  - Create, list, update, resolve, reopen.
  - Persist anchor metadata (quote, prefix, suffix, DOM path, offsets).
  - On a new version, attempt reattachment; mark stale/orphaned where
    needed.
- **Authorization**
  - Per-document visibility: private, link-with-access, org.
  - Owner vs commenter vs viewer.
- **Operational**
  - Size/type limits on uploads.
  - Audit log of writes.
  - Rate limits per token.

## Non-responsibilities

- Does not render HTML for humans (that is the frontend + render origin).
- Does not run AI agents.
- Does not modify uploaded HTML content. Versions are immutable blobs.
- Does not handle realtime collaboration in V1.

## Storage model

Postgres holds metadata; blob storage (S3) holds HTML bodies.

```
users(id, email, name, picture_url, google_sub, created_at)
documents(id, title, owner_id, visibility, current_version_id, created_at)
versions(id, document_id, number, html_blob_key, sha256, parent_version_id,
         created_by, change_summary, created_at)
comments(id, document_id, created_on_version_id, author_id, body,
         selected_text, anchor_json, status, created_at, resolved_at)
comment_placements(id, comment_id, version_id, status, anchor_json,
                   matched_text, confidence)   -- added when needed

-- principals issued by aidocs
cli_credentials(id, user_id, name, token_hash, last_used_at,
                created_at, revoked_at)
service_accounts(id, name, owner_user_id, created_by_user_id,
                 created_at, disabled_at)
service_account_keys(id, service_account_id, name, token_hash,
                     last_used_at, created_at, revoked_at)
ownership_transfers(id, service_account_id, from_user_id, to_user_id,
                    initiated_at, accepted_at, status)
```

Blob keys look like `docs/<document_id>/<version_id>.html`.

## Render origin

Uploaded HTML is served from a **separate hostname** (e.g.
`doc.aidocs.example.com`) for isolation from the authenticated app
origin. The same Go binary serves it; the host header (or a second
listener configured by env) decides which router tree runs.

The render origin:

- has no app session cookies and ignores the session cookie name,
- serves the raw HTML with a strict CSP and frame-ancestors that
  permits only the configured `APP_ORIGIN` to embed it,
- injects the annotation bridge script via a wrapper page,
- accepts only short-lived signed tokens minted by the app origin.

This is a hard requirement of the deployment, not an optional split.
Self-hosters must configure two hostnames pointing at the same
binary.

## Implementation stack

- **Language:** Go.
- **HTTP:** `gin-gonic/gin` for routing and middleware (request id,
  recovery, structured logging, auth). Same process also serves the
  embedded frontend; see the deployment section below.
- **Database driver:** `pgx` (v5) directly, no ORM.
- **Queries:** `sqlc` generates type-safe Go from hand-written SQL.
  All queries live as `.sql` files in `internal/db/queries/` and are
  compiled into a typed `Queries` struct.
- **Migrations:** `goose`, plain SQL files in `internal/db/migrations/`,
  applied at deploy time.
- **JSON:** `encoding/json` for the wire; `anchor_json` and similar
  blobs stored as Postgres `jsonb` and round-tripped through
  `[]byte`/`json.RawMessage` in sqlc-generated types.
- **Auth:** `golang.org/x/oauth2` for the Google code exchange and
  ID-token verification via Google's library. The only hand-written
  OAuth-adjacent pieces are aidocs-specific: state persistence,
  loopback CLI code exchange, and PKCE verifier checking between
  `cli` and `api`. OAuth session state and one-time CLI codes are
  stored in Postgres (`oauth_states`), not process memory, so multiple
  server replicas can handle the same login flow. Bearer-token
  middleware checks `cli_credentials` and `service_account_keys`
  (hashed lookup).
- **Blob storage:** S3-compatible via the AWS SDK v2 `s3` client;
  works against MinIO for self-hosting.
- **Config:** environment variables only, parsed with `caarlos0/env`
  or equivalent. No config file in the deployed binary.
- **Tests:** standard `testing` + `testcontainers-go` for Postgres,
  `httptest` for handler tests.

A thin `internal/repo` package wraps the sqlc-generated `Queries` with
intent-named functions (`InsertVersion`, `ListCommentsForDocument`,
`ReattachCommentsForVersion`, `TransferServiceAccount`, ...). Handlers
call the repo; the repo is the only place that touches sqlc.

Deliberately not used:

- **GORM**, **ent**, **bun** — the schema is small and query-shaped;
  an ORM would add cost without buying anything here.
- Generated OpenAPI servers — the surface is small and stable enough
  to hand-write handlers.

## Deployment shape

aidocs ships as a **single Go binary / single Docker image**
(`aidocs-server`). The frontend (Vite + React, see `frontend/hld.md`) is
built at CI time and embedded into the binary via `//go:embed`. At
runtime the Gin router:

- serves `/v1/*` (API),
- serves `/.well-known/aidocs.json` (deployment discovery),
- serves the SPA from the embedded filesystem with an `index.html`
  fallback for client-side routes,
- serves the render-origin router when the request's `Host` matches
  `RENDER_ORIGIN`.

No Node runtime, no reverse proxy, no entrypoint script juggling
multiple processes. One image, one PID, two hostnames.

The same image runs the hosted deployment (`aidocs.anuragrao.dev`)
and any self-hosted deployment (`aidocs.razorpay.com`). All
deployment-specific values come from environment variables:

| Var | Purpose |
| --- | --- |
| `DATABASE_URL` | Postgres DSN. |
| `BLOB_BUCKET` / `BLOB_*` | S3-compatible blob storage config. |
| `APP_ORIGIN` | e.g. `https://aidocs.razorpay.com`. |
| `RENDER_ORIGIN` | e.g. `https://doc.aidocs.razorpay.com`. |
| `GOOGLE_OAUTH_CLIENT_ID` / `_SECRET` | Google OAuth credentials. |
| `SESSION_SECRET` | HMAC key for session and render tokens. |
| `ALLOWED_OAUTH_DOMAINS` | Optional Google Workspace domain allowlist. |

## V1 scope

- REST JSON API over HTTPS.
- Google OAuth login for the frontend.
- PATs for `cli`.
- Single-file HTML upload up to ~10 MB.
- Immutable versions, optimistic concurrency.
- Comments with anchored selection and lifecycle states.
- Visibility: private, link-with-access, org (Google Workspace domain).
