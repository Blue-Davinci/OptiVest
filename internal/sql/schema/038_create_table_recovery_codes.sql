-- +goose Up
-- up migration for recovery_codes table
CREATE TABLE recovery_codes (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code_hash BYTEA NOT NULL,
    used BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indexes
CREATE INDEX idx_recovery_codes_user_id ON recovery_codes (user_id);
CREATE INDEX idx_recovery_codes_used ON recovery_codes (used);

-- +goose StatementBegin
CREATE TRIGGER trigger_update_recovery_tracking_timestamp
BEFORE UPDATE ON recovery_codes
FOR EACH ROW
EXECUTE FUNCTION update_timestamp();
-- +goose StatementEnd
-- +goose Down
-- down migration to drop recovery_codes table
DROP INDEX IF EXISTS idx_recovery_codes_user_id;
DROP INDEX IF EXISTS idx_recovery_codes_used;
DROP TRIGGER IF EXISTS trigger_update_recovery_tracking_timestamp ON recovery_codes;
DROP TABLE IF EXISTS recovery_codes;
