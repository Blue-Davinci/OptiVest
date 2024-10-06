-- +goose Up
CREATE TABLE debtpayments (
    id BIGSERIAL PRIMARY KEY,             
    debt_id BIGINT NOT NULL REFERENCES debts(id) ON DELETE CASCADE, 
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE, 
    payment_amount NUMERIC(12, 2) NOT NULL CHECK (payment_amount > 0), 
    payment_date TIMESTAMP WITH TIME ZONE NOT NULL, 
    interest_payment NUMERIC(12, 2) NOT NULL CHECK (interest_payment >= 0), 
    principal_payment NUMERIC(12, 2) NOT NULL CHECK (principal_payment >= 0), 
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() 
);

-- Indexes for optimized querying
CREATE INDEX idx_debt_payments_debt_id ON debtpayments(debt_id);
CREATE INDEX idx_debt_payments_user_id ON debtpayments(user_id);
CREATE INDEX idx_debt_payments_payment_date ON debtpayments(payment_date);

-- +goose Down
DROP INDEX IF EXISTS idx_debt_payments_debt_id;
DROP INDEX IF EXISTS idx_debt_payments_user_id;
DROP INDEX IF EXISTS idx_debt_payments_payment_date;
DROP TABLE IF EXISTS debtpayments;