-- name: InsertOwnershipTransfer :exec
INSERT INTO ownership_transfers(id,service_account_id,from_user_id,to_user_id,status) VALUES($1,$2,$3,$4,'pending');

-- name: ListOwnershipTransfers :many
SELECT id,service_account_id,from_user_id,to_user_id,status FROM ownership_transfers WHERE from_user_id=$1 OR to_user_id=$1 ORDER BY initiated_at DESC;

-- name: GetOwnershipTransferForUpdate :one
SELECT id,service_account_id,from_user_id,to_user_id,status FROM ownership_transfers WHERE id=$1 FOR UPDATE;

-- name: UpdateServiceAccountOwner :exec
UPDATE service_accounts SET owner_user_id=$1 WHERE id=$2;

-- name: AcceptOwnershipTransfer :one
UPDATE ownership_transfers SET status='accepted',accepted_at=now() WHERE id=$1 AND status='pending'
RETURNING id;

-- name: DeclineOwnershipTransfer :one
UPDATE ownership_transfers SET status='declined' WHERE id=$1 AND status='pending' AND (from_user_id=$2 OR to_user_id=$2)
RETURNING id;
