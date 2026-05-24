-- +goose Up
CREATE TABLE users (
  id TEXT PRIMARY KEY,
  email TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL DEFAULT '',
  picture_url TEXT NOT NULL DEFAULT '',
  google_sub TEXT UNIQUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE documents (
  id TEXT PRIMARY KEY,
  title TEXT NOT NULL,
  owner_id TEXT NOT NULL REFERENCES users(id),
  visibility TEXT NOT NULL DEFAULT 'private',
  current_version_id TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE versions (
  id TEXT PRIMARY KEY,
  document_id TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
  number INTEGER NOT NULL,
  html_blob_key TEXT NOT NULL,
  sha256 TEXT NOT NULL,
  parent_version_id TEXT REFERENCES versions(id),
  created_by_type TEXT NOT NULL,
  created_by_id TEXT NOT NULL,
  change_summary TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(document_id, number)
);

ALTER TABLE documents
  ADD CONSTRAINT documents_current_version_fk
  FOREIGN KEY (current_version_id) REFERENCES versions(id) DEFERRABLE INITIALLY DEFERRED;

CREATE TABLE service_accounts (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL UNIQUE,
  owner_user_id TEXT NOT NULL REFERENCES users(id),
  created_by_user_id TEXT NOT NULL REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  disabled_at TIMESTAMPTZ
);

CREATE TABLE service_account_keys (
  id TEXT PRIMARY KEY,
  service_account_id TEXT NOT NULL REFERENCES service_accounts(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  token_hash TEXT NOT NULL UNIQUE,
  last_used_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  revoked_at TIMESTAMPTZ
);

CREATE TABLE cli_credentials (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  token_hash TEXT NOT NULL UNIQUE,
  last_used_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  revoked_at TIMESTAMPTZ
);

CREATE TABLE resource_grants (
  id TEXT PRIMARY KEY,
  resource_type TEXT NOT NULL,
  resource_id TEXT NOT NULL,
  principal_type TEXT NOT NULL,
  principal_id TEXT NOT NULL,
  role TEXT NOT NULL,
  granted_by_user_id TEXT NOT NULL REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  revoked_at TIMESTAMPTZ,
  UNIQUE(resource_type, resource_id, principal_type, principal_id)
);

CREATE TABLE comments (
  id TEXT PRIMARY KEY,
  document_id TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
  created_on_version_id TEXT NOT NULL REFERENCES versions(id),
  author_type TEXT NOT NULL,
  author_id TEXT NOT NULL,
  author_email TEXT NOT NULL DEFAULT '',
  author_name TEXT NOT NULL DEFAULT '',
  body TEXT NOT NULL,
  selected_text TEXT NOT NULL,
  anchor_json JSONB NOT NULL,
  status TEXT NOT NULL DEFAULT 'open',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  resolved_at TIMESTAMPTZ
);

CREATE TABLE comment_placements (
  id TEXT PRIMARY KEY,
  comment_id TEXT NOT NULL REFERENCES comments(id) ON DELETE CASCADE,
  version_id TEXT NOT NULL REFERENCES versions(id) ON DELETE CASCADE,
  status TEXT NOT NULL,
  anchor_json JSONB,
  matched_text TEXT,
  confidence DOUBLE PRECISION NOT NULL DEFAULT 1.0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(comment_id, version_id)
);

CREATE TABLE ownership_transfers (
  id TEXT PRIMARY KEY,
  service_account_id TEXT NOT NULL REFERENCES service_accounts(id) ON DELETE CASCADE,
  from_user_id TEXT NOT NULL REFERENCES users(id),
  to_user_id TEXT NOT NULL REFERENCES users(id),
  initiated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  accepted_at TIMESTAMPTZ,
  status TEXT NOT NULL
);

CREATE INDEX idx_versions_document_id ON versions(document_id);
CREATE INDEX idx_comments_document_id ON comments(document_id);
CREATE INDEX idx_grants_resource ON resource_grants(resource_type, resource_id) WHERE revoked_at IS NULL;
CREATE INDEX idx_grants_principal ON resource_grants(principal_type, principal_id) WHERE revoked_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS ownership_transfers;
DROP TABLE IF EXISTS comment_placements;
DROP TABLE IF EXISTS comments;
DROP TABLE IF EXISTS resource_grants;
DROP TABLE IF EXISTS cli_credentials;
DROP TABLE IF EXISTS service_account_keys;
DROP TABLE IF EXISTS service_accounts;
ALTER TABLE IF EXISTS documents DROP CONSTRAINT IF EXISTS documents_current_version_fk;
DROP TABLE IF EXISTS versions;
DROP TABLE IF EXISTS documents;
DROP TABLE IF EXISTS users;
