-- +goose Up
CREATE TABLE awards (
    id SERIAL PRIMARY KEY,         -- Unique identifier for each award
    code VARCHAR(50) NOT NULL UNIQUE, -- Unique code for the award
    description TEXT NOT NULL,      -- Description of the award
    point INTEGER NOT NULL DEFAULT 1,             -- Point value of the award
    created_at TIMESTAMP(0) WITH TIME ZONE NOT NULL DEFAULT NOW(),  -- Timestamp for when the award was created
    updated_at TIMESTAMP(0) WITH TIME ZONE NOT NULL DEFAULT NOW()   -- Timestamp for when the award was last updated
);

-- Index for fast lookup by award code
CREATE INDEX idx_award_code ON awards (code);

-- +goose StatementBegin
CREATE TRIGGER trigger_update_awards_tracking_timestamp
BEFORE UPDATE ON awards
FOR EACH ROW
EXECUTE FUNCTION update_timestamp();
-- +goose StatementEnd

-- Insert initial award data
INSERT INTO awards (code, description, point, created_at, updated_at)
VALUES
    ('first_transaction', 'Awarded for adding the first transaction.', 5, NOW(), NOW()), 
    ('first_income', 'Awarded for adding the first income.', 10, NOW(), NOW()),
    ('first_expense', 'Awarded for adding the first expense.', 10, NOW(), NOW()),
    ('first_goal', 'Awarded for creating the first goal.', 15, NOW(), NOW()),
    ('first_budget', 'Awarded for creating the first budget.', 20, NOW(), NOW()),
    ('first_analysis', 'Awarded for completing the first financial analysis.', 25, NOW(), NOW()),
    ('first_goal_completed', 'Awarded for completing the first goal.', 40, NOW(), NOW()),
    ('first_debt_paid', 'Awarded for completing the first goal.', 40, NOW(), NOW());

-- +goose Down
DROP INDEX idx_award_code;
DROP TRIGGER trigger_update_awards_tracking_timestamp ON awards;
DROP TABLE awards;