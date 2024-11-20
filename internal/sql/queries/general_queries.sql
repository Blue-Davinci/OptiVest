
-- name: CreateContactUs :one
INSERT INTO contact_us(
    user_id,
    name,
    email,
    subject,
    message
) VALUES ($1, $2, $3, $4, $5) 
RETURNING id,status,created_at,updated_at;