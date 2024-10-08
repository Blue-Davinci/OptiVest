-- +goose Up
CREATE TABLE stock_investments (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGSERIAL NOT NULL REFERENCES Users(id) ON DELETE CASCADE,
    stock_symbol VARCHAR(10) NOT NULL,
    quantity DECIMAL(15, 2) NOT NULL CHECK (quantity > 0),           -- Ensure quantity is positive
    purchase_price DECIMAL(15, 2) NOT NULL CHECK (purchase_price >= 0), -- Purchase price cannot be negative
    current_value DECIMAL(15, 2) NOT NULL DEFAULT 0 CHECK (current_value >= 0),    -- Current value cannot be negative
    sector VARCHAR(100),  -- Optional, for sector allocation
    purchase_date DATE NOT NULL,
    dividend_yield DECIMAL(5, 2),  -- Stock-specific attribute
    dividend_yield_updated_at TIMESTAMP(0) WITH TIME ZONE DEFAULT NOW(), -- Tracks when the yield was updated
    created_at TIMESTAMP(0) WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP(0) WITH TIME ZONE DEFAULT NOW()
);

-- Indexes for common query patterns
CREATE INDEX idx_stock_investments_user_id ON stock_investments(user_id);          -- For fetching all stock investments by a user
CREATE INDEX idx_stock_investments_stock_symbol ON stock_investments(stock_symbol); -- For quick lookup by stock symbol
CREATE INDEX idx_stock_investments_user_symbol ON stock_investments(user_id, stock_symbol); -- Composite index to quickly find a specific stock investment for a user
CREATE INDEX idx_stock_investments_purchase_date ON stock_investments(purchase_date); -- Index for sorting or filtering by purchase date

-- +goose StatementBegin
CREATE TRIGGER trigger_update_stock_investment_tracking_timestamp
BEFORE UPDATE ON stock_investments
FOR EACH ROW
EXECUTE FUNCTION update_timestamp();
-- +goose StatementEnd

-- +goose Down
DROP INDEX IF EXISTS idx_stock_investments_user_id;
DROP INDEX IF EXISTS idx_stock_investments_stock_symbol;
DROP INDEX IF EXISTS idx_stock_investments_user_symbol;
DROP INDEX IF EXISTS idx_stock_investments_purchase_date;
DROP TABLE IF EXISTS stock_investments;
DROP TRIGGER IF EXISTS trigger_update_stock_investment_tracking_timestamp ON stock_investments;