-- +goose Up
CREATE TABLE stock_analysis (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGSERIAL NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    stock_symbol VARCHAR(10) NOT NULL,
    returns DECIMAL[] NOT NULL,
    sharpe_ratio DECIMAL(10, 4),
    sortino_ratio DECIMAL(10, 4),
    sector_performance DECIMAL(10, 4),
    sentiment_label VARCHAR(30),
    risk_free_rate DECIMAL(10, 4),
    analysis_date TIMESTAMP(0) WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Indexes
CREATE INDEX idx_stock_analysis_user ON stock_analysis(user_id);
CREATE INDEX idx_stock_analysis_symbol ON stock_analysis(stock_symbol);
CREATE INDEX idx_stock_analysis_analysis_data ON stock_analysis(analysis_date);

-- Down Migration
-- +goose Down
DROP INDEX IF EXISTS idx_stock_analysis_user;
DROP INDEX IF EXISTS idx_stock_analysis_symbol;
DROP INDEX IF EXISTS idx_stock_analysis_analysis_data;
DROP TABLE IF EXISTS stock_analysis;
