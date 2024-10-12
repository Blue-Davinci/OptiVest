-- +goose Up
CREATE TYPE feed_type AS ENUM ('rss', 'json');
CREATE TYPE feed_approval_status AS ENUM ('pending', 'approved', 'rejected');
CREATE TABLE feeds(
        id BIGSERIAL PRIMARY KEY,
        user_id bigserial NOT NULL REFERENCES users(id) ON DELETE CASCADE,
        name TEXT NOT NULL,
        url TEXT UNIQUE NOT NULL,
        img_url TEXT,
        feed_type feed_type NOT NULL DEFAULT 'rss',
        feed_category TEXT NOT NULL,
        feed_description TEXT,
        is_hidden BOOLEAN NOT NULL DEFAULT FALSE,
        approval_status feed_approval_status NOT NULL DEFAULT 'pending',
        version INT NOT NULL DEFAULT 1,
        created_at timestamp(0) with time zone NOT NULL DEFAULT NOW(),
        updated_at timestamp(0) with time zone NOT NULL DEFAULT NOW(),
        last_fetched_at timestamp(0) with time zone 
);

CREATE INDEX idx_feeds_user_id ON feeds(user_id);
CREATE INDEX idx_feeds_name ON feeds(name);
CREATE INDEX idx_feeds_approval_status ON feeds(approval_status);
CREATE INDEX idx_feeds_optimized ON feeds (
    created_at DESC,
    feed_type,
    is_hidden,
    approval_status,
    to_tsvector('simple', name)
);

-- +goose StatementBegin
CREATE TRIGGER trigger_update_alternative_investment_tracking_timestamp
BEFORE UPDATE ON feeds
FOR EACH ROW
EXECUTE FUNCTION update_timestamp();
-- +goose StatementEnd

-- +goose Down
DROP INDEX idx_feeds_user_id;
DROP INDEX idx_feeds_name;
DROP INDEX idx_feeds_approval_status;
DROP INDEX idx_feeds_optimized;

DROP TABLE feeds;
DROP TYPE feed_type;
DROP TYPE feed_approval_status;