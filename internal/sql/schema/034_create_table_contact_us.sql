
-- +goose Up
CREATE TYPE contact_us_status AS ENUM ('pending', 'in progress', 'resolved');
CREATE TABLE contact_us (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES users(id) ON DELETE SET NULL, -- Allows nullable user association for anonymous submissions
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255) NOT NULL,
    subject VARCHAR(255) NOT NULL,
    message TEXT NOT NULL,
    status contact_us_status DEFAULT 'pending', -- e.g., pending, in progress, resolved
    created_at TIMESTAMP(0) WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP(0) WITH TIME ZONE DEFAULT NOW(),
    version INT DEFAULT 1
);

-- Indexes for optimizing queries
CREATE INDEX idx_contact_us_user_id ON contact_us(user_id);
CREATE INDEX idx_contact_us_status ON contact_us(status);
CREATE INDEX idx_contact_us_created_at_status ON contact_us(created_at DESC, status);
CREATE INDEX idx_contact_us_email ON contact_us(email); -- Optional, for queries filtering by email


-- +goose StatementBegin
CREATE TRIGGER trigger_update_contact_us_tracking_timestamp
BEFORE UPDATE ON contact_us
FOR EACH ROW
EXECUTE FUNCTION update_timestamp();
-- +goose StatementEnd

-- +goose Down
DROP INDEX idx_contact_us_user_id;
DROP INDEX idx_contact_us_status;
DROP INDEX idx_contact_us_created_at_status;
DROP INDEX idx_contact_us_email;
DROP TRIGGER trigger_update_contact_us_tracking_timestamp ON contact_us;
DROP TABLE contact_us;
DROP TYPE contact_us_status;
