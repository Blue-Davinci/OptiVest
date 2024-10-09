-- +goose Up
CREATE TYPE transaction_type_enum AS ENUM('buy', 'sell', 'other');

-- Create the ENUM type for investment types
CREATE TYPE investment_type_enum AS ENUM ('Stock', 'Bond', 'Alternative');

CREATE TABLE investment_transactions (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGSERIAL NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    investment_type investment_type_enum NOT NULL,  -- E.g., Stock, Bond, Real Estate
    investment_id BIGSERIAL NOT NULL,  -- Refers to stock_investments, bond_investments, or alternative_investments
    transaction_type transaction_type_enum NOT NULL, -- Buy or Sell
    transaction_date DATE NOT NULL,
    transaction_amount DECIMAL(15, 2) NOT NULL CHECK (transaction_amount > 0),  -- Amount involved in the transaction
    quantity DECIMAL(15, 2) NOT NULL CHECK (quantity > 0),  -- Number of units bought/sold
    created_at TIMESTAMP(0) WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP(0) WITH TIME ZONE DEFAULT NOW()
);

-- Indexes for optimized queries
CREATE INDEX idx_investment_transactions_user_id ON investment_transactions(user_id);
CREATE INDEX idx_investment_transactions_investment_id ON investment_transactions(investment_id);
CREATE INDEX idx_investment_transactions_investment_type_id ON investment_transactions(investment_type, investment_id);
CREATE INDEX idx_investment_transactions_transaction_type ON investment_transactions(transaction_type);
CREATE INDEX idx_investment_transactions_investment_type ON investment_transactions(investment_type);


-- +goose Down
DROP INDEX IF EXISTS idx_investment_transactions_user_id;
DROP INDEX IF EXISTS idx_investment_transactions_investment_id;
DROP INDEX IF EXISTS idx_investment_transactions_investment_type_id;
DROP INDEX IF EXISTS idx_investment_transactions_transaction_type;
DROP INDEX IF EXISTS idx_investment_transactions_investment_type;
DROP TABLE IF EXISTS investment_transactions;
DROP TYPE IF EXISTS transaction_type_enum;
DROP TYPE IF EXISTS investment_type_enum;
