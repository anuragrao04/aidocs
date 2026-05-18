# aidocs Helm chart

Deploy `aidocs` to Kubernetes as one application Deployment and Service. You bring Postgres, S3-compatible blob storage, and Google OAuth credentials.

## Install

```bash
helm upgrade --install aidocs ./charts/aidocs \
  --namespace aidocs \
  --create-namespace \
  -f values.yaml
```

## Minimal values

```yaml
image:
  repository: ghcr.io/anuragrao/aidocs
  tag: "0.1.0"

origins:
  app: https://aidocs.example.com
  render: https://render.aidocs.example.com

googleOAuth:
  clientID: "..."
  clientSecret: "..."

session:
  secret: "replace-with-at-least-32-random-bytes"

database:
  url: "postgres://user:pass@postgres:5432/aidocs?sslmode=require"

blob:
  bucket: aidocs
  region: us-east-1
  accessKeyID: "..."
  secretAccessKey: "..."

ingress:
  enabled: true
  className: nginx
  hosts:
    - host: aidocs.example.com
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: aidocs-tls
      hosts:
        - aidocs.example.com
```

## Existing secrets

You can keep secrets outside Helm values:

```yaml
googleOAuth:
  existingSecret: aidocs-google-oauth
  clientIDKey: client-id
  clientSecretKey: client-secret

session:
  existingSecret: aidocs-session
  secretKey: session-secret

database:
  existingSecret: aidocs-database
  urlKey: database-url

blob:
  existingSecret: aidocs-blob
  accessKeyIDKey: access-key-id
  secretAccessKeyKey: secret-access-key
```

## Google OAuth callback

Configure this redirect URI in Google Cloud:

```text
https://aidocs.example.com/v1/auth/google/callback
```
