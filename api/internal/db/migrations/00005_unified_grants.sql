-- +goose Up
-- Unify document visibility into the grants ACL. The "everyone on this server"
-- audience becomes a resource_grants row with principal_type='anyone' and an
-- empty principal_id, so access resolution has a single code path. The legacy
-- documents.visibility column is then redundant and dropped.

-- Convert legacy visibility values into explicit "anyone" grants. Both 'link'
-- and 'org' meant "everyone who can reach this server" (org servers gate login
-- to org members), so both map to an anyone:viewer grant.
INSERT INTO resource_grants(id, resource_type, resource_id, principal_type, principal_id, role, granted_by_user_id)
SELECT 'gr_legacy_' || d.id, 'document', d.id, 'anyone', '', 'viewer', d.owner_id
FROM documents d
WHERE d.visibility IN ('link', 'org')
ON CONFLICT (resource_type, resource_id, principal_type, principal_id) DO NOTHING;

ALTER TABLE documents DROP COLUMN visibility;

-- +goose Down
ALTER TABLE documents ADD COLUMN visibility TEXT NOT NULL DEFAULT 'private';

UPDATE documents d
SET visibility = 'link'
WHERE EXISTS (
  SELECT 1 FROM resource_grants g
  WHERE g.resource_type = 'document' AND g.resource_id = d.id
    AND g.principal_type = 'anyone' AND g.revoked_at IS NULL
);

DELETE FROM resource_grants WHERE principal_type = 'anyone';
