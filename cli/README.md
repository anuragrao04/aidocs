# aidocs CLI

`aidocs` is the scriptable CLI for the aidocs HTML review service.

## Install from source

```bash
go build -o bin/aidocs ./cli
```

## Auth

```bash
aidocs auth login                         # hosted default
aidocs auth login aidocs.razorpay.com     # self-hosted/internal
aidocs auth whoami
aidocs auth logout                         # revokes server credential by default
AIDOCS_NO_KEYCHAIN=1 aidocs auth login     # force file-only token storage
```

Self-hosted login only needs the server URL. The server owns Google OAuth config (`GOOGLE_OAUTH_CLIENT_ID`/secret); the CLI uses server-mediated loopback PKCE.

By default, login stores the secret token in the OS keychain when available and keeps only metadata plus a `token_ref` in `~/.config/aidocs/config.json`. If the keychain is unavailable, it falls back to storing the token in the config file with `0600` permissions. Set `AIDOCS_NO_KEYCHAIN=1` to force file-only storage.

For CI/bots:

```bash
export AIDOCS_SERVER=https://aidocs.example.com
export AIDOCS_TOKEN=aidocs_sa_...
```

## Core agent loop

```bash
aidocs docs list
aidocs docs pull doc_123 --out report.html
aidocs docs comments list doc_123 --json > review.json
# edit report.html
aidocs docs push doc_123 report.html --summary "Address review comments"
aidocs docs comments resolve doc_123 cmt_1 cmt_2
```

## Service accounts

```bash
aidocs sa create report-bot
aidocs sa key create sa_123 --name prod-ci
aidocs sa key revoke sa_123 sak_123
```

Disable all `sa` commands in a packaged environment:

```bash
AIDOCS_DISABLE_SA_COMMANDS=1 aidocs sa list
```

## Output

Use `--json` for machine-readable output. Without `--json`, commands print concise human-readable summaries.

## Exit codes

- `0`: success
- `1`: generic error
- `2`: usage error
- `3`: auth/permission error
- `4`: not found
- `5`: version conflict
- `6`: network error
