-- +goose Up
CREATE TABLE expenses (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    budget_id BIGINT NOT NULL REFERENCES budgets(id),  -- Link to the relevant budget
    name VARCHAR(255) NOT NULL,
    category VARCHAR(255) NOT NULL,
    amount DECIMAL(15, 2) NOT NULL,                   -- Amount in the budget's default currency
    is_recurring BOOLEAN NOT NULL DEFAULT FALSE,
    description TEXT,
    date_occurred DATE NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),

    CONSTRAINT chk_amount_positive CHECK (amount > 0)
);

-- +goose StatementBegin
CREATE TRIGGER trigger_update_goals_tracking_timestamp
BEFORE UPDATE ON expenses
FOR EACH ROW
EXECUTE FUNCTION update_timestamp();
-- +goose StatementEnd

-- Indexes
CREATE INDEX idx_expenses_user_id ON expenses(user_id);
CREATE INDEX idx_expenses_date ON expenses(date_occurred);
CREATE INDEX idx_expenses_category ON expenses(category);
CREATE INDEX idx_expenses_amount ON expenses(amount);

-- +goose Down
DROP TRIGGER IF EXISTS trigger_update_goals_tracking_timestamp ON group_transactions;
DROP INDEX idx_expenses_user_id;
DROP INDEX idx_expenses_date;
DROP INDEX idx_expenses_category;
DROP INDEX idx_expenses_amount;
DROP TABLE expenses;