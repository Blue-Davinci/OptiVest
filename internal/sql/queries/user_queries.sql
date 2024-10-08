-- name: CreateNewUser :one
INSERT INTO users (
    first_name,
    last_name,
    email,
    profile_avatar_url,
    password,
    phone_number,
    profile_completed,
    dob,
    address,
    country_code
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
)
RETURNING id, created_at, updated_at, role_level, last_login, version, mfa_enabled, mfa_secret, mfa_status, mfa_last_checked;

-- name: UpdateUser :one
UPDATE users
SET
    first_name = $1,
    last_name = $2,
    email = $3,
    profile_avatar_url = $4,
    password = $5,
    role_level = $6,
    phone_number = $7,
    activated = $8,
    version = version + 1,
    updated_at = NOW(),
    last_login = $9,
    profile_completed = $10,
    dob = $11,
    address = $12,
    country_code = $13,
    currency_code = $14,
    mfa_enabled = $15,
    mfa_secret = $16,
    mfa_status = $17,
    mfa_last_checked = $18,
    risk_tolerance = $19,
    time_horizon = $20
WHERE id = $21 AND version = $22
RETURNING updated_at, version;

-- name: GetUserByEmail :one
SELECT 
    id,
    first_name,
    last_name,
    email,
    profile_avatar_url,
    password,
    role_level,
    phone_number,
    activated,
    version,
    created_at,
    updated_at,
    last_login,
    profile_completed,
    dob,
    address,
    country_code,
    currency_code,
    mfa_enabled,
    mfa_secret,
    mfa_status,
    mfa_last_checked,
    risk_tolerance,
    time_horizon
FROM users
WHERE email = $1;
