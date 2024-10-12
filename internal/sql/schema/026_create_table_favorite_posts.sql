-- +goose Up
CREATE TABLE favorite_posts (
    id BIGSERIAL PRIMARY KEY,
    post_id BIGSERIAL UNIQUE NOT NULL,
    feed_id BIGSERIAL NOT NULL,
    user_id BIGSERIAL NOT NULL,
    created_at timestamp(0) with time zone NOT NULL DEFAULT now(),
    FOREIGN KEY (post_id) REFERENCES rssfeed_posts(id) ON DELETE CASCADE,
    FOREIGN KEY (feed_id) REFERENCES feeds(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);


-- Index for joins on post_id
CREATE INDEX idx_favorite_posts_post_id ON favorite_posts (post_id);

-- Index for joins on feed_id
CREATE INDEX idx_favorite_posts_feed_id ON favorite_posts (feed_id);

-- Index for joins on user_id
CREATE INDEX idx_favorite_posts_user_id ON favorite_posts (user_id);

CREATE INDEX idx_favorite_posts_user_post ON favorite_posts (user_id, post_id);

-- +goose Down
DROP INDEX IF EXISTS idx_favorite_posts_post_id;
DROP INDEX IF EXISTS idx_favorite_posts_feed_id;
DROP INDEX IF EXISTS idx_favorite_posts_user_id;
DROP INDEX IF EXISTS idx_favorite_posts_user_post;
DROP TABLE favorite_posts;