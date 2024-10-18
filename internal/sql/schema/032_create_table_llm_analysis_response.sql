-- +goose Up
-- Create the llm_analysis_responses table
CREATE TABLE llm_analysis_responses (
    id BIGSERIAL PRIMARY KEY, -- Auto-incrementing ID
    user_id BIGSERIAL NOT NULL REFERENCES users(id) ON DELETE CASCADE, -- Foreign key to users table
    created_at TIMESTAMP(0) WITH TIME ZONE NOT NULL DEFAULT NOW(), -- Timestamp for when the analysis is created
    header TEXT, -- Store the Header as text
    analysis JSONB NOT NULL, -- Store the Analysis as JSONB for flexible querying
    footer TEXT -- Store the Footer as text
);

-- Create an index on user_id for faster queries filtering by user
CREATE INDEX idx_llm_analysis_responses_user_id ON llm_analysis_responses(user_id);

-- Create an index on created_at for faster queries filtering by creation time
CREATE INDEX idx_llm_analysis_responses_created_at ON llm_analysis_responses(created_at);

-- Create a GIN index on the JSONB 'analysis' column for better performance in JSONB queries
CREATE INDEX idx_llm_analysis_responses_analysis_gin ON llm_analysis_responses USING GIN (analysis);

-- +goose Down
-- Down Migration
DROP INDEX IF EXISTS idx_llm_analysis_responses_user_id;
DROP INDEX IF EXISTS idx_llm_analysis_responses_created_at;
DROP INDEX IF EXISTS idx_llm_analysis_responses_analysis_gin;
DROP TABLE IF EXISTS llm_analysis_responses;
