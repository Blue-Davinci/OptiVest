-- +goose Up
CREATE TABLE goal_tracking (
    id BIGSERIAL PRIMARY KEY,                                        -- Unique tracking entry ID
    user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,            -- User who owns the goal
    goal_id BIGINT REFERENCES goals(id) ON DELETE CASCADE,            -- Reference to the goal
    tracking_date DATE NOT NULL DEFAULT CURRENT_DATE,                 -- Date of tracking (when progress was recorded)
    contributed_amount NUMERIC(20, 2) NOT NULL,                       -- Amount contributed towards the goal
    created_at TIMESTAMP(0) WITH TIME ZONE DEFAULT NOW(),                  -- When the contribution was made
    updated_at TIMESTAMP(0) WITH TIME ZONE DEFAULT NOW()                  -- When the contribution was last updated
);

-- Index for fast retrieval by goal and user
CREATE INDEX idx_goal_tracking_goal_user ON goal_tracking (goal_id, user_id);

-- Index for fast retrieval by user
CREATE INDEX idx_goal_tracking_user_id ON goal_tracking (user_id);

-- Composite index for fast retrieval by goal and tracking date
CREATE INDEX idx_goal_tracking_goal_date ON goal_tracking (goal_id, tracking_date);

-- Index for fast retrieval by tracking date
CREATE INDEX idx_goal_tracking_date ON goal_tracking (tracking_date);

-- +goose Down
DROP INDEX IF EXISTS idx_goal_tracking_date;
DROP INDEX IF EXISTS idx_goal_tracking_goal_date;
DROP INDEX IF EXISTS idx_goal_tracking_user_id;
DROP INDEX IF EXISTS idx_goal_tracking_goal_user;
DROP TABLE IF EXISTS goal_tracking;