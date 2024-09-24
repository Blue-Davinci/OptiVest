-- name: CreateNewUser :one
INSERT INTO users (
    first_name, 
    last_name, 
    email, 
    password_hash, 
    phone_number, 
    activated, 
    profile_completed, 
    dob, 
    address, 
    country_code, 
    currency_code
) VALUES (
    $1,  -- First name
    $2,  -- Last name
    $3,  -- Email
    $4,  -- Password hash
    $5,  -- Phone number
    $6,  -- Activated
    $7,  -- Profile completed
    $8,  -- Date of birth (dob)
    $9, -- Address
    $10, -- Country code
    $11  -- Currency code
)
RETURNING id, created_at, updated_at, last_login version;

-- name: UpdateUser :one
UPDATE users
SET
    first_name = $1,
    last_name = $2,
    email = $3,
    profile_avatar_url = $4,
    password_hash = $5,
    phone_number = $6,
    activated = $7,
    version = version + 1,
    updated_at = NOW(),
    last_login = $8,
    profile_completed = $9,
    dob = $10,
    address = $11,
    country_code = $12,
    currency_code = $13
WHERE id = $14
RETURNING updated_at, version;

-- name: GetUserByEmail :one
SELECT 
    id,
    first_name,
    last_name,
    email,
    profile_avatar_url,
    password_hash,
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
    currency_code
FROM users
WHERE email = $1;
