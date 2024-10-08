-- name: CreateNewStockInvestment :one
INSERT INTO
    stock_investments (
        user_id,
        stock_symbol,
        quantity,
        purchase_price,
        current_value,
        sector,
        purchase_date,
        dividend_yield
    )
VALUES
    ($1, $2, $3, $4, $5, $6, $7, $8) 
RETURNING id,dividend_yield_updated_at, created_at,updated_at;

-- name: UpdateStockInvestment :one
UPDATE stock_investments
SET
    quantity = $1,
    purchase_price = $2,
    current_value = $3,
    sector = $4,
    purchase_date = $5,
    dividend_yield = $6,
    dividend_yield_updated_at = $7
WHERE id = $8
AND user_id = $9
RETURNING dividend_yield_updated_at,updated_at;

-- name: GetStockByStockID :one
SELECT
    id,
    user_id,
    stock_symbol,
    quantity,
    purchase_price,
    current_value,
    sector,
    purchase_date,
    dividend_yield,
    dividend_yield_updated_at,
    created_at,
    updated_at
FROM stock_investments
WHERE id = $1;

-- name: GetStockInvestmentByUserIDAndStockSymbol :one
SELECT
    id,
    user_id,
    stock_symbol,
    quantity,
    purchase_price,
    current_value,
    sector,
    purchase_date,
    dividend_yield,
    dividend_yield_updated_at,
    created_at,
    updated_at
FROM stock_investments
WHERE user_id = $1
AND stock_symbol = $2;
