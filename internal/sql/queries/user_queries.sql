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
