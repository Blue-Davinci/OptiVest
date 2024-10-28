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

-- name: GetAllStockInvestmentByUserID :many
SELECT 
    si.id AS stock_id,
    si.stock_symbol,
    si.quantity,
    si.purchase_price,
    si.current_value,
    si.sector,
    si.purchase_date,
    si.dividend_yield,
    si.dividend_yield_updated_at,
    si.created_at AS stock_created_at,
    si.updated_at AS stock_updated_at,

    -- Total count of stocks for the user
    (SELECT COUNT(*) 
     FROM stock_investments si2 
     WHERE si2.user_id = $1) AS total_stocks,

    -- Sum of all transactions (transaction_amount) for the specific stock
    (SELECT COALESCE(SUM(it.transaction_amount), 0)::NUMERIC
     FROM investment_transactions it
     WHERE it.investment_id = si.id 
       AND it.investment_type = 'Stock'
       AND it.user_id = si.user_id) AS total_transaction_sum,

    -- Sum of all purchase prices for the specific stock
    (SELECT COALESCE(SUM(si2.purchase_price * si2.quantity), 0)::NUMERIC
     FROM stock_investments si2
     WHERE si2.user_id = si.user_id
       AND si2.stock_symbol = si.stock_symbol) AS total_purchase_price_sum,

    -- Transaction details as JSON array, matching InvestmentTransaction struct
COALESCE((
    SELECT json_agg(json_build_object(
            'id', it.id,
            'user_id', it.user_id,
            'investment_type', it.investment_type,
            'investment_id', it.investment_id,
            'transaction_type', it.transaction_type,
            'transaction_date', it.transaction_date,
            'transaction_amount', it.transaction_amount,
            'quantity', it.quantity,
            'created_at', it.created_at,
            'updated_at', it.updated_at
        ))
    FROM investment_transactions it
    WHERE it.investment_id = si.id 
      AND it.investment_type = 'Stock'
      AND it.user_id = si.user_id
), '[]'::json) AS transactions,

    -- Stock analysis details as JSON object, matching StockAnalysis struct
COALESCE((
        SELECT json_build_object(
            'stock_symbol', sa.stock_symbol,
            'quantity', si.quantity,
            'purchase_price', si.purchase_price,
            'sector', si.sector,
            'dividend_yield', si.dividend_yield,
            'returns', sa.returns,
            'sharpe_ratio', sa.sharpe_ratio,
            'sortino_ratio', sa.sortino_ratio,
            'sentiment_label', sa.sentiment_label,
            'sector_performance', sa.sector_performance
        )
        FROM stock_analysis sa
        WHERE sa.stock_symbol = si.stock_symbol 
        AND sa.user_id = si.user_id
        ORDER BY sa.analysis_date DESC
        LIMIT 1
), '{}'::json) AS stock_analysis

FROM 
    stock_investments si

WHERE 
    si.user_id = $1
    AND ($2 = '' OR to_tsvector('simple', si.stock_symbol) @@ plainto_tsquery('simple', $2))

ORDER BY 
    si.updated_at DESC  
LIMIT $3
OFFSET $4;


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

-- name: GetAllBondInvestmentByUserID :many
SELECT 
    bi.id AS bond_id,
    bi.bond_symbol,
    bi.quantity,
    bi.purchase_price,
    bi.current_value,
    bi.coupon_rate,
    bi.maturity_date,
    bi.purchase_date,
    bi.created_at AS bond_created_at,
    bi.updated_at AS bond_updated_at,

    -- Total count of bonds for the user
    (SELECT COUNT(*) 
     FROM bond_investments bi2 
     WHERE bi2.user_id = $1) AS total_bonds,

    -- Sum of all transactions (transaction_amount) for the specific bond
    (SELECT COALESCE(SUM(it.transaction_amount), 0)::NUMERIC
     FROM investment_transactions it
     WHERE it.investment_id = bi.id 
       AND it.investment_type = 'Bond'
       AND it.user_id = bi.user_id) AS total_transaction_sum,

    -- Sum of all purchase prices for the specific bond
    (SELECT COALESCE(SUM(bi2.purchase_price * bi2.quantity), 0)::NUMERIC
     FROM bond_investments bi2
     WHERE bi2.user_id = bi.user_id
       AND bi2.bond_symbol = bi.bond_symbol) AS total_purchase_price_sum,

    -- Transaction details as JSON array, matching InvestmentTransaction struct
    COALESCE((
        SELECT json_agg(json_build_object(
                'id', it.id,
                'user_id', it.user_id,
                'investment_type', it.investment_type,
                'investment_id', it.investment_id,
                'transaction_type', it.transaction_type,
                'transaction_date', it.transaction_date,
                'transaction_amount', it.transaction_amount,
                'quantity', it.quantity,
                'created_at', it.created_at,
                'updated_at', it.updated_at
            ))
        FROM investment_transactions it
        WHERE it.investment_id = bi.id 
          AND it.investment_type = 'Bond'
          AND it.user_id = bi.user_id
    ), '[]'::json) AS transactions,

    -- Bond analysis details as JSON object, matching BondAnalysisStatistics struct
COALESCE((
    SELECT json_build_object(
        'ytm', ba.ytm,
        'current_yield', ba.current_yield,
        'macaulay_duration', ba.macaulay_duration,
        'convexity', ba.convexity,
        'bond_returns', ba.bond_returns,
        'annual_return', ba.annual_return,
        'bond_volatility', ba.bond_volatility,
        'sharpe_ratio', ba.sharpe_ratio,
        'sortino_ratio', ba.sortino_ratio,
        'risk_free_rate', ba.risk_free_rate,
        'analysis_date', ba.analysis_date
    )
    FROM bond_analysis ba
    WHERE ba.bond_symbol = bi.bond_symbol 
      AND ba.user_id = bi.user_id
    ORDER BY ba.analysis_date DESC
    LIMIT 1
), '{}'::json) AS bond_analysis

FROM 
    bond_investments bi

WHERE 
    bi.user_id = $1
    AND ($2 = '' OR to_tsvector('simple', bi.bond_symbol) @@ plainto_tsquery('simple', $2))

ORDER BY 
    bi.updated_at DESC  
LIMIT $3
OFFSET $4;


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

-- name: GetAllAlternativeInvestmentByUserID :many
SELECT 
    ai.id AS investment_id,
    ai.investment_type,
    ai.investment_name,
    ai.is_business,
    ai.quantity,
    ai.annual_revenue,
    ai.acquired_at,
    ai.profit_margin,
    ai.valuation,
    ai.valuation_updated_at,
    ai.location,
    ai.created_at AS investment_created_at,
    ai.updated_at AS investment_updated_at,

    -- Total count of alternative investments for the user
    (SELECT COUNT(*) 
     FROM alternative_investments ai2 
     WHERE ai2.user_id = $1) AS total_alternative_investments,

    -- Sum of all transaction amounts for the specific alternative investment
    (SELECT COALESCE(SUM(it.transaction_amount), 0)::NUMERIC
     FROM investment_transactions it
     WHERE it.investment_id = ai.id 
       AND it.investment_type = 'Alternative'
       AND it.user_id = ai.user_id) AS total_transaction_sum,

    -- Sum of all valuations for the specific alternative investment type for this user
    (SELECT COALESCE(SUM(ai2.valuation), 0)::NUMERIC
     FROM alternative_investments ai2
     WHERE ai2.user_id = ai.user_id
       AND ai2.investment_type = ai.investment_type) AS total_valuation_sum,

    -- Transaction details as JSON array, matching InvestmentTransaction struct
    COALESCE((
        SELECT json_agg(json_build_object(
                'id', it.id,
                'user_id', it.user_id,
                'investment_type', it.investment_type,
                'investment_id', it.investment_id,
                'transaction_type', it.transaction_type,
                'transaction_date', it.transaction_date,
                'transaction_amount', it.transaction_amount,
                'quantity', it.quantity,
                'created_at', it.created_at,
                'updated_at', it.updated_at
            ))
        FROM investment_transactions it
        WHERE it.investment_id = ai.id 
          AND it.investment_type = 'Alternative'
          AND it.user_id = ai.user_id
    ), '[]'::json) AS transactions

FROM 
    alternative_investments ai

WHERE 
    ai.user_id = $1
    AND ($2 = '' OR to_tsvector('simple', ai.investment_type || ' ' || COALESCE(ai.investment_name, '')) @@ plainto_tsquery('simple', $2))

ORDER BY 
    ai.updated_at DESC  
LIMIT $3
OFFSET $4;


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


-- name: CreateStockAnalysis :one
INSERT INTO stock_analysis (
    user_id,
    stock_symbol,
    returns,
    sharpe_ratio,
    sortino_ratio,
    sector_performance,
    sentiment_label,
    risk_free_rate
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
)
RETURNING id, analysis_date;

-- name: CreateBondAnalysis :one
INSERT INTO bond_analysis (
    user_id,
    bond_symbol,
    ytm,
    current_yield,
    macaulay_duration,
    convexity,
    bond_returns,
    annual_return,
    bond_volatility,
    sharpe_ratio,
    sortino_ratio,
    risk_free_rate
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
)
RETURNING id, analysis_date;

-- name: CreateLLMAnalysisResponse :one
INSERT INTO llm_analysis_responses (
    user_id,
    header,
    analysis,
    footer
) VALUES ($1, $2, $3,$4  ) 
RETURNING id;

-- name: GetLatestLLMAnalysisResponseByUserID :one
SELECT
    id,
    user_id,
    header,
    analysis,
    footer,
    created_at
FROM llm_analysis_responses
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT 1;


-- name: GetAllInvestmentInfoByUserID :many
WITH stock_data AS (
    SELECT 'stock' AS investment_type, 
           stock_symbol AS symbol, 
           array_to_string(returns, ',') AS returns, -- Convert numeric[] to TEXT
           sharpe_ratio, 
           sortino_ratio, 
           COALESCE(sector_performance, 0) AS sector_performance, -- Default to 0 if NULL
           COALESCE(sentiment_label, 'No Sentiment') AS sentiment_label -- Default to 'No Sentiment' if NULL
    FROM stock_analysis
    WHERE stock_analysis.user_id = $1
),
bond_data AS (
    SELECT 'bond' AS investment_type, 
           bond_symbol AS symbol, 
           COALESCE(NULL::TEXT, 'No Returns') AS returns, -- Default to 'No Returns' if NULL
           sharpe_ratio, 
           sortino_ratio, 
           COALESCE(NULL::DECIMAL(10, 4), 0) AS sector_performance, -- Default to 0 if NULL
           COALESCE(NULL::VARCHAR(30), 'No Sentiment') AS sentiment_label -- Default to 'No Sentiment' if NULL
    FROM bond_analysis
    WHERE bond_analysis.user_id = $1
),
alternative_investment_data AS (
    SELECT 'alternative' AS investment_type, 
           COALESCE(investment_name, 'N\A') AS symbol,  -- Use investment_name as the symbol for alternative investments
           COALESCE(NULL::TEXT, 'No Returns') AS returns, -- Default to 'No Returns' if NULL
           COALESCE(NULL::DECIMAL(10, 4), 0) AS sharpe_ratio,  -- Default to 0 if NULL
           COALESCE(NULL::DECIMAL(10, 4), 0) AS sortino_ratio,  -- Default to 0 if NULL
           COALESCE(profit_margin, 0) AS sector_performance,  -- Default to 0 if NULL
           COALESCE(NULL::VARCHAR(30), 'No Sentiment') AS sentiment_label -- Default to 'No Sentiment' if NULL
    FROM alternative_investments
    WHERE alternative_investments.user_id = $1
)
SELECT *
FROM stock_data
UNION ALL
SELECT *
FROM bond_data
UNION ALL
SELECT *
FROM alternative_investment_data;


