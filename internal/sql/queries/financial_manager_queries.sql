-------------------------------------------------------------------------------------------------------
------------------------- Budgets
-------------------------------------------------------------------------------------------------------
-- name: CreateNewBudget :one
INSERT INTO budgets (
    user_id, 
    name, 
    is_Strict, 
    category, 
    total_amount, 
    currency_code, 
    conversion_rate, 
    description 
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, created_at, updated_at;

-- name: GetBudgetByID :one
SELECT 
    id, 
    user_id, 
    name,
    is_strict, 
    category, 
    total_amount, 
    currency_code, 
    conversion_rate,
    description, 
    created_at, 
    updated_at
FROM budgets
WHERE id = $1;

-- name: GetBudgetsForUser :many
SELECT count(*) OVER() AS total_budgets,
    id, 
    user_id, 
    name,
    is_strict, 
    category, 
    total_amount, 
    currency_code, 
    conversion_rate,
    description, 
    created_at, 
    updated_at
FROM budgets
WHERE user_id = $1
AND ($2 = '' OR to_tsvector('simple', name) @@ plainto_tsquery('simple', $2))
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- name: DeleteBudgetById :one
DELETE FROM budgets
WHERE id = $1 AND user_id = $2
RETURNING id;


-- name: UpdateBudgetById :one
UPDATE budgets
SET 
    name = $2,
    is_strict = $3,
    category = $4,
    total_amount = $5,
    currency_code = $6,
    conversion_rate = $7,
    description = $8
WHERE id = $1 and user_id = $9
RETURNING updated_at;


-------------------------------------------------------------------------------------------------------
------------------------- Goals
-------------------------------------------------------------------------------------------------------
-- name: CreateNewGoal :one
INSERT INTO goals (
    user_id, 
    budget_id, 
    name, 
    current_amount, 
    target_amount, 
    monthly_contribution, 
    start_date, 
    end_date, 
    status
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING id, created_at, updated_at;

-- name: GetAllGoalSummaryBuBudgetID :many
SELECT count(*) OVER() AS total_goals,
    g.id AS goal_id,
    g.name AS goal_name,
    g.monthly_contribution AS goal_monthly_contribution,
    g.target_amount AS goal_target_amount,
    b.total_amount AS budget_total_amount,
    b.currency_code AS budget_currency,
    b.is_strict AS is_strict,
    COALESCE(SUM(g.monthly_contribution) OVER (), 0)::numeric AS total_monthly_contributions,
   (b.total_amount - COALESCE(SUM(g.monthly_contribution) OVER (), 0))::numeric AS budget_surplus
FROM budgets b
LEFT JOIN goals g ON g.budget_id = b.id
WHERE b.id = $1 AND b.user_id = $2
GROUP BY 
    b.id, g.id
ORDER BY g.name;

-- name: GetGoalByID :one
SELECT 
    id, 
    user_id, 
    budget_id, 
    name, 
    current_amount, 
    target_amount, 
    monthly_contribution, 
    start_date, 
    end_date, 
    created_at, 
    updated_at,
    status
FROM goals
WHERE id = $1 AND user_id = $2;

-- name: UpdateGoalByID :one
UPDATE goals
SET 
    name = $1,
    current_amount = $2,
    target_amount = $3,
    monthly_contribution = $4,
    start_date = $5,
    end_date = $6,
    status = $7
WHERE id = $8 AND user_id = $9
RETURNING updated_at;

-- name: GetAndSaveAllGoalsForTracking :many
-- Insert tracked goals that haven't been tracked for more than 1 month
INSERT INTO goal_tracking (user_id, goal_id, contributed_amount, tracking_type)
SELECT g.user_id, g.id, g.monthly_contribution, 'monthly'
FROM goals g
LEFT JOIN goal_tracking gt ON g.id = gt.goal_id 
   AND gt.truncated_tracking_date = date_trunc('month', CURRENT_DATE)::date
WHERE gt.goal_id IS NULL
  AND g.status = 'ongoing' 
  AND g.start_date < CURRENT_DATE
ORDER BY truncated_tracking_date ASC
RETURNING id, user_id, goal_id, contributed_amount;

-- name: CreateNewGoalPlan :one
INSERT INTO goal_plans (
    user_id, 
    name, 
    description, 
    target_amount, 
    monthly_contribution, 
    duration_in_months, 
    is_strict
) VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, created_at, updated_at;

-- name: UpdateGoalPlanByID :one
UPDATE goal_plans SET
    name = $1,
    description = $2,
    target_amount = $3,
    monthly_contribution = $4,
    duration_in_months = $5,
    is_strict = $6
WHERE
    id = $7 AND user_id = $8
RETURNING updated_at;

-- name: GetGoalPlanByID :one
SELECT 
    id, 
    user_id, 
    name, 
    description, 
    target_amount, 
    monthly_contribution, 
    duration_in_months, 
    is_strict, 
    created_at, 
    updated_at
FROM goal_plans
WHERE id = $1 AND user_id = $2;

-- name: GetGoalPlansForUser :many
SELECT count(*) OVER() AS total_goal_plans,
    id, 
    user_id, 
    name, 
    description, 
    target_amount, 
    monthly_contribution, 
    duration_in_months, 
    is_strict, 
    created_at, 
    updated_at
FROM goal_plans
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: UpdateGoalProgressOnExpiredGoals :exec
UPDATE goals
SET status = 'completed',
    updated_at = NOW()
WHERE current_amount >= target_amount
  AND status = 'ongoing';
