# CLI — Interfaces

The CLI binary is `aidocs`. Inputs are command-line arguments, stdin
(for piping HTML), and a local config file. Outputs are stdout
(human or JSON), stderr (diagnostics), and exit codes.

## Global flags

| Flag | Meaning |
| --- | --- |
| `--server <url>` | Override API base URL/context for this invocation. Bare hosts are normalized to `https://...`. Default is active context, or `https://aidocs.anuragrao.dev` if unset. |
| `--token <token>` | Override stored credential for this invocation. |
| `--json` | Machine-readable output. |
| `-q, --quiet` | Suppress non-error output. |
| `-v, --verbose` | Debug logging to stderr. |

Environment:

| Var | Meaning |
| --- | --- |
| `AIDOCS_SERVER` | Same as `--server`. |
| `AIDOCS_TOKEN` | Bearer token to use. Accepts either an aidocs CLI credential or a service account key. Takes precedence over the stored credential, intended for CI/bots. |

Exit codes: `0` success, `1` generic error, `2` usage error,
`3` auth error, `4` not found, `5` version conflict, `6` network error.

## Config file

Path: `$XDG_CONFIG_HOME/aidocs/config.json` (or `~/.config/aidocs/config.json`).
Permissions: `0600`.

```json
{ "active_context": "aidocs.razorpay.com",
  "contexts": {
    "aidocs.anuragrao.dev": {
      "server": "https://aidocs.anuragrao.dev",
      "credential": {
        "id": "cred_...",
        "token": "aidocs_cli_...",
        "principal": { "type": "user", "email": "me@example.com" }
      },
      "default_doc": null,
      "pulled": { "doc_123": "ver_7" }
    },
    "aidocs.razorpay.com": {
      "server": "https://aidocs.razorpay.com",
      "credential": { "id": "cred_...", "token": "aidocs_cli_..." },
      "default_doc": null,
      "pulled": {}
    }
  } }
```

The shape of `credential` is forward-compatible: when V2 introduces
refresh + access tokens, the same field will carry
`{ refresh_token, access_token, expires_at, ... }` and older configs
will be migrated on first use.

`pulled` is scoped per context and tracks the last version each
document was pulled at, used as the default `--base-version` for
`push`.

## Commands

### `aidocs auth login [server] [--name <label>]`
- **In:** optional `server`. If omitted, logs into
  `https://aidocs.anuragrao.dev`. Bare hosts such as
  `aidocs.razorpay.com` are normalized to `https://aidocs.razorpay.com`.
- Starts a local loopback server, opens the browser to
  `<server>/v1/auth/google/start?mode=cli&...` with a PKCE challenge,
  receives a one-time code, and exchanges it at
  `POST /v1/auth/cli/exchange` for an aidocs-issued credential.
  `--name` labels the credential (defaults to `<hostname>`).
- **Out:** stores credential in keychain/config. Prints
  `Logged in as <email>`.
- **Notes:** equivalent to the underlying `aidocs auth login` alias.

### `aidocs auth whoami`
- **Out (text):** `name <email>` for users, `service-account <name> (owner: <email>)` for service accounts.
- **Out (json):** `GET /v1/me` response.

### `aidocs auth logout`
- Deletes local credentials for the active context. Does not revoke
  them on the server unless `--revoke` is passed.

### `aidocs context list`
- Lists saved deployment contexts, the active marker, server URL, and
  principal summary.

### `aidocs context use <server>`
- Sets the active context. Accepts a saved context name/host or full
  URL. Does not perform login by itself.

## Service accounts

These commands manage aidocs-native non-human principals.

### `aidocs sa create <name>`
- Creates a service account owned by the calling user.
- Ownership is administrative only; the service account receives no
  document access until granted on a document.
- **Out (json):** `{ "id": "sa_...", "name": "...", "owner": { ... } }`

### `aidocs sa list`
- Lists service accounts the caller owns (and, with `--all`, those
  visible in the org).

### `aidocs sa key create <sa_id> [--name <label>]`
- Mints a new key. Token is printed once on stdout (or in the JSON
  output) and never retrievable again.
- **Out (json):** `{ "id": "sak_...", "token": "aidocs_sa_..." }`

### `aidocs sa key list <sa_id>` / `aidocs sa key revoke <sa_id> <key_id>`
- List or revoke keys. Revocation is immediate server-side.

### `aidocs sa transfer <sa_id> --to <email>`
- Initiates an ownership transfer. Bot keeps working; keys are not
  rotated.
- **Out (json):** `{ "id": "xfer_...", "status": "pending" }`

### `aidocs sa transfers list`
- Lists transfers initiated by or addressed to the caller.

### `aidocs sa transfer accept <xfer_id>` / `aidocs sa transfer decline <xfer_id>`
- Target user accepts or declines. On accept, ownership flips and an
  audit record is written.

### `aidocs docs list`
- **Out (json):** `{ "items": [ { "id", "title", "updated_at", "visibility", "role" } ] }`

### `aidocs docs create <file.html> [--title T] [--visibility private|link|org]`
- **In:** path to a single HTML file.
- **Auth:** user credential only; service accounts cannot own
  documents in V1.
- **Out:** `{ "id": "doc_...", "current_version_id": "ver_..." }`

### `aidocs docs show <doc_id>`
- **Out:** document metadata + current version + caller role.

### `aidocs docs grants list <doc_id>`
- Lists document grants. Requires document `owner`.
- **Out (json):** `{ "items": [ <grant object> ] }`

### `aidocs docs grants add <doc_id> --principal user:<id-or-email>|sa:<id> --role viewer|commenter|editor`
- Grants a user or service account access to a document. Requires
  document `owner`. `owner` is not accepted as a grant role in V1.

### `aidocs docs grants update <doc_id> <grant_id> --role viewer|commenter|editor`
- Updates an existing grant. Requires document `owner`.

### `aidocs docs grants revoke <doc_id> <grant_id>`
- Revokes a grant. Requires document `owner`; cannot revoke the
  implicit owner grant.

### `aidocs docs pull <doc_id> [--version ver_...] [--out path]`
- Requires `viewer` or above.
- Default: write current version's HTML to stdout.
- Side effect: records `pulled[doc_id] = ver_...` in config.
- **Out:** raw HTML.

### `aidocs docs push <doc_id> <file.html> [--base-version ver_...] [--summary "..."]`
- Requires `editor` or `owner`.
- If `--base-version` is omitted, uses `pulled[doc_id]` in the active context.
- On `409` version conflict, exits with code `5` and prints the
  current version id on stderr.
- **Out (json):** `{ "id": "ver_...", "number": 4, "sha256": "..." }`

### `aidocs docs comments list <doc_id> [--status open|resolved|stale|orphaned|all] [--version ver_...]`
- Requires `viewer` or above.
- **Out (json):** identical shape to `GET /v1/documents/:id/comments`.

Example output:

```json
{ "items": [
  { "id": "cmt_1",
    "body": "Add a number here.",
    "selected_text": "higher payment success rates",
    "anchor": {
      "quote": "higher payment success rates",
      "prefix": "increased due to ",
      "suffix": " and improved checkout latency",
      "dom_path": "main/section[2]/p[1]",
      "start_offset": 24,
      "end_offset": 53 },
    "status": "open",
    "current_placement": { "version_id": "ver_7", "status": "attached" }
  }
] }
```

### `aidocs docs comments resolve <doc_id> <cmt_id> [<cmt_id> ...]`
- Marks one or more comments resolved. Requires comment author,
  document `editor`, or document `owner`.

### `aidocs docs comments reopen <doc_id> <cmt_id> [<cmt_id> ...]`
- Marks them open again. Same permissions as resolve.

### `aidocs open <doc_id>`
- Opens the document URL in the system browser. Convenience.

## Stdin / piping

- `aidocs docs pull doc_123 > report.html`
- `cat report.html | aidocs docs create - --title "Q4 plan"` — `-` means
  read HTML from stdin.

## Inputs from API

The CLI consumes the same endpoints described in `api/interface.md`.
No additional contract.

## Outputs to agents

- HTML body on stdout (for `pull`).
- JSON on stdout (for everything else with `--json`).
- Human-readable text otherwise.
- Errors with stable codes on stderr and via exit code.

## Non-interfaces

- No interactive TUI in V1.
- No plugin system.
- No background daemon.
- No local indexing of documents.
