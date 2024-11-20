-- +goose Up
-- +goose StatementBegin

-- 1. Create the comments table
CREATE TYPE comment_associated_type AS ENUM ('group', 'feed', 'other');
CREATE TABLE comments (
    id BIGSERIAL PRIMARY KEY,
    content TEXT NOT NULL,
    user_id BIGINT NOT NULL,
    parent_id BIGINT NULL,
    associated_type comment_associated_type NOT NULL,
    associated_id BIGINT NOT NULL,
    created_at TIMESTAMP(0) WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP(0) WITH TIME ZONE DEFAULT NOW(),
    version INT DEFAULT 1,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (parent_id) REFERENCES comments(id) ON DELETE CASCADE,
    CHECK (LENGTH(content) > 0)
);

-- 2. Create indexes separately
CREATE INDEX idx_associated ON comments (associated_type, associated_id);
CREATE INDEX idx_parent ON comments (parent_id);
CREATE INDEX idx_user ON comments (user_id);
CREATE INDEX idx_created_at ON comments (created_at);

-- 3. Create the award trigger function for the "first comment"
CREATE OR REPLACE FUNCTION award_first_comment()
RETURNS TRIGGER AS $$
BEGIN
    -- Check if this is the first comment by the user
    IF (SELECT COUNT(*) FROM comments WHERE user_id = NEW.user_id) = 1 THEN
        -- Insert the award for the first comment
        INSERT INTO user_awards (user_id, award_id, created_at)
        SELECT NEW.user_id, a.id, NOW()
        FROM awards a
        WHERE a.code = 'first_comment';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- 4. Create the trigger for awarding the "first comment"
CREATE TRIGGER trigger_award_first_comment
AFTER INSERT ON comments
FOR EACH ROW
EXECUTE FUNCTION award_first_comment();

-- 5. Create the trigger function to update the updated_at column
CREATE TRIGGER update_updated_at_column
BEFORE UPDATE ON comments
FOR EACH ROW
EXECUTE FUNCTION update_timestamp();

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- 1. Drop the trigger and its function
DROP TRIGGER IF EXISTS trigger_award_first_comment ON comments;
DROP FUNCTION IF EXISTS award_first_comment;

-- 2. Drop the trigger for updating the updated_at column
DROP TRIGGER IF EXISTS update_updated_at_column ON comments;

-- 2. Drop indexes
DROP INDEX IF EXISTS idx_associated;
DROP INDEX IF EXISTS idx_parent;
DROP INDEX IF EXISTS idx_user;
DROP INDEX IF EXISTS idx_created_at;

-- 3. Drop the comments table
DROP TABLE IF EXISTS comments;
DROP TYPE IF EXISTS comment_associated_type;

-- +goose StatementEnd
