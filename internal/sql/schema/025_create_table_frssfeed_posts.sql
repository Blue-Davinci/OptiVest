-- +goose Up
CREATE TABLE rssfeed_posts (
    id BIGSERIAL PRIMARY KEY,
    created_at timestamp(0) NOT NULL DEFAULT now(),
    updated_at timestamp(0) NOT NULL DEFAULT now(),
    channeltitle TEXT NOT NULL,
    channelurl TEXT,
    channeldescription TEXT,
    channellanguage TEXT DEFAULT 'en',
    itemtitle TEXT NOT NULL,
    itemdescription TEXT,
    itemcontent TEXT,
    itempublished_at timestamp(0) with time zone NOT NULL,
    itemurl TEXT NOT NULL UNIQUE,
    img_url TEXT NOT NULL,
    feed_id BIGSERIAL NOT NULL REFERENCES feeds(id) ON DELETE CASCADE
);

CREATE INDEX ON rssfeed_posts (feed_id);
CREATE INDEX idx_rssfeed_posts_itemtitle_tsvector ON rssfeed_posts USING gin (to_tsvector('simple', itemtitle));
CREATE INDEX idx_rssfeed_posts_created_at ON rssfeed_posts (created_at DESC);
-- +goose Down
DROP INDEX IF EXISTS idx_rssfeed_posts_feed_id;
DROP INDEX IF EXISTS idx_rssfeed_posts_itemtitle_tsvector;
DROP INDEX IF EXISTS idx_rssfeed_posts_created_at;
DROP TABLE IF EXISTS rssfeed_posts;