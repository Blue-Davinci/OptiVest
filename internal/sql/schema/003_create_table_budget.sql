-- +goose Up
CREATE TABLE budgets (
    id BIGSERIAL PRIMARY KEY,                                -- Unique ID for each budget
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,    -- Reference to the user creating the budget
    name VARCHAR(255) NOT NULL,                              -- Budget name (e.g., "My Tech budget")
    is_Strict boolean NOT NULL DEFAULT TRUE,                              -- Strict budget or not i.e. if the user can spend more than the budget
    category VARCHAR(255) NOT NULL,                          -- Budget category (e.g., "Technology", "Groceries")
    total_amount NUMERIC(20, 2) NOT NULL,                    -- Total budget amount for the specified period
    currency_code VARCHAR(3) NOT NULL,                       -- Currency code linked to user's profile (e.g., "USD")
    conversion_rate NUMERIC(20, 2) NOT NULL,                 -- Conversion rate to the user's default currency
    description TEXT,                                        -- Description or notes about the budget
    created_at TIMESTAMP(0) WITH TIME ZONE NOT NULL DEFAULT NOW(),    -- When the budget was created
    updated_at TIMESTAMP(0) WITH TIME ZONE NOT NULL DEFAULT NOW(),    -- When the budget was last updated
    CONSTRAINT chk_total_amount CHECK (total_amount > 0)     -- Ensure positive values for budget
);

-- Attach the reusable trigger to the `budgets` table
-- +goose StatementBegin
CREATE TRIGGER trigger_update_budgets_timestamp
BEFORE UPDATE ON budgets
FOR EACH ROW
EXECUTE FUNCTION update_timestamp();
-- +goose StatementEnd

-- Create indexes for the `budgets` table
CREATE INDEX idx_budgets_user_id ON budgets(user_id);
CREATE INDEX idx_budgets_name ON budgets USING gin(to_tsvector('simple', name));
CREATE INDEX idx_budgets_user_id_name_created_at ON budgets(user_id, to_tsvector('simple', name), created_at DESC);

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trigger_update_budgets_timestamp ON budgets;
-- +goose StatementEnd

DROP INDEX IF EXISTS idx_budgets_user_id;
DROP INDEX IF EXISTS idx_budgets_name;
DROP INDEX IF EXISTS idx_budgets_user_id_name_created_at;

DROP TABLE IF EXISTS budgets;
