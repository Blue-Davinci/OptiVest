-- +goose Up
CREATE TABLE income (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    source VARCHAR(255) NOT NULL,
    original_currency_code CHAR(3) NOT NULL,                  -- Original currency before conversion
    amount_original DECIMAL(15, 2) NOT NULL,                  -- Original amount in foreign currency
    amount DECIMAL(15, 2) NOT NULL,                           -- Amount converted to user's default currency
    exchange_rate DECIMAL(15, 6) NOT NULL DEFAULT 1,                    -- Exchange rate used during conversion
    description TEXT,
    date_received DATE NOT NULL,
    created_at TIMESTAMP(0) WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP(0) WITH TIME ZONE DEFAULT NOW(),

    CONSTRAINT chk_amount_positive CHECK (amount_original > 0)
);

-- +goose StatementBegin
CREATE TRIGGER trigger_update_goals_tracking_timestamp
BEFORE UPDATE ON income
FOR EACH ROW
EXECUTE FUNCTION update_timestamp();
-- +goose StatementEnd

-- Indexes
CREATE INDEX idx_incomes_user_id ON income(user_id);
CREATE INDEX idx_incomes_date_received ON income(date_received);
CREATE INDEX idx_incomes_created_at ON income(created_at);

-- +goose Down
DROP INDEX IF EXISTS idx_incomes_user_id;
DROP INDEX IF EXISTS idx_incomes_date_received;
DROP INDEX IF EXISTS idx_incomes_created_at;
DROP TABLE IF EXISTS income;
DROP TRIGGER IF EXISTS trigger_update_goals_tracking_timestamp ON incomes;