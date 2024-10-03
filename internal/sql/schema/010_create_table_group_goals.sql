
-- +goose Up
CREATE TABLE group_goals (
    id BIGSERIAL PRIMARY KEY,                                        -- Unique ID for each group goal
    group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE, -- Group reference, deletes goal if group is deleted
    creator_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE, -- User who created the goal
    goal_name VARCHAR(255) NOT NULL,                                 -- Name of the goal
    target_amount NUMERIC(12, 2) NOT NULL,                           -- The target amount to achieve the goal
    current_amount NUMERIC(12, 2) DEFAULT 0 CHECK (current_amount >= 0), -- Current amount contributed, cannot be negative
    start_date DATE NOT NULL DEFAULT NOW(),                          -- Start date of the goal
    deadline DATE NOT NULL,                                          -- Deadline for achieving the goal
    description TEXT NOT NULL,                                       -- Description of the goal
    status goal_status NOT NULL DEFAULT 'ongoing',                    -- Status of the goal (active, completed, failed)
    created_at TIMESTAMP(0) WITH TIME ZONE DEFAULT NOW() NOT NULL,                   -- When the goal was created
    updated_at TIMESTAMP(0) WITH TIME ZONE DEFAULT NOW() NOT NULL,                   -- Last time the goal was updated
    CONSTRAINT unique_goal_name_per_user_group UNIQUE (goal_name, group_id, creator_user_id),
    CONSTRAINT no_overfunding CHECK (current_amount <= target_amount), -- Prevent overfunding, ensures current amount does not exceed the target
    CONSTRAINT goal_deadline_future CHECK (deadline > NOW())        -- Ensure the deadline is in the future when creating the goal
);

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION update_goal_amount()
RETURNS TRIGGER AS $$
BEGIN
    -- If a contribution is made, update the associated goal's current amount
    IF NEW.transaction_type = 'contribution' AND NEW.goal_id IS NOT NULL THEN
        UPDATE group_goals
        SET current_amount = current_amount + NEW.amount
        WHERE id = NEW.goal_id;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER trigger_update_goals_goals_timestamp
BEFORE UPDATE ON group_goals
FOR EACH ROW
EXECUTE FUNCTION update_timestamp();
-- +goose StatementEnd

-- Indexes for faster querying
CREATE INDEX idx_group_goals_group_id ON group_goals(group_id);        -- Index to quickly find goals by group
CREATE INDEX idx_group_goals_deadline ON group_goals(deadline);        -- Index for querying goals based on deadline
CREATE INDEX idx_group_goals_target_current ON group_goals(target_amount, current_amount); -- Index for goal contribution analysis

-- +goose Down
DROP TRIGGER IF EXISTS trigger_update_goals_goals_timestamp ON group_goals;
DROP TRIGGER IF EXISTS trigger_update_goal_amount ON group_transactions;
DROP INDEX IF EXISTS idx_group_goals_target_current;
DROP INDEX IF EXISTS idx_group_goals_deadline;
DROP INDEX IF EXISTS idx_group_goals_group_id;
DROP TABLE IF EXISTS group_goals;
