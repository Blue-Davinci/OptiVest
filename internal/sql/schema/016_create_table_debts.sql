-- +goose Up
CREATE TABLE debts (
    id BIGSERIAL PRIMARY KEY,             
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE, 
    name VARCHAR(255) NOT NULL,
    amount NUMERIC(12, 2) NOT NULL CHECK (amount > 0),   
    remaining_balance NUMERIC(12, 2) NOT NULL CHECK (remaining_balance >= 0), 
    interest_rate NUMERIC(5, 2) CHECK (interest_rate >= 0), 
    description TEXT,                     
    due_date DATE NOT NULL,               
    minimum_payment NUMERIC(12, 2) NOT NULL CHECK (minimum_payment > 0),  
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(), 
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(), 
    -- For tracking the debt lifecycle and interest calculations
    next_payment_date DATE NOT NULL, -- The next scheduled payment date
    estimated_payoff_date DATE,      -- Estimated date when the debt will be fully paid off
    accrued_interest NUMERIC(12, 2) DEFAULT 0, -- Interest that has accrued but not yet paid
    interest_last_calculated DATE,   -- Last date interest was calculated
    last_payment_date DATE,          -- The date of the last payment made on the debt
    total_interest_paid NUMERIC(12, 2) DEFAULT 0 CHECK (total_interest_paid >= 0), -- Cumulative interest paid
    
    -- Constraints to enforce data integrity
    CONSTRAINT unique_debt_description_per_user UNIQUE (user_id, description),
    CONSTRAINT valid_interest CHECK (interest_rate >= 0 AND interest_rate <= 100)
);

-- Indexes for optimized queries
CREATE INDEX idx_debts_user_id ON debts(user_id);
CREATE INDEX idx_debts_due_date ON debts(due_date);
CREATE INDEX idx_debts_remaining_balance ON debts(remaining_balance);
CREATE INDEX idx_debts_next_payment_date ON debts(next_payment_date);

-- +goose StatementBegin
CREATE TRIGGER trigger_update_goals_tracking_timestamp
BEFORE UPDATE ON debts
FOR EACH ROW
EXECUTE FUNCTION update_timestamp();
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER IF EXISTS after_debt_insert ON debts;
DROP INDEX IF EXISTS idx_debts_user_id;
DROP INDEX IF EXISTS idx_debts_due_date;
DROP INDEX IF EXISTS idx_debts_remaining_balance;
DROP INDEX IF EXISTS idx_debts_next_payment_date;
DROP TABLE IF EXISTS debts;