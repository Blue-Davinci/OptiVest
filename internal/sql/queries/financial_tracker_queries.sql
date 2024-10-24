
-- name: CreateNewRecurringExpense :one
INSERT INTO recurring_expenses (
    user_id, budget_id, amount,name, description, recurrence_interval,projected_amount, next_occurrence
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
)
RETURNING id, created_at, updated_at;

-- name: GetAllRecurringExpensesDueForProcessing :many
SELECT
    COUNT(*) OVER() AS total_count,
    id, 
    user_id, 
    budget_id, 
    amount, 
    name, 
    description, 
    recurrence_interval, 
    projected_amount,
    next_occurrence, 
    created_at, 
    updated_at
FROM recurring_expenses
WHERE next_occurrence <= CURRENT_DATE
ORDER BY next_occurrence ASC
LIMIT $1 OFFSET $2;

-- name: UpdateRecurringExpenseByID :one
UPDATE recurring_expenses SET
    amount = $1,
    name = $2,
    description = $3,
    recurrence_interval = $4,
    projected_amount = $5,
    next_occurrence = $6
WHERE
    id = $7 AND user_id = $8
RETURNING  updated_at;

-- name: GetRecurringExpenseByID :one
SELECT 
    id, 
    user_id, 
    budget_id, 
    amount, 
    name, 
    description, 
    recurrence_interval, 
    projected_amount,
    next_occurrence, 
    created_at, 
    updated_at
FROM recurring_expenses
WHERE id = $1 AND user_id = $2;

-- name: CreateNewExpense :one
INSERT INTO expenses (
    user_id, 
    budget_id, 
    name,
    category,
    amount, 
    is_recurring, 
    description, 
    date_occurred
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
)
RETURNING id, created_at, updated_at;

-- name: GetExpenseByID :one
SELECT 
    id, 
    user_id, 
    budget_id, 
    name, 
    category, 
    amount, 
    is_recurring, 
    description, 
    date_occurred, 
    created_at, 
    updated_at
FROM expenses
WHERE id = $1 AND user_id = $2;

-- name: UpdateExpenseByID :one
UPDATE expenses SET
    name = $1,
    category = $2,
    amount = $3,
    is_recurring = $4,
    description = $5,
    date_occurred = $6
WHERE
    id = $7 AND user_id = $8
RETURNING updated_at;

-- name: CreateNewIncome :one
    INSERT INTO income (
        user_id, 
        source, 
        original_currency_code, 
        amount_original, 
        amount, 
        exchange_rate, 
        description, 
        date_received
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
) RETURNING id, created_at, updated_at;

-- name: CreateNewDebt :one
INSERT INTO debts (
    user_id, name, amount, remaining_balance, interest_rate, description, 
    due_date, minimum_payment, next_payment_date, estimated_payoff_date, 
    accrued_interest, interest_last_calculated, total_interest_paid
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
RETURNING id, created_at, updated_at;

-- name: UpdateIncomeByID :one
UPDATE income
SET
    source = $1,
    original_currency_code = $2,
    amount_original = $3,
    amount = $4,
    exchange_rate = $5,
    description = $6,
    date_received = $7
WHERE
    id=$8 AND user_id=$9
RETURNING updated_at;

-- name: GetIncomeByID :one
SELECT
    id,
    user_id,
    source,
    original_currency_code,
    amount_original,
    amount,
    exchange_rate,
    description,
    date_received,
    created_at,
    updated_at
FROM income
WHERE id = $1 AND user_id = $2;


-- name: UpdateDebtByID :one
UPDATE debts
SET
    name = $2,                                  -- New name
    amount = $3,                                -- New amount
    remaining_balance = $4,                     -- New remaining balance
    interest_rate = $5,                         -- New interest rate
    description = $6,                           -- New description
    due_date = $7,                              -- New due date
    minimum_payment = $8,                       -- New minimum payment
    next_payment_date = $9,                     -- New next payment date
    accrued_interest = $10,                     -- New accrued interest
    total_interest_paid = $11,                  -- New total interest paid
    estimated_payoff_date = $12,                -- New estimated payoff date
    interest_last_calculated = $13,               -- New interest last calculated date
    last_payment_date = $14                     -- New last payment date
WHERE
    id = $1 AND user_id=$15                                   -- ID of the debt to update
RETURNING updated_at;

-- name: GetAllDebtsByUserID :many
SELECT 
    id,
    user_id,
    name,
    amount,
    remaining_balance,
    interest_rate,
    description,
    due_date,
    minimum_payment,
    created_at,
    updated_at,
    next_payment_date,
    estimated_payoff_date,
    accrued_interest,
    interest_last_calculated,
    last_payment_date,
    total_interest_paid,
    COUNT(*) OVER() AS total_debts,
    CAST(SUM(amount) OVER() AS NUMERIC) AS total_amounts,                -- Cast after SUM
    CAST(SUM(remaining_balance) OVER() AS NUMERIC) AS total_remaining_balances
FROM debts
WHERE user_id = $1
AND ($2 = '' OR to_tsvector('simple', name) @@ plainto_tsquery('simple', $2))
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- name: GetDebtPaymentsByDebtUserID :many
SELECT 
    id,
    debt_id,
    user_id,
    payment_amount,
    payment_date,
    interest_payment,
    principal_payment,
    created_at,
    COUNT(*) OVER() AS total_payments,
    CAST(SUM(payment_amount) OVER() AS NUMERIC)::NUMERIC AS total_payment_amount,  -- Cast after the SUM
    CAST(SUM(interest_payment) OVER() AS NUMERIC)::NUMERIC AS total_interest_payment,
    CAST(SUM(principal_payment) OVER() AS NUMERIC)::NUMERIC AS total_principal_payment
FROM debtpayments
WHERE user_id = $1
AND debt_id = $2
AND ($3::TIMESTAMP IS NULL OR payment_date >= $3::TIMESTAMP)  -- Cast to TIMESTAMP explicitly
AND ($4::TIMESTAMP IS NULL OR payment_date <= $4::TIMESTAMP) 
ORDER BY payment_date DESC
LIMIT $5 OFFSET $6;




-- name: GetDebtByID :one
SELECT 
    id, 
    user_id, 
    name, 
    amount, 
    remaining_balance, 
    interest_rate, 
    description, 
    due_date, 
    minimum_payment, 
    created_at, 
    updated_at, 
    next_payment_date, 
    estimated_payoff_date, 
    accrued_interest, 
    interest_last_calculated, 
    last_payment_date, 
    total_interest_paid
FROM debts
WHERE id = $1;

-- name: CreateNewDebtPayment :one
INSERT INTO debtpayments (
    debt_id,
    user_id,
    payment_amount,
    payment_date,
    interest_payment,
    principal_payment
) VALUES (
    $1, -- debt_id
    $2, -- user_id
    $3, -- payment_amount
    $4, -- payment_date
    $5, -- interest_payment
    $6  -- principal_payment
)
RETURNING id, created_at;

-- name: GetAllOverdueDebts :many
SELECT 
    COUNT(*) OVER() AS total_count,
    id, 
    user_id, 
    name, 
    amount, 
    remaining_balance, 
    interest_rate, 
    description, 
    due_date, 
    minimum_payment, 
    created_at, 
    updated_at, 
    next_payment_date, 
    estimated_payoff_date, 
    accrued_interest, 
    interest_last_calculated, 
    last_payment_date, 
    total_interest_paid
FROM 
    debts
WHERE 
    remaining_balance > 0  -- Debt is not fully paid
AND (interest_last_calculated IS NULL OR interest_last_calculated < CURRENT_DATE) -- Interest calculation is overdue
LIMIT $1 OFFSET $2;

-- name: GetAllExpensesByUserID :many
SELECT 
    e.id,
    e.user_id,
    e.budget_id,
    e.name,
    e.category,
    e.amount,
    e.is_recurring,
    e.description,
    e.date_occurred,
    e.created_at,
    e.updated_at,
    COUNT(*) OVER () AS total_count
FROM 
    expenses e
WHERE e.user_id = $1  -- Filter by user ID
AND ($2 = '' OR to_tsvector('simple', e.name) @@ plainto_tsquery('simple', $2))
ORDER BY 
    e.date_occurred DESC
LIMIT 
    $3 OFFSET $4; 
