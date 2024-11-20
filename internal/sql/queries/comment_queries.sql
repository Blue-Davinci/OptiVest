-- name: CreateNewComment :one
INSERT INTO comments (
    content,
    user_id,
    parent_id,
    associated_type,
    associated_id
    )
VALUES ($1, $2, $3, $4, $5)
RETURNING id, created_at, updated_at, version;

-- name: UpdateComment :one
UPDATE comments
SET content = $1, version = version + 1
WHERE id = $2 AND user_id = $3 AND version = $4
RETURNING updated_at, version;

-- name: GetCommentById :one
SELECT 
    id,
    content,
    user_id,
    parent_id,
    associated_type,
    associated_id,
    created_at,
    updated_at,
    version
FROM comments
WHERE id = $1 AND user_id = $2;

-- name: DeleteComment :one
DELETE FROM comments
WHERE id = $1 AND user_id = $2
RETURNING id;