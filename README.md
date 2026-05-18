# aidocs

Review AI-generated HTML documents with a Google-Docs-like workflow.

`aidocs` lets agents and humans create, host, version, comment on, and share single-file HTML documents. It includes a Go API server, an embedded React review UI, and a Go CLI for agent/headless workflows.

## Why aidocs?

AI agents are good at producing rich HTML reports, specs, dashboards, and design docs. Reviewing those artifacts through chat is painful. `aidocs` turns each generated HTML file into a reviewable document with immutable versions, anchored comments, sharing, and CLI automation.

## Features

- Single self-contained HTML document per artifact
- Browser review UI with sandboxed rendering
- Anchored comments and resolve/reopen workflow
- Immutable versions with optimistic concurrency
- Google OAuth for human users
- Native service accounts for headless/agent use
- Go CLI with compact default output and JSON mode
- Generated OpenAPI/Swagger docs
- Self-hostable API server with embedded frontend

## Install from source

```bash
git clone https://github.com/anuragrao/aidocs.git
cd aidocs
make build
```

Binaries are written to:

```text
bin/aidocs-server
bin/aidocs
```

## Run locally

You need PostgreSQL and an S3-compatible blob store, or configure the server for your deployment environment.

Common environment variables:

```bash
export DATABASE_URL='postgres://user:pass@localhost:5432/aidocs?sslmode=disable'
export BLOB_BUCKET='aidocs'
export BLOB_REGION='us-east-1'
export APP_ORIGIN='http://localhost:8080'
export RENDER_ORIGIN='http://localhost:8080'
export GOOGLE_OAUTH_CLIENT_ID='...'
export GOOGLE_OAUTH_CLIENT_SECRET='...'
export SESSION_SECRET='change-me'
```

Then run:

```bash
./bin/aidocs-server
```

Open:

```text
http://localhost:8080
```

API docs:

```text
http://localhost:8080/api-docs
http://localhost:8080/openapi.json
```

Runtime commit endpoint:

```text
http://localhost:8080/commit.txt
```

## CLI quickstart

```bash
aidocs auth login localhost:8080
aidocs auth whoami

aidocs docs create report.html --title "Agent report"
aidocs docs list
aidocs docs pull doc_123 --out report.html
aidocs docs push doc_123 report.html --summary "Update findings"

aidocs docs comments list doc_123
aidocs docs comments create doc_123 --quote "important text" --body "Please clarify this."
aidocs docs comments resolve doc_123 cmt_123
```

Use `--json` for structured output:

```bash
aidocs --json docs list
```

## Development

```bash
make build
make swagger
go test ./...
npm run build --prefix frontend
```

Useful targets:

```text
make build          build server and CLI
make build-backend  build API server
make build-cli      build CLI
make build-frontend build embedded frontend assets
make swagger        regenerate OpenAPI docs
```

## Project layout

```text
api/       Go API server and embedded frontend assets
cli/       Go CLI
frontend/  React + Vite frontend
```

## Security model

Uploaded HTML is rendered in a sandboxed review frame. Treat uploaded documents as untrusted content and deploy with separate app/render origins for stronger isolation in production.

## License

Apache-2.0
