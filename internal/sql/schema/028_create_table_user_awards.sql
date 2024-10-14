-- +goose Up
CREATE TABLE user_awards (
    user_id BIGSERIAL NOT NULL,                -- Foreign key to users table
    award_id SERIAL NOT NULL,               -- Foreign key to awards table
    created_at TIMESTAMP(0) WITH TIME ZONE NOT NULL DEFAULT NOW(),  -- Timestamp for when the award was earned
    PRIMARY KEY (user_id, award_id),     -- Composite primary key to prevent duplicate entries
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE, -- Foreign key constraint
    FOREIGN KEY (award_id) REFERENCES awards(id) ON DELETE CASCADE  -- Foreign key constraint
);

-- Index for fast lookup by user_id
CREATE INDEX idx_user_awards_user ON user_awards (user_id);

-- Index for fast lookup by award_id
CREATE INDEX idx_user_awards_award ON user_awards (award_id);

-- +goose Down
DROP INDEX idx_user_awards_user;
DROP INDEX idx_user_awards_award;
DROP TABLE user_awards;
