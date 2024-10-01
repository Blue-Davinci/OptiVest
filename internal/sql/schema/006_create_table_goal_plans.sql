-- +goose Up
CREATE TABLE goal_plans (
    id BIGSERIAL PRIMARY KEY,                                  -- Unique ID for each template
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,      -- User-specific saving plan
    name VARCHAR(255) NOT NULL,                                -- Name of the saving plan (e.g., "Default Plan")
    description TEXT,                                        -- Description of the saving plan
    target_amount NUMERIC(20, 2),                              -- Target amount for the plan (optional)
    monthly_contribution NUMERIC(20, 2),                       -- Monthly contribution towards savings (optional)
    duration_in_months INTEGER,                                -- Duration for saving (optional)
    is_strict BOOLEAN NOT NULL DEFAULT FALSE,                           -- Whether the plan is strict or not
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,            -- When the template was created
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP             -- When the template was last updated
);

-- Add a unique constraint to ensure that each user has unique goal plan names
CREATE UNIQUE INDEX idx_unique_user_goal_plan_name ON goal_plans (user_id, name);
-- Index for user_id to quickly fetch saving plans
CREATE INDEX idx_saving_plans_user_id ON goal_plans (user_id);
CREATE INDEX idx_goals_created_date ON goal_plans (created_at);

-- +goose StatementBegin
CREATE TRIGGER trigger_update_goals_tracking_timestamp
BEFORE UPDATE ON goal_plans
FOR EACH ROW
EXECUTE FUNCTION update_timestamp();
-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trigger_update_goals_tracking_timestamp ON goal_plans;
-- +goose StatementEnd
DROP INDEX IF EXISTS idx_unique_user_goal_plan_name;
DROP INDEX IF EXISTS idx_saving_plans_user_id;
DROP INDEX IF EXISTS idx_goals_created_date;
DROP TABLE IF EXISTS goal_plans;

