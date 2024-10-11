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

-- name: GetAllGoalSummaryByBudgetID :many
WITH 
    -- Calculate total non-recurring expenses
    NonRecurringExpenses AS (
        SELECT 
            COALESCE(SUM(e.amount), 0) AS total_expenses
        FROM expenses e
        WHERE e.budget_id = $1
        AND e.is_recurring = FALSE
        AND e.created_at >= DATE_TRUNC('month', CURRENT_DATE)
    ),

    -- Calculate projected recurring expenses
    RecurringExpenses AS (
        SELECT 
            COALESCE(SUM(
                r.amount * 
                CASE 
                    WHEN r.recurrence_interval = 'daily' THEN 30
                    WHEN r.recurrence_interval = 'weekly' THEN 4
                    WHEN r.recurrence_interval = 'monthly' THEN 1
                    ELSE 0
                END
            ), 0) AS projected_recurring_expenses
        FROM recurring_expenses r
        WHERE r.budget_id = $1
    ),

    -- Calculate total monthly contributions from goals
    MonthlyContributions AS (
        SELECT 
            COALESCE(SUM(g.monthly_contribution), 0) AS total_monthly_contributions
        FROM goals g
        WHERE g.budget_id = $1
    )

-- Final query combining all the calculations
SELECT 
    CAST(b.total_amount AS NUMERIC) AS total_amount,
    mc.total_monthly_contributions,
    nr.total_expenses,
    re.projected_recurring_expenses,

    -- Budget surplus calculation
    CAST(
        b.total_amount - (
            mc.total_monthly_contributions + 
            nr.total_expenses + 
            re.projected_recurring_expenses
        ) AS NUMERIC
    ) AS budget_surplus

FROM budgets b
LEFT JOIN MonthlyContributions mc ON TRUE
LEFT JOIN NonRecurringExpenses nr ON TRUE
LEFT JOIN RecurringExpenses re ON TRUE
WHERE b.id = $1
AND b.user_id = $2;




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

-- name: GetGoalsForUserInvestmentHelper :many
SELECT
    name,
    current_amount,
    target_amount,
    monthly_contribution,
    start_date,
    end_date
FROM goals
WHERE user_id = $1
AND status = 'ongoing';

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
