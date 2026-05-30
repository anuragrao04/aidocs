-- name: InsertServiceAccount :exec
INSERT INTO service_accounts(id,name,owner_user_id,created_by_user_id) VALUES($1,$2,$3,$3);

-- name: ListServiceAccounts :many
SELECT sa.id,sa.name,(sa.disabled_at IS NOT NULL)::bool AS disabled,u.id AS owner_id,u.email AS owner_email,u.name AS owner_name
FROM service_accounts sa JOIN users u ON u.id=sa.owner_user_id WHERE sa.owner_user_id=$1 ORDER BY sa.created_at DESC;

-- name: UpdateServiceAccountName :exec
UPDATE service_accounts SET name=$1 WHERE id=$2;

-- name: DisableServiceAccount :exec
UPDATE service_accounts SET disabled_at=COALESCE(disabled_at,now()) WHERE id=$1;

-- name: EnableServiceAccount :exec
UPDATE service_accounts SET disabled_at=NULL WHERE id=$1;

-- name: InsertServiceAccountKey :exec
INSERT INTO service_account_keys(id,service_account_id,name,token_hash) VALUES($1,$2,$3,$4);

-- name: ListServiceAccountKeys :many
SELECT id,name FROM service_account_keys WHERE service_account_id=$1 AND revoked_at IS NULL ORDER BY created_at DESC;

-- name: RevokeServiceAccountKey :exec
UPDATE service_account_keys SET revoked_at=now() WHERE service_account_id=$1 AND id=$2;

-- name: ServiceAccountExists :one
SELECT EXISTS(SELECT 1 FROM service_accounts WHERE id = $1)::bool;

-- name: GetServiceAccount :one
SELECT sa.id, sa.name, (sa.disabled_at IS NOT NULL)::bool AS disabled, u.id AS owner_id, u.email AS owner_email, u.name AS owner_name
FROM service_accounts sa
JOIN users u ON u.id = sa.owner_user_id
WHERE sa.id = $1;

-- name: GetServiceAccountByName :one
SELECT sa.id, sa.name, (sa.disabled_at IS NOT NULL)::bool AS disabled, u.id AS owner_id, u.email AS owner_email, u.name AS owner_name
FROM service_accounts sa
JOIN users u ON u.id = sa.owner_user_id
WHERE sa.name = $1;
