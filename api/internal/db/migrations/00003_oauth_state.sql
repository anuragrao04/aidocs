-- +goose Up
CREATE TABLE oauth_states (
  id TEXT NOT NULL,
  kind TEXT NOT NULL,
  state_json JSONB NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (kind, id)
);

CREATE INDEX idx_oauth_states_expires_at ON oauth_states(expires_at);

-- +goose Down
DROP TABLE IF EXISTS oauth_states;
