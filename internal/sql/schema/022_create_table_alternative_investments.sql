-- +goose Up
CREATE TABLE alternative_investments (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    investment_type VARCHAR(100) NOT NULL,  -- E.g., 'Real Estate', 'Business', 'Art', etc.
    investment_name VARCHAR(255),  -- Optional, could be NULL
    is_business BOOLEAN NOT NULL DEFAULT FALSE,  -- Flag to indicate if the investment is business-related
    quantity DECIMAL(10,2),  -- E.g., number of properties or units (Optional, could be NULL)
    annual_revenue DECIMAL(15, 2),  -- Optional, for business-related investments
    acquired_at DATE NOT NULL,  -- Date when the investment was acquired
    profit_margin DECIMAL(5, 2),  -- Optional, profit margin as a percentage for business-related investments
    valuation DECIMAL(15, 2) NOT NULL,  -- Current valuation of the investment
    valuation_updated_at TIMESTAMP(0) WITH TIME ZONE DEFAULT NOW(),  -- To track when the valuation was updated
    location VARCHAR(255),  -- Location of the investment, e.g., city, state, country
    created_at TIMESTAMP(0) WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP(0) WITH TIME ZONE DEFAULT NOW()
);

-- Add indexes for optimized queries
CREATE INDEX idx_alternative_investments_user_id ON alternative_investments (user_id);
CREATE INDEX idx_alternative_investments_type ON alternative_investments (investment_type);
CREATE INDEX idx_alternative_investments_valuation ON alternative_investments (valuation DESC);
CREATE INDEX idx_alternative_investments_created_at ON alternative_investments (created_at);

-- +goose StatementBegin
CREATE TRIGGER trigger_update_alternative_investment_tracking_timestamp
BEFORE UPDATE ON alternative_investments
FOR EACH ROW
EXECUTE FUNCTION update_timestamp();
-- +goose StatementEnd

-- +goose Down
-- Drop indexes first
DROP INDEX IF EXISTS idx_alternative_investments_user_id;
DROP INDEX IF EXISTS idx_alternative_investments_type;
DROP INDEX IF EXISTS idx_alternative_investments_valuation;
DROP INDEX IF EXISTS idx_alternative_investments_created_at;

-- Drop the table
DROP TABLE IF EXISTS alternative_investments;
DROP TRIGGER IF EXISTS trigger_update_alternative_investment_tracking_timestamp ON stock_investments;
