# aidocs

Give your agents the power to publish beautiful, reviewable documents.

`aidocs` is a Google-Docs-like review layer for AI-generated HTML artifacts. Agents create rich, self-contained HTML reports, specs, dashboards, diagrams, and decision docs; humans review them in the browser with comments, versions, and sharing.

Instead of pasting giant reports into chat, let agents push a document, iterate on feedback, and keep a durable review trail.

## What you can build with it

- Agent-generated architecture reviews
- Incident reports and postmortems
- Product specs and launch plans
- Data analysis reports
- Design proposals with Mermaid/SVG diagrams
- QA findings and browser-test reports
- Internal dashboards captured as portable HTML

## Features

- One self-contained HTML file per document
- Browser review UI with sandboxed rendering
- Anchored comments and resolve/reopen workflow
- Immutable versions with optimistic concurrency
- Google OAuth for human users
- Native service accounts for headless/agent workflows
- Go CLI with compact default output and JSON mode
- Generated OpenAPI/Swagger docs
- Self-hostable Go server with embedded React frontend
- Dockerfile for container deployments

## Why one HTML file?

A single HTML artifact is easy for agents to generate, easy to version, easy to archive, and easy to move through a CLI. Images can be embedded with base64 data URLs, and diagrams can be Mermaid, inline SVG, or rendered HTML/CSS.

```html
<img src="data:image/png;base64,..." alt="embedded screenshot" />
```

No separate image hosting is required for v1.

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

## Run with Docker

Build the application container:

```bash
docker build -t aidocs:local .
```

Run it with Postgres, S3-compatible storage, and Google OAuth configured:

```bash
docker run --rm -p 8080:8080 \
  -e DATABASE_URL='postgres://user:pass@host.docker.internal:5432/aidocs?sslmode=disable' \
  -e BLOB_BUCKET='aidocs' \
  -e BLOB_REGION='us-east-1' \
  -e APP_ORIGIN='http://localhost:8080' \
  -e RENDER_ORIGIN='http://localhost:8080' \
  -e GOOGLE_OAUTH_CLIENT_ID='your-client-id' \
  -e GOOGLE_OAUTH_CLIENT_SECRET='your-client-secret' \
  -e SESSION_SECRET='replace-with-at-least-32-random-bytes' \
  aidocs:local
```

See [Self-hosting](docs/self-hosting.md) for production setup and all environment variables. A Kubernetes Helm chart is available in [`charts/aidocs`](charts/aidocs).

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
docs/      Self-hosting and operator docs
```

## API docs

When the server is running:

```text
/openapi.json
/api-docs
/commit.txt
```

## Security model

Uploaded HTML should be treated as untrusted content. `aidocs` renders documents in a sandboxed frame. For production, deploy with separate app and render origins when possible.

## License

Apache-2.0
