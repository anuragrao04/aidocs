# Self-hosting aidocs

This guide runs `aidocs` as a single application container. The container serves the API and the embedded React UI. You provide Postgres, S3-compatible blob storage, and Google OAuth credentials.

## Requirements

- Postgres 14+
- S3-compatible object storage
- Google OAuth web client
- A public HTTPS origin for production

For production, use separate app and render origins when possible:

```text
APP_ORIGIN=https://aidocs.example.com
RENDER_ORIGIN=https://render.aidocs.example.com
```

Using a separate render origin further isolates untrusted uploaded HTML from the authenticated application.

## Build the container

```bash
docker build -t aidocs:local .
```

## Configure Google OAuth

Create a Google OAuth web client and add this callback URL:

```text
https://aidocs.example.com/v1/auth/google/callback
```

For local testing, add:

```text
http://localhost:8080/v1/auth/google/callback
```

## Run locally with Docker

This starts only the aidocs app container. Point it at your existing local or hosted Postgres and S3-compatible storage.

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
  aidocs:local
```

Open:

```text
http://localhost:8080
```

## Required environment variables

```text
DATABASE_URL
BLOB_BUCKET
APP_ORIGIN
RENDER_ORIGIN
GOOGLE_OAUTH_CLIENT_ID
GOOGLE_OAUTH_CLIENT_SECRET
SESSION_SECRET
```

## Optional environment variables

```text
AIDOCS_ADDR=:8080
AIDOCS_MIGRATE=true
AIDOCS_COMMIT_SHA=...
COMMIT_SHA=...
ALLOWED_OAUTH_DOMAINS=example.com,company.com
BLOB_REGION=us-east-1
BLOB_ENDPOINT=https://...
BLOB_ACCESS_KEY_ID=...
BLOB_SECRET_ACCESS_KEY=...
BLOB_FORCE_PATH_STYLE=true
```

`AIDOCS_MIGRATE=false` disables automatic database migrations at startup.

`ALLOWED_OAUTH_DOMAINS` restricts Google login to specific email domains.


## Deploy to Kubernetes with Helm

A Helm chart is available at `charts/aidocs`. It deploys one application container; you provide Postgres, S3-compatible blob storage, Google OAuth credentials, and ingress/TLS.

```bash
helm upgrade --install aidocs ./charts/aidocs \
  --namespace aidocs \
  --create-namespace \
  -f my-values.yaml
```

Start from:

```bash
cp charts/aidocs/values.yaml my-values.yaml
```

Then set `origins`, `database`, `blob`, `googleOAuth`, `session`, and `ingress`.

## CLI installation

Once releases are published, install the CLI with Homebrew:

```bash
brew install anuragrao04/tap/aidocs
```

Until then, build locally:

```bash
git clone https://github.com/anuragrao04/aidocs.git
cd aidocs
make build-cli
./bin/aidocs auth login https://aidocs.example.com
```

## Health and metadata endpoints

```text
GET /openapi.json
GET /api-docs
GET /commit.txt
```

Set `AIDOCS_COMMIT_SHA` or `COMMIT_SHA` in your deployment to expose the running revision through `/commit.txt`.

## Storage notes

`aidocs` stores uploaded HTML versions in blob storage and metadata/comments/auth state in Postgres. Documents are versioned immutably; each push creates a new version.

For v1, each document should be one self-contained HTML file. If a document needs images, embed them as data URLs, for example:

```html
<img src="data:image/png;base64,..." alt="diagram" />
```
