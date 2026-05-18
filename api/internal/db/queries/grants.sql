-- name: UpsertResourceGrant :one
INSERT INTO resource_grants(id,resource_type,resource_id,principal_type,principal_id,role,granted_by_user_id)
VALUES($1,'document',$2,$3,$4,$5,$6)
ON CONFLICT(resource_type,resource_id,principal_type,principal_id) DO UPDATE SET role=EXCLUDED.role, revoked_at=NULL
RETURNING id;

-- name: ListGrants :many
SELECT id,principal_type,principal_id,role,granted_by_user_id FROM resource_grants WHERE resource_type='document' AND resource_id=$1 AND revoked_at IS NULL;

-- name: UpdateGrantRole :one
UPDATE resource_grants SET role=$1 WHERE id=$2 AND resource_type='document' AND resource_id=$3 AND revoked_at IS NULL
RETURNING id,resource_id,principal_type,principal_id,role,granted_by_user_id;

-- name: GetGrant :one
SELECT id,resource_id,principal_type,principal_id,role,granted_by_user_id FROM resource_grants WHERE id=$1 AND resource_type='document' AND resource_id=$2 AND revoked_at IS NULL;

-- name: DeleteGrant :one
UPDATE resource_grants SET revoked_at=now() WHERE id=$1 AND resource_type='document' AND resource_id=$2 AND revoked_at IS NULL
RETURNING id;
