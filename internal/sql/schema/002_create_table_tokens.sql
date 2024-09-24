-- +goose Up
CREATE TABLE IF NOT EXISTS tokens (
    hash bytea PRIMARY KEY, -- Unique token hash
    user_id bigint NOT NULL REFERENCES users ON DELETE CASCADE, -- Foreign key to users table
    expiry timestamp(0) with time zone NOT NULL, -- Token expiration timestamp
    scope text NOT NULL -- Token scope (e.g., "activation", "password_reset")
);

-- Create indexes
CREATE INDEX idx_tokens_user_id ON tokens(user_id);
CREATE INDEX idx_tokens_expiry ON tokens(expiry);
CREATE INDEX idx_tokens_scope ON tokens(scope);

-- +goose Down
DROP INDEX IF EXISTS idx_tokens_user_id;
DROP INDEX IF EXISTS idx_tokens_expiry;
DROP INDEX IF EXISTS idx_tokens_scope;

DROP TABLE IF EXISTS tokens;