-- name: CreateNewToken :one
INSERT INTO tokens (hash, user_id, expiry, scope)
VALUES ($1, $2, $3, $4)
RETURNING user_id;

-- name: DeletAllTokensForUser :exec
DELETE FROM tokens
WHERE scope = $1 AND user_id = $2;

-- name: GetForToken :one
SELECT
    users.id,
    users.first_name,
    users.last_name,
    users.email,
    users.profile_avatar_url,
    users.password,
    users.user_role,
    users.phone_number,
    users.activated,
    users.version,
    users.created_at,
    users.updated_at,
    users.last_login,
    users.profile_completed,
    users.dob,
    users.address,
    users.country_code,
    users.currency_code,
    users.mfa_enabled,
    users.mfa_secret,
    users.mfa_status,
    users.mfa_last_checked
FROM users
INNER JOIN tokens
ON users.id = tokens.user_id
WHERE tokens.hash = $1
AND tokens.scope = $2
AND tokens.expiry > $3;