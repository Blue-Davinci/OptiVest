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

-- name: CreateNewReaction :one
INSERT INTO comment_reactions (
    comment_id,
    user_id
)
VALUES ($1, $2)
RETURNING id,created_at, updated_at;

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

-- name: GetCommentsWithReactionsByAssociatedId :many
WITH comments_with_likes AS (
    SELECT 
        c.id AS comment_id,
        c.content,
        c.user_id,
        c.associated_type,
        c.associated_id,
        c.created_at,
        c.updated_at,
        c.parent_id,
        u.first_name,
        u.last_name,
        u.profile_avatar_url,
        COALESCE(gm.role, NULL) AS user_role, -- Role only if the type is 'group'
        COUNT(cr.id) AS likes_count, -- Total likes for the comment
        EXISTS (
            SELECT 1 
            FROM comment_reactions cr2 
            WHERE cr2.comment_id = c.id AND cr2.user_id = $3 -- Requesting user ID
        ) AS liked_by_requesting_user
    FROM 
        comments c
    JOIN 
        users u ON c.user_id = u.id
    LEFT JOIN 
        group_memberships gm ON gm.group_id = c.associated_id AND gm.user_id = u.id
    LEFT JOIN 
        comment_reactions cr ON cr.comment_id = c.id
    WHERE 
        c.associated_id = $1 -- Filter by associated_id
        AND c.associated_type = $2 -- Filter by associated_type ('group', 'feed')
        AND (
            c.associated_type != 'group' -- For non-group types, no membership check
            OR EXISTS (
                SELECT 1
                FROM group_memberships gm2
                WHERE gm2.group_id = c.associated_id 
                  AND gm2.user_id = $3 -- Requesting user ID
                  AND gm2.status = 'accepted' -- Check for approved membership
            )
        )
    GROUP BY 
        c.id, u.id, gm.role
)
SELECT * 
FROM comments_with_likes
ORDER BY 
    parent_id ASC NULLS FIRST, -- Ensure parent comments appear first
    created_at ASC;            -- Sort by creation time within each parent group



-- name: DeleteComment :one
DELETE FROM comments
WHERE id = $1 AND user_id = $2
RETURNING id;

-- name: DeleteReaction :one
DELETE FROM comment_reactions
WHERE comment_id = $1 AND user_id = $2
RETURNING id;