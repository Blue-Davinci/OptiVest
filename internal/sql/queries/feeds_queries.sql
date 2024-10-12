
-- name: CreateNewFeed :one
INSERT INTO feeds (
    user_id,
    name, 
    url, 
    img_url,
    feed_type, 
    feed_category, 
    feed_description,
    is_hidden
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
)
RETURNING id, created_at, updated_at, version, approval_status;

-- name: UpdateFeed :one
UPDATE feeds
SET
    name = $2,
    url = $3,
    img_url = $4,
    feed_type = $5,
    feed_category = $6,
    feed_description = $7,
    is_hidden = $8,
    approval_status = $9,
    version = version + 1
WHERE id = $1 AND user_id = $10 AND version = $11
RETURNING updated_at, version;

-- name: GetFeedByID :one
SELECT
    id,
    user_id,
    name,
    url,
    img_url,
    feed_type,
    feed_category,
    feed_description,
    is_hidden,
    approval_status,
    version,
    created_at,
    updated_at
FROM feeds
WHERE id = $1;

-- name: GetNextFeedsToFetch :many
SELECT
    id,
    user_id,
    name,
    url,
    img_url,
    feed_type,
    feed_category,
    feed_description,
    is_hidden,
    approval_status,
    version,
    created_at,
    updated_at
FROM feeds
WHERE approval_status = 'approved'
ORDER BY last_fetched_at ASC NULLS FIRST
LIMIT $1;

-- name: GetAllFeeds :many
SELECT count(*) OVER() AS total_count, 
    id, 
    user_id,
    name, 
    url, 
    img_url,
    feed_type,
    feed_category,
    feed_description,
    is_hidden,
    approval_status,
    version,
    created_at, 
    updated_at 
FROM feeds
WHERE ($1 = '' OR to_tsvector('simple', name) @@ plainto_tsquery('simple', $1))
AND feed_type = $2 OR $2 = ''
AND is_hidden = FALSE
AND approval_status='approved'
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- name: DeleteFeedByID :one
DELETE FROM feeds
WHERE id = $1 AND user_id = $2
RETURNING id;

-- name: MarkFeedAsFetched :one
UPDATE feeds
SET last_fetched_at = NOW()
WHERE id = $1
RETURNING *;

-- name: CreateRssFeedPost :one
INSERT INTO rssfeed_posts (
    channeltitle, 
    channelurl,
    channeldescription,
    channellanguage,
    itemtitle,
    itemdescription,
    itempublished_at, 
    itemcontent,
    itemurl, 
    img_url, 
    feed_id
)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9, $10, $11)
RETURNING id, created_at, updated_at;

-- name: GetRssFeedPostByID :one
SELECT
    id,
    channeltitle,
    channelurl,
    channeldescription,
    channellanguage,
    itemtitle,
    itemdescription,
    itempublished_at,
    itemcontent,
    itemurl,
    img_url,
    feed_id,
    created_at,
    updated_at
FROM rssfeed_posts
WHERE id = $1;

-- name: GetAllRSSPostWithFavoriteTag :many
SELECT 
    COUNT(*) OVER() AS total_count,
    p.id,
    p.created_at,
    p.updated_at,
    p.channeltitle,
    p.channelurl,
    p.channeldescription,
    p.channellanguage,
    p.itemtitle,
    p.itemdescription,
    p.itemcontent,
    p.itempublished_at,
    p.itemurl,
    p.img_url,
    p.feed_id,
    CASE WHEN fp.post_id IS NOT NULL THEN true ELSE false END AS is_favorite  -- Check if the post is favorited
FROM 
    rssfeed_posts p
LEFT JOIN 
    favorite_posts fp ON p.id = fp.post_id AND fp.user_id = $1  -- Check if the post is in the favorites table for the user
WHERE 
    ($2 = '' OR to_tsvector('simple', p.itemtitle) @@ plainto_tsquery('simple', $2))  -- Full-text search for item title
    AND ($3 = 0 OR p.feed_id = $3)  -- Filter by feed_id if provided, return all posts if feed_id is NULL
ORDER BY 
    p.created_at DESC
LIMIT $4 OFFSET $5;


-- name: CreateNewFavoriteOnPost :one
INSERT INTO favorite_posts (
    post_id, 
    feed_id, 
    user_id
    )
VALUES ($1, $2, $3)
RETURNING id, created_at;

-- name: DeleteFavoriteOnPost :one
DELETE FROM favorite_posts
WHERE post_id = $1 AND user_id = $2
RETURNING id;
