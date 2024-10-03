-- +goose Up
CREATE TABLE group_expenses (
    id BIGSERIAL PRIMARY KEY,                                  -- Unique expense ID
    group_id BIGINT REFERENCES groups(id) ON DELETE CASCADE,    -- Reference to the group
    member_id BIGINT REFERENCES users(id) ON DELETE CASCADE,    -- Reference to the member who made the expense
    amount NUMERIC(12, 2) NOT NULL CHECK (amount > 0),          -- Amount of the expense
    description TEXT,                                           -- Optional description of the expense
    category VARCHAR(100),                                      -- Category of the expense (e.g., 'operations', 'purchase', etc.)
    created_at TIMESTAMP(0) WITH TIME ZONE DEFAULT NOW(),       -- Time when the expense was created
    updated_at TIMESTAMP(0) WITH TIME ZONE DEFAULT NOW()        -- Time when the expense was last updated
);

-- Indexes for optimization
CREATE INDEX idx_group_expenses_group_id ON group_expenses (group_id);
CREATE INDEX idx_group_expenses_member_id ON group_expenses (member_id);
CREATE INDEX idx_group_expenses_created_at ON group_expenses (created_at);

-- +goose Down
DROP INDEX IF EXISTS idx_group_expenses_group_id;
DROP INDEX IF EXISTS idx_group_expenses_member_id;
DROP INDEX IF EXISTS idx_group_expenses_created_at;
DROP TABLE IF EXISTS group_expenses;
