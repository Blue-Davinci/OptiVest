
-- name: GetDistinctBudgetCategory :many
SELECT DISTINCT category
FROM Budgets
WHERE user_id = $1;

-- name: GetDistinctBudgetIdBudgetName :many
SELECT id, name
FROM Budgets
WHERE user_id = $1;