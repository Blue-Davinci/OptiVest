-- +goose Up
CREATE TABLE bond_analysis (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGSERIAL NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    bond_symbol VARCHAR(10) NOT NULL,
    ytm DECIMAL(10, 4),
    current_yield DECIMAL(10, 4),
    macaulay_duration DECIMAL(10, 4),
    convexity DECIMAL(10, 4),
    bond_returns DECIMAL[] NOT NULL,
    annual_return DECIMAL(10, 4),
    bond_volatility DECIMAL(10, 4),
    sharpe_ratio DECIMAL(10, 4),
    sortino_ratio DECIMAL(10, 4),
    risk_free_rate DECIMAL(10, 4),
    analysis_date TIMESTAMP(0) WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Indexes
CREATE INDEX idx_bond_analysis_user ON bond_analysis(user_id);
CREATE INDEX idx_bond_analysis_symbol ON bond_analysis(bond_symbol);
CREATE INDEX idx_bond_analysis_analysis_date ON bond_analysis(analysis_date);

-- Down Migration
-- +goose Down
DROP INDEX IF EXISTS idx_bond_analysis_user;
DROP INDEX IF EXISTS idx_bond_analysis_symbol;
DROP INDEX IF EXISTS idx_bond_analysis_analysis_date;
DROP TABLE IF EXISTS bond_analysis;
