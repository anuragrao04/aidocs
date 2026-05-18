-- name: CreateCLICredential :exec
INSERT INTO cli_credentials(id,user_id,name,token_hash) VALUES($1,$2,$3,$4);

-- name: ListCLICredentials :many
SELECT id,name FROM cli_credentials WHERE user_id=$1 AND revoked_at IS NULL ORDER BY created_at DESC;

-- name: RevokeCLICredential :exec
UPDATE cli_credentials SET revoked_at=now() WHERE id=$1 AND user_id=$2;
