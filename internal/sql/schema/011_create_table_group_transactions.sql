-- +goose Up
CREATE TABLE group_transactions (
    id BIGSERIAL PRIMARY KEY,                                  -- Unique transaction ID
    goal_id BIGINT REFERENCES group_goals(id) ON DELETE CASCADE, -- Reference to the group goal
    member_id BIGINT REFERENCES users(id) ON DELETE CASCADE,    -- User/member who made the transaction
    amount NUMERIC(12, 2) NOT NULL CHECK (amount > 0),          -- Amount involved in the transaction
    description TEXT,                                           -- Optional description of the transaction
    created_at TIMESTAMP(0) WITH TIME ZONE DEFAULT NOW(),                       -- Time the transaction was created
    updated_at TIMESTAMP(0) WITH TIME ZONE DEFAULT NOW()                       -- Time the transaction was last updated
);

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION update_goal_amount()
RETURNS TRIGGER AS $$
BEGIN
    -- Check if it's an insert or delete operation
    IF (TG_OP = 'INSERT') THEN
        -- Add the amount on insert
        UPDATE group_goals
        SET current_amount = current_amount + NEW.amount,
            status = CASE 
                        WHEN current_amount + NEW.amount >= target_amount THEN 'completed' 
                        ELSE status 
                     END
        WHERE id = NEW.goal_id;
    
    ELSIF (TG_OP = 'DELETE') THEN
        -- Subtract the amount on delete
        UPDATE group_goals
        SET current_amount = current_amount - OLD.amount,
            status = CASE 
                        WHEN current_amount - OLD.amount < target_amount THEN 'ongoing'
                        ELSE status 
                     END
        WHERE id = OLD.goal_id;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd



-- +goose StatementBegin
CREATE TRIGGER trigger_update_goals_tracking_timestamp
BEFORE UPDATE ON group_transactions
FOR EACH ROW
EXECUTE FUNCTION update_timestamp();
-- +goose StatementEnd

-- Trigger to update the goal's current amount when a new transaction is inserted
CREATE TRIGGER trigger_update_goal_amount
AFTER INSERT ON group_transactions
FOR EACH ROW
EXECUTE FUNCTION update_goal_amount();

-- Trigger to update goal's current amount when a transaction is deleted
CREATE TRIGGER trigger_update_goal_amount_delete
AFTER DELETE ON group_transactions
FOR EACH ROW
EXECUTE FUNCTION update_goal_amount();

-- Indexes for optimization
CREATE INDEX idx_group_transactions_goal_id ON group_transactions (goal_id);
CREATE INDEX idx_group_transactions_member_id ON group_transactions (member_id);
CREATE INDEX idx_group_transactions_created_at ON group_transactions (created_at);

-- +goose Down
DROP TRIGGER IF EXISTS trigger_update_goal_amount ON group_transactions;
DROP INDEX IF EXISTS idx_group_transactions_goal_id;
DROP INDEX IF EXISTS idx_group_transactions_member_id;
DROP INDEX IF EXISTS idx_group_transactions_created_at;
DROP TABLE IF EXISTS group_transactions;
