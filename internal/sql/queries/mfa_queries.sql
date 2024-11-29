-- name: CreateNewRecoveryCode :one
INSERT INTO recovery_codes (user_id, code_hash)
VALUES ($1, $2)
RETURNING id, created_at;

-- name: GetRecoveryCodesByUserID :one
SELECT id, user_id, code_hash, used, created_at, updated_at
FROM recovery_codes
WHERE user_id = $1 AND used = FALSE;

-- name: DeleteRecoveryCodeByID :one
DELETE FROM recovery_codes
WHERE id = $1 AND user_id = $2
RETURNING id;

-- name: MarkRecoveryCodeAsUsed :one
UPDATE recovery_codes
SET used = TRUE
WHERE id = $1 AND user_id = $2
RETURNING id, updated_at;