
-- name: CreateNewUserAward :one
INSERT INTO user_awards (user_id, award_id)
VALUES ($1, $2)
RETURNING created_at;

-- name: GetAllAwardsForUserByID :many
SELECT a.*
FROM awards a
INNER JOIN user_awards ua ON ua.award_id = a.id
WHERE ua.user_id = $1;

-- name: GetAllAwards :many
SELECT 
    id,
    code,
    description,
    award_image_url,
    points,
    created_at,
    updated_at
FROM awards;

