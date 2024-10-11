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

-- name: DeleteStockInvestmentByID :one
DELETE FROM stock_investments
WHERE id = $1 AND user_id = $2
RETURNING id;


-- name: CreateNewBondInvestment :one
INSERT INTO bond_investments (
    user_id, 
    bond_symbol, 
    quantity, 
    purchase_price, 
    current_value, 
    coupon_rate, 
    maturity_date, 
    purchase_date
) 
VALUES ($1,$2,$3,$4,$5,$6,$7,$8            
) RETURNING id, created_at, updated_at;

-- name: UpdateBondInvestment :one
UPDATE bond_investments
SET
    quantity = $1,
    purchase_price = $2,
    current_value = $3,
    coupon_rate = $4,
    maturity_date = $5,
    purchase_date = $6
WHERE id = $7
AND user_id = $8
RETURNING updated_at;

-- name: GetBondByBondID :one
SELECT
    id,
    user_id,
    bond_symbol,
    quantity,
    purchase_price,
    current_value,
    coupon_rate,
    maturity_date,
    purchase_date,
    created_at,
    updated_at
FROM bond_investments
WHERE id = $1;

-- name: DeleteBondInvestmentByID :one
DELETE FROM bond_investments
WHERE id = $1 AND user_id = $2
RETURNING id;

-- name: CreateNewAlternativeInvestment :one
INSERT INTO alternative_investments (
    user_id,
    investment_type,
    investment_name,
    is_business,
    quantity,
    annual_revenue,
    acquired_at,
    profit_margin,
    valuation,
    location
) 
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING id, valuation_updated_at,created_at, updated_at;

-- name: UpdateAlternativeInvestment :one
UPDATE alternative_investments
SET
    investment_type = $1,
    investment_name = $2,
    is_business = $3,
    quantity = $4,
    annual_revenue = $5,
    acquired_at = $6,
    profit_margin = $7,
    valuation = $8,
    valuation_updated_at = $9,
    location = $10
WHERE id = $11
AND user_id = $12
RETURNING valuation_updated_at, updated_at;

-- name: GetAlternativeInvestmentByAlternativeID :one
SELECT
    id,
    user_id,
    investment_type,
    investment_name,
    is_business,
    quantity,
    annual_revenue,
    acquired_at,
    profit_margin,
    valuation,
    valuation_updated_at,
    location,
    created_at,
    updated_at
FROM alternative_investments
WHERE id = $1;

-- name: DeleteAlternativeInvestmentByID :one
DELETE FROM alternative_investments
WHERE id = $1 AND user_id = $2
RETURNING id;

-- name: CreateNewInvestmentTransaction :one
INSERT INTO investment_transactions (
    user_id,
    investment_type,
    investment_id,
    transaction_type,
    transaction_date,
    transaction_amount,
    quantity
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, created_at, updated_at;

-- name: DeleteInvestmentTransactionByID :one
DELETE FROM investment_transactions
WHERE id = $1 AND user_id = $2
RETURNING id;

-- name: GetAllInvestmentsByUserID :many
-- name: GetAllInvestmentsByUserID :many
SELECT 
    'stock' AS investment_type,
    jsonb_agg(
        jsonb_build_object(
            'stock_symbol', s.stock_symbol,
            'quantity', s.quantity,
            'purchase_price', s.purchase_price,
            'sector', s.sector,
            'dividend_yield', s.dividend_yield
        )
    ) AS investments
FROM stock_investments s
WHERE s.user_id = $1

UNION ALL

SELECT 
    'bond' AS investment_type,
    jsonb_agg(
        jsonb_build_object(
            'bond_symbol', b.bond_symbol,
            'quantity', b.quantity,
            'purchase_price', b.purchase_price,
            'coupon_rate', b.coupon_rate,
            'maturity_date', b.maturity_date
        )
    ) AS investments
FROM bond_investments b
WHERE b.user_id = $1

UNION ALL

SELECT 
    'alternative' AS investment_type,
    jsonb_agg(
        jsonb_build_object(
            'investment_type', a.investment_type,
            'investment_name', a.investment_name,
            'quantity', a.quantity,
            'valuation', a.valuation,
            'annual_revenue', a.annual_revenue,
            'profit_margin', a.profit_margin
        )
    ) AS investments
FROM alternative_investments a
WHERE a.user_id = $1;

