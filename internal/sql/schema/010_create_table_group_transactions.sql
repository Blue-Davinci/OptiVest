-- +goose Up
CREATE TABLE group_transactions (
    id BIGSERIAL PRIMARY KEY,                                  -- Unique transaction ID
    group_id BIGINT REFERENCES groups(id) ON DELETE CASCADE,    -- Reference to the group
    member_id BIGINT REFERENCES users(id) ON DELETE CASCADE,    -- User/member who made the transaction
    transaction_type VARCHAR(50) NOT NULL,                     -- Type of transaction ('contribution', 'expense', etc.)
    amount NUMERIC(12, 2) NOT NULL CHECK (amount > 0),          -- Amount involved in the transaction
    description TEXT,                                           -- Optional description of the transaction
    created_at TIMESTAMPTZ DEFAULT NOW(),                       -- Time the transaction was created
    updated_at TIMESTAMPTZ DEFAULT NOW(),                       -- Time the transaction was last updated
    CONSTRAINT check_valid_transaction_type CHECK (transaction_type IN ('contribution', 'expense'))
);

-- +goose StatementBegin
CREATE TRIGGER trigger_update_goals_tracking_timestamp
BEFORE UPDATE ON group_transactions
FOR EACH ROW
EXECUTE FUNCTION update_timestamp();
-- +goose StatementEnd

-- Indexes for optimization
CREATE INDEX idx_group_transactions_group_id ON group_transactions (group_id);
CREATE INDEX idx_group_transactions_member_id ON group_transactions (member_id);
CREATE INDEX idx_group_transactions_created_at ON group_transactions (created_at);

-- +goose Down
DROP INDEX IF EXISTS idx_group_transactions_group_id;
DROP INDEX IF EXISTS idx_group_transactions_member_id;
DROP INDEX IF EXISTS idx_group_transactions_created_at;
DROP TABLE IF EXISTS group_transactions;
