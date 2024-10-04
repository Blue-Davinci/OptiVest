
-- name: CreateNewRecurringExpense :one
INSERT INTO recurring_expenses (
    user_id, budget_id, amount,name, description, recurrence_interval,projected_amount, next_occurrence
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
)
RETURNING id, created_at, updated_at;

-- name: GetAllRecurringExpensesDueForProcessing :many
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