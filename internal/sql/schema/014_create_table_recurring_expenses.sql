-- +goose Up
-- ENUM for the recurrence interval
CREATE TYPE recurrence_interval_enum AS ENUM ('daily', 'weekly', 'monthly', 'yearly');
CREATE TABLE recurring_expenses (
    id BIGSERIAL PRIMARY KEY,                                  -- Unique ID for the recurring expense
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,   -- Reference to the user
    budget_id BIGINT NOT NULL REFERENCES budgets(id) ON DELETE CASCADE, -- Link to the budget
    amount NUMERIC(12, 2) NOT NULL CHECK (amount > 0),       -- Amount of the recurring expense
    name VARCHAR(255) NOT NULL,
    description TEXT,                                         -- Description of the expense
    recurrence_interval  recurrence_interval_enum NOT NULL DEFAULT 'monthly',                -- Interval type (e.g., daily, weekly, monthly, etc.)
    projected_amount NUMERIC(12, 2) NOT NULL CHECK (projected_amount > 0), -- Projected amount for the recurring expense
    next_occurrence DATE NOT NULL,                           -- The next date the expense should be added
    created_at TIMESTAMP(0) WITH TIME ZONE DEFAULT NOW(),    -- Creation timestamp
    updated_at TIMESTAMP(0) WITH TIME ZONE DEFAULT NOW(),     -- Last updated timestamp
    CONSTRAINT unique_recurring_expense UNIQUE (user_id, budget_id,name, recurrence_interval) -- Unique constraint
);

-- +goose StatementBegin
CREATE TRIGGER trigger_update_goals_tracking_timestamp
BEFORE UPDATE ON recurring_expenses
FOR EACH ROW
EXECUTE FUNCTION update_timestamp();
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION insert_recurring_expense_to_expenses()
RETURNS TRIGGER AS $$
BEGIN
    -- Insert a new record into the expenses table using data from the recurring_expenses table
    INSERT INTO expenses (user_id, budget_id, category, amount,name,  description,is_recurring, date_occurred, created_at, updated_at)
    VALUES (NEW.user_id, NEW.budget_id, 'Recurring', NEW.amount, NEW.name, NEW.description, true, NEW.next_occurrence, NOW(), NOW());

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER trigger_insert_recurring_expense
AFTER INSERT ON recurring_expenses
FOR EACH ROW
EXECUTE FUNCTION insert_recurring_expense_to_expenses();
-- +goose StatementEnd



-- Indexes
CREATE INDEX idx_recurring_expenses_user_id ON recurring_expenses(user_id);
CREATE INDEX idx_recurring_expenses_budget_id ON recurring_expenses(budget_id);
CREATE INDEX idx_recurring_expenses_next_occurrence ON recurring_expenses(next_occurrence);
CREATE INDEX idx_recurring_expenses_created_at ON recurring_expenses(created_at);

-- +goose Down
DROP TRIGGER IF EXISTS trigger_insert_recurring_expense ON recurring_expenses;
DROP TRIGGER IF EXISTS trigger_update_goals_tracking_timestamp ON recurring_expenses;
DROP INDEX IF EXISTS idx_recurring_expenses_user_id;
DROP INDEX IF EXISTS idx_recurring_expenses_budget_id;
DROP INDEX IF EXISTS idx_recurring_expenses_next_occurrence;
DROP INDEX IF EXISTS idx_recurring_expenses_created_at;
DROP TABLE IF EXISTS recurring_expenses;
DROP TYPE IF EXISTS recurrence_interval_enum;