# Self-hosting aidocs

`aidocs` ships as a single Go binary that embeds the web UI. You provide
Postgres, S3-compatible blob storage, and Google OAuth credentials.

This guide covers three deployment paths:

- [Docker](#docker)
- [Kubernetes with Helm](#kubernetes-with-helm)
- [Bare metal / systemd](#bare-metal--systemd)

It also documents [every environment variable](#environment-variables)
the binary understands.

## Requirements

- Postgres 14+
- S3-compatible object storage (AWS S3, MinIO, Cloudflare R2, etc.)
- A Google OAuth web client
- A public HTTPS origin for production

For production, use **separate app and render origins** when possible:

```text
APP_ORIGIN=https://aidocs.example.com
RENDER_ORIGIN=https://render.aidocs.example.com
```

The render origin serves uploaded HTML inside a sandboxed iframe. Putting
it on a different host isolates untrusted user-supplied HTML from your
authenticated application origin.

## Configure Google OAuth

Create an OAuth web client in Google Cloud and add this callback URL:

```text
https://aidocs.example.com/v1/auth/google/callback
```

For local testing add:

```text
http://localhost:8080/v1/auth/google/callback
```

Optionally restrict logins to specific email domains with
`ALLOWED_OAUTH_DOMAINS=example.com,company.com`.

### Deployment type: public vs org

aidocs runs in one of two modes, set by `AIDOCS_DEPLOYMENT`:

- **`public`** (default) — anyone with a Google account can sign in. Sharing a
  document with "everyone" means *anyone with the link*.
- **`org`** — Google login is gated to your organization's domains
  (`AIDOCS_ORG_DOMAINS`, a comma-separated list that supports multiple domains
  for acquisitions). Sharing with "everyone" then means *anyone in the org*,
  because only org members can authenticate. Set `AIDOCS_ORG_NAME` to label the
  org in the UI (e.g. "Anyone in Acme").

On an org deployment the org domains are the login gate, so you do not also
need `ALLOWED_OAUTH_DOMAINS`. The document permission model is identical in both
modes; only the login gate and the wording differ.

```text
AIDOCS_DEPLOYMENT=org
AIDOCS_ORG_DOMAINS=acme.com,acme-labs.io
AIDOCS_ORG_NAME=Acme
```

## Docker

Build the image from source, or pull the prebuilt image from GHCR.

```bash
# Build locally
docker build -t aidocs:local .

# Or pull a tagged release
docker pull ghcr.io/anuragrao04/aidocs:latest
```

Run it, pointing at your Postgres and S3-compatible storage:

```bash
docker run --rm -p 8080:8080 \
  -e DATABASE_URL='postgres://user:pass@host.docker.internal:5432/aidocs?sslmode=disable' \
  -e BLOB_BUCKET='aidocs' \
  -e BLOB_REGION='us-east-1' \
  -e BLOB_ENDPOINT='http://host.docker.internal:9000' \
  -e BLOB_ACCESS_KEY_ID='minioadmin' \
  -e BLOB_SECRET_ACCESS_KEY='minioadmin' \
  -e BLOB_FORCE_PATH_STYLE='true' \
  -e APP_ORIGIN='http://localhost:8080' \
  -e RENDER_ORIGIN='http://localhost:8080' \
  -e GOOGLE_OAUTH_CLIENT_ID='your-client-id' \
  -e GOOGLE_OAUTH_CLIENT_SECRET='your-client-secret' \
  -e SESSION_SECRET='replace-with-at-least-32-random-bytes' \
  ghcr.io/anuragrao04/aidocs:latest
```

Open <http://localhost:8080>.

For production, sit aidocs behind a TLS-terminating reverse proxy (nginx,
Caddy, Cloudflare, ALB, etc.) and set `APP_ORIGIN` / `RENDER_ORIGIN` to
the public HTTPS URLs.

## Kubernetes with Helm

A Helm chart lives at [`charts/aidocs/`](../charts/aidocs/). It deploys
one Deployment, a Service, optional Ingress, and a PodDisruptionBudget.
You bring your own Postgres and S3.

### Quick start

```bash
helm upgrade --install aidocs ./charts/aidocs \
  --namespace aidocs \
  --create-namespace \
  -f my-values.yaml
```

Start by copying the bundled values file:

```bash
cp charts/aidocs/values.yaml my-values.yaml
```

### Minimal `my-values.yaml`

```yaml
image:
  repository: ghcr.io/anuragrao04/aidocs
  tag: "0.1.0"

origins:
  app: https://aidocs.example.com
  render: https://render.aidocs.example.com

googleOAuth:
  clientID: "your-google-client-id"
  clientSecret: "your-google-client-secret"

session:
  # At least 32 random bytes. Generate with: openssl rand -hex 32
  secret: "replace-with-at-least-32-random-bytes"

database:
  url: "postgres://aidocs:pass@postgres.aidocs.svc:5432/aidocs?sslmode=require"

blob:
  bucket: "aidocs-prod"
  region: "us-east-1"
  accessKeyID: "AKIA..."
  secretAccessKey: "..."

ingress:
  enabled: true
  className: "nginx"
  hosts:
    - host: aidocs.example.com
      paths:
        - path: /
          pathType: Prefix
    - host: render.aidocs.example.com
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: aidocs-tls
      hosts:
        - aidocs.example.com
        - render.aidocs.example.com

allowedOAuthDomains: "example.com"
```

### Using existing Secrets

To avoid putting credentials in `values.yaml`, reference pre-created
Kubernetes Secrets via the `existingSecret` fields:

```yaml
googleOAuth:
  existingSecret: aidocs-google-oauth
  clientIDKey: client-id
  clientSecretKey: client-secret

session:
  existingSecret: aidocs-session
  secretKey: secret

database:
  existingSecret: aidocs-db
  urlKey: url

blob:
  bucket: aidocs-prod
  region: us-east-1
  existingSecret: aidocs-blob
  accessKeyIDKey: access-key-id
  secretAccessKeyKey: secret-access-key
```

Then create those Secrets in your cluster with `kubectl create secret`
or your preferred secrets manager (External Secrets Operator, SOPS,
Sealed Secrets, etc.).

### Useful chart values

| Path | Purpose |
|------|---------|
| `replicaCount` | Number of pod replicas. |
| `image.{repository,tag,pullPolicy}` | Container image. |
| `origins.{app,render}` | Public URLs. `app` is injected into the served frontend. |
| `database.{url,migrate}` | Postgres DSN; `migrate: false` skips startup migrations. |
| `blob.{bucket,region,endpoint,forcePathStyle,accessKeyID,secretAccessKey}` | S3 settings. |
| `googleOAuth.{clientID,clientSecret}` | OAuth credentials. |
| `session.secret` | At least 32 random bytes. |
| `allowedOAuthDomains` | Comma-separated email domain allowlist. |
| `commitSha` | Optional revision surfaced at `/commit.txt`. |
| `ingress` | Standard ingress block; supports multiple hosts. |
| `extraEnv` / `extraEnvFrom` | Append arbitrary env vars / envFrom sources. |
| `resources` | CPU/memory requests and limits. |
| `livenessProbe` / `readinessProbe` | HTTP probes against `/commit.txt` and `/openapi.json`. |
| `podDisruptionBudget` | Optional PDB for safer upgrades. |

See [`charts/aidocs/values.yaml`](../charts/aidocs/values.yaml) for the
full schema with defaults.

## Bare metal / systemd

```bash
git clone https://github.com/anuragrao04/aidocs.git
cd aidocs
make build              # produces bin/aidocs-server and bin/aidocs

# Place the binary and create an env file
sudo install -m 0755 bin/aidocs-server /usr/local/bin/aidocs-server
sudo install -m 0640 -o root -g root /dev/stdin /etc/aidocs/env <<'EOF'
DATABASE_URL=postgres://aidocs:pass@localhost:5432/aidocs?sslmode=disable
BLOB_BUCKET=aidocs-prod
BLOB_REGION=us-east-1
BLOB_ACCESS_KEY_ID=AKIA...
BLOB_SECRET_ACCESS_KEY=...
APP_ORIGIN=https://aidocs.example.com
RENDER_ORIGIN=https://render.aidocs.example.com
GOOGLE_OAUTH_CLIENT_ID=...
GOOGLE_OAUTH_CLIENT_SECRET=...
SESSION_SECRET=...
EOF

sudo tee /etc/systemd/system/aidocs.service > /dev/null <<'EOF'
[Unit]
Description=aidocs
After=network.target

[Service]
EnvironmentFile=/etc/aidocs/env
ExecStart=/usr/local/bin/aidocs-server
Restart=on-failure
DynamicUser=true

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable --now aidocs
```

Terminate TLS at nginx, Caddy, or any reverse proxy of your choice.

## Environment variables

### Required

| Variable | Purpose |
|----------|---------|
| `DATABASE_URL` | Postgres connection string. Auto-runs migrations at startup unless disabled. |
| `BLOB_BUCKET` | S3-compatible bucket name for stored HTML versions. |
| `APP_ORIGIN` | Public URL where the web UI is served. Also injected into the served `index.html` so the frontend can render correct copy-paste commands (CLI, curl, agent prompts). |
| `RENDER_ORIGIN` | Public URL where uploaded HTML is sandbox-rendered. May equal `APP_ORIGIN` for non-production. |
| `GOOGLE_OAUTH_CLIENT_ID` | Google OAuth web client ID. |
| `GOOGLE_OAUTH_CLIENT_SECRET` | Google OAuth web client secret. |
| `SESSION_SECRET` | Session signing key. At least 32 random bytes. |

### Optional

| Variable | Default | Purpose |
|----------|---------|---------|
| `AIDOCS_ADDR` | `:8080` | TCP listen address. |
| `AIDOCS_MIGRATE` | `true` | Run database migrations on boot. Set `false` to skip. |
| `AIDOCS_COMMIT_SHA` / `COMMIT_SHA` | unset | Build revision exposed at `/commit.txt`. |
| `ALLOWED_OAUTH_DOMAINS` | unset | Comma-separated email-domain allowlist for OAuth logins (public deployments). |
| `AIDOCS_DEPLOYMENT` | `public` | `public` or `org`. An org deployment gates login to `AIDOCS_ORG_DOMAINS`. |
| `AIDOCS_ORG_DOMAINS` | unset | Comma-separated org domains. Required when `AIDOCS_DEPLOYMENT=org`; also the login gate. |
| `AIDOCS_ORG_NAME` | unset | Org display name used in share UI copy (e.g. "Anyone in Acme"). |
| `BLOB_REGION` | `us-east-1` | AWS region for the S3 client. |
| `BLOB_ENDPOINT` | unset | Custom S3 endpoint (MinIO, R2, etc.). |
| `BLOB_ACCESS_KEY_ID` | unset | Override AWS-SDK default credential chain. |
| `BLOB_SECRET_ACCESS_KEY` | unset | Override AWS-SDK default credential chain. |
| `BLOB_FORCE_PATH_STYLE` | `false` | Set to `true` for MinIO and most non-AWS S3 implementations. |

If `BLOB_ACCESS_KEY_ID` and `BLOB_SECRET_ACCESS_KEY` are omitted, the
AWS SDK default credential chain is used (IRSA, instance profile,
shared config, etc.).

## Health and metadata endpoints

```text
GET /openapi.json              OpenAPI 3 spec
GET /api-docs                  Swagger UI
GET /commit.txt                Build revision (controlled by AIDOCS_COMMIT_SHA)
GET /v1/health                 Liveness JSON: {"ok": true}
GET /.well-known/aidocs.json   Deployment discovery
GET /onboarding/sample.html    Bundled sample report used by the in-app
                               setup guide. Downloadable, no auth.
```

## How the frontend gets its public URL

When the server returns `index.html`, it substitutes the `APP_ORIGIN`
value into a placeholder so the React app can read it synchronously at
boot. This drives copy-paste-correct commands in the UI:

- The `curl` in the **Try it end-to-end** step of the setup guide.
- The `curl` examples on the **Developers** page.
- The bearer-token CLI snippet in the **Service accounts** detail
  panel.

No extra API call is made; the value comes from the same env you already
configured for the server.

## Storage notes

`aidocs` stores uploaded HTML versions in blob storage and metadata,
comments, grants, sessions, and OAuth state in Postgres. Documents are
versioned immutably; each push creates a new version.

For v1, each document should be one self-contained HTML file. If a
document needs images, embed them as data URLs:

```html
<img src="data:image/png;base64,..." alt="diagram" />
```

## Upgrading

Postgres migrations are applied automatically at startup unless
`AIDOCS_MIGRATE=false`. The chart and Docker image both honor this. To
roll back, deploy the previous image tag and pre-apply any matching
down-migration manually if needed; the binary does not auto-rollback.

## Production checklist

- [ ] `APP_ORIGIN` and `RENDER_ORIGIN` on different hostnames.
- [ ] HTTPS everywhere (TLS at the proxy/ingress).
- [ ] `SESSION_SECRET` rotated to a fresh 32+ byte value, stored in a
      secret manager.
- [ ] Google OAuth callback URLs match `APP_ORIGIN`.
- [ ] `ALLOWED_OAUTH_DOMAINS` set if you don't want public sign-ups.
- [ ] Postgres backups configured.
- [ ] Blob storage versioning / lifecycle rules configured.
- [ ] `AIDOCS_COMMIT_SHA` baked into your image so `/commit.txt`
      reports the running build.
- [ ] Monitoring scraping `/metrics` (Prometheus exposition).
- [ ] PodDisruptionBudget enabled (`podDisruptionBudget.enabled: true`)
      for at least 1 minAvailable.

## CLI installation for users

Once you've shipped a release, users (and their agents) install the CLI
with:

```bash
brew install anuragrao04/tap/aidocs
aidocs auth login https://aidocs.example.com
```

For self-hosted instances, pass the URL to `aidocs auth login` so the
CLI knows which server to authenticate against.
