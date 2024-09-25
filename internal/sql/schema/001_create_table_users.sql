-- +goose Up
CREATE EXTENSION IF NOT EXISTS citext;

-- Create ENUM type for MFA status
CREATE TYPE mfa_status_type AS ENUM ('pending', 'accepted', 'rejected');

CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,        -- Unique user ID
    first_name VARCHAR(50) NOT NULL,          -- First name 
    last_name VARCHAR(50) NOT NULL,           -- Last name
    email CITEXT UNIQUE NOT NULL,    -- Case-insensitive email, must be unique
    profile_avatar_url TEXT NOT NULL,    -- URL to user's profile picture
    password BYTEA NOT NULL,    -- Securely stored password hash (bcrypt or argon2 recommended)
    user_role VARCHAR(15) NOT NULL DEFAULT 'user', -- User role (e.g., admin, user, moderator)
    phone_number TEXT NOT NULL,      -- phone number for multi-factor authentication (MFA)
    activated BOOLEAN DEFAULT FALSE NOT NULL, -- Account activation status (email confirmation)
    version INTEGER DEFAULT 1 NOT NULL,       -- Record versioning for optimistic locking
    created_at TIMESTAMP(0) WITH TIME ZONE NOT NULL DEFAULT NOW(), -- Timestamp of account creation
    updated_at TIMESTAMP(0) WITH TIME ZONE NOT NULL DEFAULT NOW(), -- Timestamp for last update (e.g., profile changes)
    last_login TIMESTAMP(0) WITH TIME ZONE NOT NULL DEFAULT NOW(), -- Track the user's last login time
    profile_completed BOOLEAN DEFAULT FALSE NOT NULL, -- Whether the user completed full profile
    dob DATE NOT NULL,              -- Date of Birth (for financial regulations)
    address TEXT,                   -- Optional address for KYC requirements
    country_code CHAR(2),           -- Two-letter ISO country code for region-specific financial services
    currency_code CHAR(3),          -- Default currency (ISO 4217) for transactions and accounts
    mfa_enabled BOOLEAN DEFAULT FALSE NOT NULL, -- Multi-factor authentication (MFA) enabled
    mfa_secret TEXT,                -- Secret key for TOTP-based MFA
    mfa_status mfa_status_type DEFAULT 'pending', -- Status of MFA (pending, accepted, rejected)
    mfa_last_checked TIMESTAMP(0) WITH TIME ZONE -- Timestamp of the last MFA check
);

-- Create indexes
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_phone_number ON users(phone_number);
CREATE INDEX idx_users_last_login ON users(last_login);
CREATE INDEX idx_users_user_role ON users(user_role);
CREATE INDEX idx_users_country_code ON users(country_code);
CREATE INDEX idx_users_currency_code ON users(currency_code);
CREATE INDEX idx_users_mfa_status ON users(mfa_status);


-- +goose Down
DROP INDEX IF EXISTS idx_users_email;
DROP INDEX IF EXISTS idx_users_phone_number;
DROP INDEX IF EXISTS idx_users_last_login;
DROP INDEX IF EXISTS idx_users_country_code;
DROP INDEX IF EXISTS idx_users_currency_code;
DROP INDEX IF EXISTS idx_users_mfa_status;

DROP TABLE users;

-- Drop the ENUM type on downgrade
DROP TYPE IF EXISTS mfa_status_type;
