
-- name: GetDistinctBudgetCategory :many
SELECT DISTINCT category
FROM Budgets
WHERE user_id = $1;