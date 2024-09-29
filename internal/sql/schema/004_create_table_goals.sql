-- +goose Up
-- Create ENUM type for MFA status
CREATE TYPE goal_status AS ENUM ('ongoing', 'completed', 'cancelled');
CREATE TABLE goals (
    id BIGSERIAL PRIMARY KEY,                                    -- Unique ID for each goal
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,        -- Reference to the user
    budget_id BIGINT REFERENCES budgets(id) ON DELETE SET NULL,   -- Optional reference to a budget, allows null
    name VARCHAR(255) NOT NULL,                                  -- Goal name (e.g., "Save for a car")
    current_amount NUMERIC(20, 2) DEFAULT 0,                     -- Current amount saved towards the goal
    target_amount NUMERIC(20, 2) NOT NULL,                       -- Total amount the user wants to achieve
    monthly_contribution NUMERIC(20, 2) NOT NULL,                -- Monthly savings towards the goal
    start_date DATE NOT NULL,                                    -- Start date of the goal
    end_date DATE NOT NULL,                                      -- End date or target date for completion
    status goal_status NOT NULL DEFAULT 'ongoing',                        -- Status of the goal (e.g., "ongoing", "completed")
    created_at TIMESTAMP(0) WITH TIME ZONE NOT NULL DEFAULT NOW(),              -- Timestamp for creation
    updated_at TIMESTAMP(0) WITH TIME ZONE NOT NULL DEFAULT NOW()               -- Timestamp for last update
    -- Constraints
    CONSTRAINT chk_positive_target CHECK (target_amount > 0),
    CONSTRAINT chk_positive_contribution CHECK (monthly_contribution > 0),
    CONSTRAINT chk_date_order CHECK (end_date > start_date),
    CONSTRAINT unique_user_goal_name UNIQUE (user_id, name)
);

-- Attach the reusable trigger to the `budgets` table
-- +goose StatementBegin
CREATE TRIGGER trigger_update_goals_timestamp
BEFORE UPDATE ON goals
FOR EACH ROW
EXECUTE FUNCTION update_timestamp();
-- +goose StatementEnd

-- Index to retrieve goals for users quickly
CREATE INDEX idx_goals_user_id ON goals (user_id);
-- Index to retrieve goals by status quickly
CREATE INDEX idx_goals_status ON goals (status);
-- Index to retrieve goals by budget quickly
CREATE INDEX idx_goals_budget_id ON goals(budget_id);


-- Composite index to retrieve goals by user and status quickly
CREATE INDEX idx_goals_user_id_status ON goals (user_id, status);

-- Index to retrieve goals by end date quickly
CREATE INDEX idx_goals_end_date ON goals (end_date);

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trigger_update_goals_timestamp ON goals;
-- +goose StatementEnd
DROP INDEX IF EXISTS idx_goals_end_date;
DROP INDEX IF EXISTS idx_goals_user_id_status;
DROP INDEX IF EXISTS idx_goals_status;
DROP INDEX IF EXISTS idx_goals_user_id;
DROP TABLE IF EXISTS goals;
DROP TYPE IF EXISTS goal_status;
