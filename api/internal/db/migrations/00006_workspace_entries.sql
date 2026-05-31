-- +goose Up
-- A broad ("anyone") grant gives access via the link but does not, on its own,
-- place a document in every user's workspace. Instead, the first time a
-- principal opens such a document it is recorded here, so it then shows up in
-- their workspace listing -- mirroring "Shared with me" once you open a link.
-- Documents shared with a principal explicitly (a user/service-account grant)
-- are listed without needing an entry here.
CREATE TABLE document_workspace_entries (
    document_id    TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    principal_type TEXT NOT NULL,
    principal_id   TEXT NOT NULL,
    added_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (document_id, principal_type, principal_id)
);

CREATE INDEX idx_workspace_entries_principal
    ON document_workspace_entries (principal_type, principal_id);

-- +goose Down
DROP TABLE document_workspace_entries;
