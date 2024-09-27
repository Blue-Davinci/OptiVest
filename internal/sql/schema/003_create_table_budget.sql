-- +goose Up
CREATE TABLE budgets (
    id BIGSERIAL PRIMARY KEY,                                -- Unique ID for each budget
    user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,    -- Reference to the user creating the budget
    name VARCHAR(255) NOT NULL,                              -- Budget name (e.g., "My Tech budget")
    category VARCHAR(255) NOT NULL,                          -- Budget category (e.g., "Technology", "Groceries")
    total_amount NUMERIC(20, 2) NOT NULL,                    -- Total budget amount for the specified period
    currency_code VARCHAR(3) NOT NULL,                       -- Currency code linked to user's profile (e.g., "USD")
    description TEXT,                                        -- Description or notes about the budget
    created_at TIMESTAMP(0) WITH TIME ZONE DEFAULT NOW(),          -- When the budget was created
    updated_at TIMESTAMP(0) WITH TIME ZONE DEFAULT NOW(),          -- When the budget was last updated
    CONSTRAINT chk_total_amount CHECK (total_amount > 0)     -- Ensure positive values for budget
);

-- Index for quickly fetching budgets by user_id
CREATE INDEX idx_budgets_user_id ON budgets (user_id);

-- +goose Down
DROP INDEX IF EXISTS idx_budgets_user_id;
DROP TABLE IF EXISTS budgets;
