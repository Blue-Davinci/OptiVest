-- +goose Up
-- Create enumeration type for tracking types
CREATE TYPE tracking_type_enum AS ENUM ('monthly', 'bonus', 'other');

CREATE TABLE goal_tracking (
    id BIGSERIAL PRIMARY KEY,                                        -- Unique tracking entry ID
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,            -- User who owns the goal
    goal_id BIGINT NOT NULL REFERENCES goals(id) ON DELETE SET NULL,            -- Reference to the goal
    tracking_date DATE NOT NULL DEFAULT CURRENT_DATE,                 -- Date of tracking (when progress was recorded)
    contributed_amount NUMERIC(20, 2) NOT NULL,                       -- Amount contributed towards the goal
    tracking_type tracking_type_enum NOT NULL DEFAULT 'monthly',      -- Type of tracking
    created_at TIMESTAMP(0) WITH TIME ZONE DEFAULT NOW(),             -- When the contribution was made
    updated_at TIMESTAMP(0) WITH TIME ZONE DEFAULT NOW(),             -- When the contribution was last updated
    truncated_tracking_date DATE,                                     -- Truncated tracking date for indexing

    CONSTRAINT check_positive_contribution CHECK (contributed_amount > 0)
);

-- Trigger function to populate truncated_tracking_date
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION set_truncated_tracking_date()
RETURNS TRIGGER AS $$
BEGIN
    NEW.truncated_tracking_date := date_trunc('month', NEW.tracking_date)::DATE;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- Attach the reusable trigger to the `goal_tracking` table
-- +goose StatementBegin
CREATE TRIGGER trigger_update_goals_tracking_timestamp
BEFORE UPDATE ON goal_tracking
FOR EACH ROW
EXECUTE FUNCTION update_timestamp();
-- +goose StatementEnd

-- Trigger to set truncated_tracking_date on insert and update
CREATE TRIGGER set_truncated_tracking_date_trigger
BEFORE INSERT OR UPDATE ON goal_tracking
FOR EACH ROW
EXECUTE FUNCTION set_truncated_tracking_date();

-- Index for fast retrieval by goal and user
CREATE INDEX idx_goal_tracking_goal_user ON goal_tracking (goal_id, user_id);
-- Index for fast retrieval by user
CREATE INDEX idx_goal_tracking_user_id ON goal_tracking (user_id);
-- Composite index for fast retrieval by goal and tracking date
CREATE INDEX idx_goal_tracking_goal_date ON goal_tracking (goal_id, tracking_date);
-- Index for fast retrieval by tracking date
CREATE INDEX idx_goal_tracking_date ON goal_tracking (tracking_date);
-- Composite index for fast retrieval by tracking date, goal and user
CREATE INDEX idx_goal_tracking_date_goal_user ON goal_tracking (tracking_date, goal_id, user_id);
-- Unique constraint for monthly tracking entries
CREATE UNIQUE INDEX unique_goal_monthly_tracking
ON goal_tracking (goal_id, user_id, truncated_tracking_date)
WHERE tracking_type = 'monthly';

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trigger_update_goals_tracking_timestamp ON goal_tracking;
DROP TRIGGER IF EXISTS set_truncated_tracking_date_trigger ON goal_tracking;
DROP FUNCTION IF EXISTS set_truncated_tracking_date;
-- +goose StatementEnd
DROP INDEX IF EXISTS unique_goal_monthly_tracking;
DROP INDEX IF EXISTS idx_goal_tracking_date;
DROP INDEX IF EXISTS idx_goal_tracking_goal_date;
DROP INDEX IF EXISTS idx_goal_tracking_user_id;
DROP INDEX IF EXISTS idx_goal_tracking_goal_user;
DROP INDEX IF EXISTS idx_goal_tracking_date_goal_user;
DROP TABLE IF EXISTS goal_tracking;
DROP TYPE IF EXISTS tracking_type_enum;