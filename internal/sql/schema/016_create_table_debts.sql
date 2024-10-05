CREATE TABLE debts (
    id BIGSERIAL PRIMARY KEY,             
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE, 
    amount NUMERIC(12, 2) NOT NULL CHECK (amount > 0),   
    remaining_balance NUMERIC(12, 2) NOT NULL CHECK (remaining_balance >= 0), 
    interest_rate NUMERIC(5, 2) CHECK (interest_rate >= 0), 
    description TEXT,                     
    due_date DATE NOT NULL,               
    minimum_payment NUMERIC(12, 2) NOT NULL CHECK (minimum_payment > 0),  
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(), 
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(), 

    -- New columns for tracking the debt lifecycle and interest calculations
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
CREATE OR REPLACE FUNCTION add_initial_tracking_entry()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO debt_tracking_entries (debt_id, tracked_date, amount, balance, interest, total_paid, next_due_date, created_at)
    VALUES (NEW.id, NOW(), NEW.amount, NEW.amount, 0, 0, NEW.due_date, NOW());
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER after_debt_insert
AFTER INSERT ON debts
FOR EACH ROW
EXECUTE FUNCTION add_initial_tracking_entry();
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER IF EXISTS after_debt_insert ON debts;
DROP FUNCTION IF EXISTS add_initial_tracking_entry();
DROP INDEX IF EXISTS idx_debts_user_id;
DROP INDEX IF EXISTS idx_debts_due_date;
DROP INDEX IF EXISTS idx_debts_remaining_balance;
DROP INDEX IF EXISTS idx_debts_next_payment_date;
DROP TABLE IF EXISTS debts;