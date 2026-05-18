# CLI — High-Level Design

## Role in the system

`cli` (`aidocs`) is the agent-facing interface. It is a thin client over
the `api` server. Anything `cli` does, a human could also do by curling
`api`, but the CLI gives AI agents and humans a stable, scriptable
surface.

```
   AI agent / shell user
            |
            v
        aidocs CLI ----HTTPS----> api ----> storage
```

The CLI is the second-class citizen of the system: simple, stateless
except for a local config/keychain entry holding deployment-scoped
credentials, active context, and a small pulled-version cache.

## Responsibilities

- **Auth bootstrap**
  - `aidocs auth login` logs into the hosted deployment by default
    (`https://aidocs.anuragrao.dev`). `aidocs auth login <server>` accepts
    either a bare host (`aidocs.razorpay.com`) or full URL
    (`https://aidocs.razorpay.com`) for self-hosted/internal
    deployments.
  - Login runs the OAuth 2.0 loopback flow with PKCE: starts a local
    HTTP server on `127.0.0.1:<random>`, opens the browser to the
    chosen server's Google OAuth flow with `mode=cli`, receives a
    one-time code on the loopback, and exchanges it with `api` for an
    aidocs-issued CLI credential.
  - Store the credential securely: OS keychain when available, falling
    back to `~/.config/aidocs/config.json` with `0600` permissions.
  - The credential is a single long-lived token in V1. The CLI sends
    it as `Authorization: Bearer <token>`. The on-disk and on-wire
    shape is forward-compatible with a future upgrade to a refresh
    token + short-lived access token pair; the user experience does
    not change.
  - For headless / CI use, the CLI reads `AIDOCS_TOKEN` from the
    environment. The token can be either a CLI credential (created by
    `aidocs auth login` on some machine) or a **service account key**
    minted in the aidocs UI/CLI. The wire protocol is identical.
  - Google service accounts are intentionally not supported in V1.
    Headless identities are aidocs-native so they can be transferred
    between owners without rotating keys.
- **Documents**
  - Create a document by uploading an HTML file. Only human user
    credentials can create documents in V1; service accounts cannot
    own documents.
  - Append a new version of an existing document when the acting
    principal has `editor` or `owner` access.
  - Download the current or a specific version's HTML when the acting
    principal has `viewer` access.
  - List documents the active principal can access.
- **Comments**
  - List comments for a document, filterable by status.
  - Output them as JSON suitable for piping into an agent prompt.
  - Resolve / reopen a comment.
  - (V1.1) Write comments back from an agent.
- **Service accounts**
  - Create a service account owned by the calling user, mint and
    revoke keys, list service accounts the user owns.
  - Service-account ownership is administrative only; it does not
    grant the bot access to the owner's documents.
  - Grant service accounts explicit document roles (`viewer`,
    `commenter`, `editor`) via the document grants API.
  - Initiate and accept ownership transfers so bots survive employees
    leaving the company without rotating keys.
- **Ergonomics**
  - Sensible defaults so a one-shot agent can do
    `aidocs docs pull` → edit → `aidocs docs push` with minimal flags.
  - Optimistic concurrency: every `push` defaults to `--base-version`
    being the version most recently pulled.
  - Machine-readable `--json` mode for every command.

## Non-responsibilities

- No business logic. The CLI does not anchor comments, render HTML, or
  resolve placements itself.
- No local persistence beyond config and a small per-document cache
  of `last pulled version_id` to support optimistic concurrency.
- No bundled AI agent. The CLI is the wire between an agent and the
  service; the agent itself lives elsewhere.

## Where it fits

A typical agent loop:

```
aidocs docs pull doc_123 > report.html
aidocs docs comments list doc_123 --json > review.json
# agent reads report.html + review.json, produces report.v2.html
aidocs docs push doc_123 report.v2.html
aidocs docs comments resolve doc_123 cmt_456 cmt_789
```

The CLI keeps the loop boring and reliable so the interesting parts
live in the agent.

## Implementation stack

- **Language:** Go, same as `api`. Shipping a single static binary
  (`aidocs`) for macOS, Linux, and Windows from `goreleaser` keeps
  install/upgrade trivial for agents and humans.
- **CLI framework:** `spf13/cobra` for commands and flags;
  `spf13/pflag` semantics are familiar and ubiquitous.
- **HTTP client:** `net/http` + a small typed client package that is
  generated or hand-written off the `api` interface. Shared types for
  request/response payloads live in a `pkg/aidocsclient` module that
  third-party Go code can import.
- **Config:** JSON file at `$XDG_CONFIG_HOME/aidocs/config.json`
  (`0600`), OS keychain when available via `zalando/go-keyring`.
- **OAuth loopback:** `golang.org/x/oauth2` plus a tiny embedded HTTP
  server on `127.0.0.1` for the PKCE redirect.
- **Output:** human-readable via `text/tabwriter`; JSON via
  `encoding/json` when `--json` is set.
- **Tests:** `testing`, table-driven; integration tests run against a
  locally booted `api` plus `testcontainers-go` Postgres.

## V1 scope

- `login`, `whoami`, `logout` (`auth ...` aliases allowed).
- `context list`, `context use` for multiple deployments.
- `docs list`, `docs create`, `docs show`.
- `docs grants list|add|update|revoke`.
- `pull`, `push`.
- `comments list`, `resolve`, `reopen`.
- `sa create|list|key|transfer`.
- `--json` everywhere.
- Hosted default plus configurable server URL for self-hosted deployments.
