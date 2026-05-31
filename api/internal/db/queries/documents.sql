-- name: GetDocumentOwnerID :one
SELECT owner_id FROM documents WHERE id=$1;

-- name: GetDocumentGrantRole :one
SELECT role FROM resource_grants WHERE resource_type='document' AND resource_id=$1 AND principal_type=$2 AND principal_id=$3 AND revoked_at IS NULL;

-- name: InsertDocument :exec
INSERT INTO documents(id,title,owner_id) VALUES($1,$2,$3);

-- name: UpdateDocumentCurrentVersion :exec
UPDATE documents SET current_version_id=$1, updated_at=now() WHERE id=$2;

-- name: GetDocument :one
SELECT d.id,d.title,d.current_version_id,u.id AS owner_id,u.email AS owner_email,u.name AS owner_name
FROM documents d JOIN users u ON u.id=d.owner_id WHERE d.id=$1;

-- name: GetDocumentVersionForUpdate :one
SELECT current_version_id, COALESCE((SELECT max(number)+1 FROM versions WHERE document_id=$1),1)::int AS next_number
FROM documents WHERE id=$1 FOR UPDATE;

-- name: RecordWorkspaceEntry :exec
-- Records that a principal has opened a document, so a broadly-shared ("anyone")
-- document joins their workspace listing after first open. Idempotent.
INSERT INTO document_workspace_entries(document_id,principal_type,principal_id)
VALUES($1,$2,$3)
ON CONFLICT(document_id,principal_type,principal_id) DO NOTHING;

-- name: ListDocuments :many
-- A document appears in a principal's workspace when they:
--   1. own it, or
--   2. hold an explicit (user/service-account) grant for it, or
--   3. have opened it (a workspace entry exists) AND it is still broadly shared
--      via an active "anyone" grant.
-- A bare "anyone" grant alone does NOT list the document: a public link grants
-- access via the link, it does not place the document in every user's
-- workspace until they open it. Link access is authorized separately via the
-- per-document role resolution.
SELECT d.id,d.title,d.current_version_id,u.id AS owner_id,u.email AS owner_email,u.name AS owner_name
FROM documents d JOIN users u ON u.id=d.owner_id
WHERE (d.owner_id=$1 AND $2='user')
   OR EXISTS (
       SELECT 1 FROM resource_grants g
       WHERE g.resource_type='document'
         AND g.resource_id=d.id
         AND g.revoked_at IS NULL
         AND g.principal_type=$2 AND g.principal_id=$1
   )
   OR (
       EXISTS (
           SELECT 1 FROM document_workspace_entries w
           WHERE w.document_id=d.id
             AND w.principal_type=$2 AND w.principal_id=$1
       )
       AND EXISTS (
           SELECT 1 FROM resource_grants g
           WHERE g.resource_type='document'
             AND g.resource_id=d.id
             AND g.revoked_at IS NULL
             AND g.principal_type='anyone'
       )
   )
ORDER BY d.updated_at DESC;

-- name: UpdateDocumentTitle :exec
UPDATE documents SET title=$1,updated_at=now() WHERE id=$2;

-- name: DeleteDocument :exec
DELETE FROM documents WHERE id=$1;
