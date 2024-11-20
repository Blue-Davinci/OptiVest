
-- +goose Up
-- +goose StatementBegin

-- 1. Create the `comment_reactions` table
CREATE TABLE comment_reactions (
    id BIGSERIAL PRIMARY KEY,
    comment_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    created_at TIMESTAMP(0) WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP(0) WITH TIME ZONE DEFAULT NOW(),
    FOREIGN KEY (comment_id) REFERENCES comments(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    UNIQUE (comment_id, user_id) -- Each user can react to a comment only once
);

-- +goose StatementEnd

-- +goose StatementBegin

-- 2. Create indexes for `comment_reactions` table
CREATE INDEX idx_comment_user ON comment_reactions (comment_id, user_id);

-- +goose StatementEnd

-- +goose StatementBegin

-- 3. Create the trigger function for awarding the "first_comment_reaction"
CREATE OR REPLACE FUNCTION award_first_comment_reaction()
RETURNS TRIGGER AS $$
BEGIN
    -- Check if the user has not received the "first_comment_reaction" award yet
    IF (SELECT COUNT(*) FROM user_awards ua
        JOIN awards a ON a.id = ua.award_id
        WHERE ua.user_id = NEW.user_id 
        AND a.code = 'first_comment_reaction') = 0 THEN

        -- Insert the "first_comment_reaction" award
        INSERT INTO user_awards (user_id, award_id, created_at)
        SELECT NEW.user_id, a.id, NOW()
        FROM awards a
        WHERE a.code = 'first_comment_reaction';
    END IF;
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- 4. Create the trigger
CREATE TRIGGER trigger_award_first_comment_reaction
AFTER INSERT ON comment_reactions
FOR EACH ROW
EXECUTE FUNCTION award_first_comment_reaction();

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- 1. Drop the trigger for "first_comment_reaction" award
DROP TRIGGER IF EXISTS trigger_award_first_comment_reaction ON comment_reactions;
DROP FUNCTION IF EXISTS award_first_comment_reaction();

-- +goose StatementEnd

-- +goose StatementBegin

-- 2. Drop indexes for `comment_reactions`
DROP INDEX IF EXISTS idx_comment_user;
DROP INDEX IF EXISTS idx_user_liked;

-- +goose StatementEnd

-- +goose StatementBegin

-- 3. Drop the `comment_reactions` table
DROP TABLE IF EXISTS comment_reactions;

-- +goose StatementEnd
