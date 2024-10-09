-- +goose Up
CREATE TABLE bond_investments (
    id BIGSERIAL PRIMARY KEY,                          -- Unique bond investment ID
    user_id BIGSERIAL NOT NULL REFERENCES Users(id) ON DELETE CASCADE,  -- Reference to the user, cascade on delete
    bond_symbol VARCHAR(10) NOT NULL,                  -- Bond symbol, cannot be null
    quantity DECIMAL(15, 2) NOT NULL CHECK (quantity > 0),                 -- Quantity must be a positive integer
    purchase_price DECIMAL(15, 2) NOT NULL CHECK (purchase_price >= 0), -- Purchase price cannot be negative
    current_value DECIMAL(15, 2) NOT NULL CHECK (current_value >= 0),   -- Current value cannot be negative
    coupon_rate DECIMAL(5, 2) CHECK (current_value >= 0),        -- Coupon rate should be a non-negative percentage
    maturity_date DATE NOT NULL,                       -- Maturity date for the bond, must be specified
    purchase_date DATE NOT NULL,                       -- Purchase date is mandatory
    created_at TIMESTAMP(0) WITH TIME ZONE DEFAULT NOW(),  -- Record creation timestamp
    updated_at TIMESTAMP(0) WITH TIME ZONE DEFAULT NOW()  -- Record update timestamp
);

-- Indexes for efficient querying
CREATE INDEX idx_bond_investments_user_id ON bond_investments(user_id);               -- For fetching bond investments by user
CREATE INDEX idx_bond_investments_bond_symbol ON bond_investments(bond_symbol);       -- For searching by bond symbol
CREATE INDEX idx_bond_investments_user_symbol ON bond_investments(user_id, bond_symbol); -- Composite index to fetch specific bonds for a user
CREATE INDEX idx_bond_investments_maturity_date ON bond_investments(maturity_date);    -- Index to quickly filter or sort by maturity date

-- +goose StatementBegin
CREATE TRIGGER trigger_update_bond_investment_tracking_timestamp
BEFORE UPDATE ON bond_investments
FOR EACH ROW
EXECUTE FUNCTION update_timestamp();
-- +goose StatementEnd

-- +goose Down
DROP INDEX IF EXISTS idx_bond_investments_user_id;
DROP INDEX IF EXISTS idx_bond_investments_bond_symbol;
DROP INDEX IF EXISTS idx_bond_investments_user_symbol;
DROP INDEX IF EXISTS idx_bond_investments_maturity_date;
DROP TABLE IF EXISTS bond_investments;
DROP TRIGGER IF EXISTS trigger_update_bond_investment_tracking_timestamp ON stock_investments;


