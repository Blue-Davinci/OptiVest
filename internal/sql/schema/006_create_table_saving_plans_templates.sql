-- +goose Up
CREATE TABLE saving_plans (
    id BIGSERIAL PRIMARY KEY,                                  -- Unique ID for each template
    user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,      -- User-specific saving plan
    name VARCHAR(255) NOT NULL,                                -- Name of the saving plan (e.g., "Default Plan")
    target_amount NUMERIC(20, 2),                              -- Target amount for the plan (optional)
    monthly_contribution NUMERIC(20, 2),                       -- Monthly contribution towards savings (optional)
    duration_in_months INTEGER,                                -- Duration for saving (optional)
    created_at TIMESTAMP(0) WITH TIME ZONE DEFAULT NOW(),            -- When the template was created
    updated_at TIMESTAMP(0) WITH TIME ZONE DEFAULT NOW()            -- When the template was last updated
);

-- Index for user_id to quickly fetch saving plans
CREATE INDEX idx_saving_plans_user_id ON saving_plans (user_id);

-- +goose Down
DROP INDEX IF EXISTS idx_saving_plans_user_id;
DROP TABLE IF EXISTS saving_plans;
