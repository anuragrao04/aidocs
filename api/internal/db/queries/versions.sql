-- name: InsertInitialVersion :exec
INSERT INTO versions(id,document_id,number,html_blob_key,sha256,parent_version_id,created_by_type,created_by_id,change_summary)
VALUES($1,$2,1,$3,$4,NULL,$5,$6,'');

-- name: InsertVersion :exec
INSERT INTO versions(id,document_id,number,html_blob_key,sha256,parent_version_id,created_by_type,created_by_id,change_summary)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9);

-- name: ListVersions :many
SELECT id,number,document_id,created_by_type,created_by_id,change_summary,sha256 FROM versions WHERE document_id=$1 ORDER BY number;

-- name: GetVersion :one
SELECT id,number,document_id,created_by_type,created_by_id,change_summary,sha256 FROM versions WHERE id=$1;

-- name: GetVersionBlobKey :one
SELECT html_blob_key FROM versions WHERE id=$1;
