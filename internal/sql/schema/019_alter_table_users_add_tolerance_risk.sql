-- +goose Up
-- Create ENUM type for risk_tolerance
CREATE TYPE risk_tolerance_type AS ENUM ('low', 'medium', 'high');

-- Create ENUM type for time_horizon
CREATE TYPE time_horizon_type AS ENUM ('short', 'medium', 'long');

-- Alter the users table to add risk_tolerance and time_horizon columns
ALTER TABLE users
ADD COLUMN risk_tolerance risk_tolerance_type,
ADD COLUMN time_horizon time_horizon_type;

-- +goose Down
-- Drop the columns risk_tolerance and time_horizon from the users table
ALTER TABLE users
DROP COLUMN IF EXISTS risk_tolerance,
DROP COLUMN IF EXISTS time_horizon;

-- Drop ENUM type for risk_tolerance
DROP TYPE IF EXISTS risk_tolerance_type;

-- Drop ENUM type for time_horizon
DROP TYPE IF EXISTS time_horizon_type;