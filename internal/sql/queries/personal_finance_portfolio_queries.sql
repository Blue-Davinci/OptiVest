-- name: GetAllFinanceDetailsForAnalysisByUserID :many
WITH total_incomes AS (
    SELECT 
        'income' AS type,
        jsonb_agg(
            jsonb_build_object(
                'income_source', i.source,
                'amount', i.amount,
                'date_received', i.date_received
            )
        ) AS details,
        SUM(i.amount)::numeric AS total_amount  -- Cast to numeric explicitly
    FROM income i
    WHERE i.user_id = $1
    AND date_trunc('month', i.date_received) = date_trunc('month', CURRENT_DATE)
),
total_expenses AS (
    SELECT 
        'expense' AS type,
        jsonb_agg(
            jsonb_build_object(
                'expense_name', e.name,
                'category', e.category,
                'amount', e.amount,
                'is_recurring', e.is_recurring,
                'budget_name', b.name
            )
        ) AS details,
        SUM(e.amount)::numeric AS total_amount  -- Cast to numeric explicitly
    FROM expenses e
    JOIN budgets b ON e.budget_id = b.id
    WHERE e.user_id = $1
    AND e.category != 'recurring'  -- Exclude recurring expenses
    AND date_trunc('month', e.date_occurred) = date_trunc('month', CURRENT_DATE)
),
total_recurring_expenses AS (
    SELECT 
        'recurring_expense' AS type,
        jsonb_agg(
            jsonb_build_object(
                'expense_name', re.name,
                'amount', re.amount,
                'projected_monthly_amount', re.projected_amount,
                'recurrence_interval', re.recurrence_interval,
                'budget_name', b.name
            )
        ) AS details,
        SUM(re.projected_amount)::numeric AS total_amount  -- Cast to numeric explicitly
    FROM recurring_expenses re
    JOIN budgets b ON re.budget_id = b.id
    WHERE re.user_id = $1
),
total_goals AS (
    SELECT 
        'goal' AS type,
        jsonb_agg(
            jsonb_build_object(
                'goal_name', g.name,
                'amount', g.target_amount,
                'target_date', g.end_date,
                'budget_name', b.name
            )
        ) AS details,
        SUM(g.monthly_contribution)::numeric AS total_amount  -- Cast to numeric explicitly
    FROM goals g
    JOIN budgets b ON g.budget_id = b.id
    WHERE g.user_id = $1
),
total_budgets AS (
    SELECT 
        'budget' AS type,
        jsonb_agg(
            jsonb_build_object(
                'budget_name', b.name,
                'category', b.category,
                'total_amount', b.total_amount
            )
        ) AS details,
        0::numeric AS total_amount -- Explicit cast to numeric
    FROM budgets b
    WHERE b.user_id = $1
),
total_debts AS (
    SELECT 
        'debt' AS type,
        jsonb_agg(
            jsonb_build_object(
                'debt_name', d.name,
                'due_date', d.due_date,
                'interest_rate', d.interest_rate,
                'remaining_balance', d.remaining_balance
            )
        ) AS details,
        SUM(d.remaining_balance)::numeric AS total_amount  -- Cast to numeric explicitly
    FROM debts d
    WHERE d.user_id = $1
    AND d.remaining_balance > 0 -- Only include debts with remaining balance/amount
)

-- Combine all results using UNION ALL
SELECT * FROM total_incomes
UNION ALL
SELECT * FROM total_expenses
UNION ALL
SELECT * FROM total_recurring_expenses  -- Add recurring expenses
UNION ALL
SELECT * FROM total_goals  
UNION ALL
SELECT * FROM total_budgets
UNION ALL
SELECT * FROM total_debts;

-- name: CheckIfUserHasEnoughPredictionData :one
SELECT 
    CASE
        WHEN $2 = 'weekly' AND (
            SELECT COUNT(DISTINCT DATE_TRUNC('week', e.date_occurred)) 
            FROM expenses e 
            WHERE e.user_id = $1
              AND e.date_occurred >= $3  -- Start date
              AND e.date_occurred <= NOW()
        ) >= 2
        THEN 'sufficient_data_weekly'

        WHEN $2 = 'monthly' AND (
            SELECT COUNT(DISTINCT DATE_TRUNC('month', e.date_occurred)) 
            FROM expenses e 
            WHERE e.user_id = $1
              AND e.date_occurred >= $3  -- Start date
              AND e.date_occurred <= NOW()
        ) >= 2
        THEN 'sufficient_data_monthly'

        ELSE 'insufficient_data'
    END AS data_status;


-- name: GetPersonalFinanceDataForMonthByUserID :many
WITH income_data AS (
    SELECT 
        i.user_id,
        i.amount,
        i.date_received
    FROM income i
    WHERE i.user_id = $1
    AND i.date_received BETWEEN $2 AND CURRENT_DATE -- Filter by start date and current date
),
expense_data AS (
    SELECT 
        e.user_id,
        e.amount,
        e.date_occurred
    FROM expenses e
    WHERE e.user_id = $1
    AND e.date_occurred BETWEEN $2 AND CURRENT_DATE -- Filter by start date and current date
),
goal_data AS (
    SELECT 
        g.user_id,
        g.monthly_contribution AS amount,
        g.start_date
    FROM goals g
    WHERE g.user_id = $1
    AND g.status = 'ongoing'  -- Aggregate only "ongoing" goals
    AND g.start_date BETWEEN $2 AND CURRENT_DATE -- Filter by start date and current date
)
SELECT
    'income' AS type,
    CAST(DATE_TRUNC('month', id.date_received) AS DATE) AS period_start,
    CAST(SUM(id.amount) AS NUMERIC(15, 2)) AS total_amount
FROM income_data id
GROUP BY DATE_TRUNC('month', id.date_received)

UNION ALL

SELECT
    'expense' AS type,
    CAST(DATE_TRUNC('month', ed.date_occurred) AS DATE) AS period_start,
    CAST(SUM(ed.amount) AS NUMERIC(15, 2)) AS total_amount
FROM expense_data ed
GROUP BY DATE_TRUNC('month', ed.date_occurred)

UNION ALL

SELECT
    'goal' AS type,
    CAST(DATE_TRUNC('month', gd.start_date) AS DATE) AS period_start,
    CAST(SUM(gd.amount) AS NUMERIC(15, 2)) AS total_amount
FROM goal_data gd
GROUP BY DATE_TRUNC('month', gd.start_date);

-- name: GetPersonalFinanceDataForWeeklyByUserID :many
WITH income_data AS (
    SELECT 
        i.user_id,
        i.amount,
        i.date_received
    FROM income i
    WHERE i.user_id = $1
    AND i.date_received BETWEEN $2 AND CURRENT_DATE -- Filter by start date and current date
),
expense_data AS (
    SELECT 
        e.user_id,
        e.amount,
        e.date_occurred
    FROM expenses e
    WHERE e.user_id = $1
    AND e.date_occurred BETWEEN $2 AND CURRENT_DATE -- Filter by start date and current date
),
goal_data AS (
    SELECT 
        g.user_id,
        g.monthly_contribution AS amount,
        g.start_date
    FROM goals g
    WHERE g.user_id = $1
    AND g.status = 'ongoing'  -- Aggregate only "ongoing" goals
    AND g.start_date BETWEEN $2 AND CURRENT_DATE -- Filter by start date and current date
)
SELECT
    'income' AS type,
    CAST(DATE_TRUNC('week', id.date_received) AS DATE) AS period_start,
    CAST(SUM(id.amount) AS NUMERIC(15, 2)) AS total_amount
FROM income_data id
GROUP BY DATE_TRUNC('week', id.date_received)

UNION ALL

SELECT
    'expense' AS type,
    CAST(DATE_TRUNC('week', ed.date_occurred) AS DATE) AS period_start,
    CAST(SUM(ed.amount) AS NUMERIC(15, 2)) AS total_amount
FROM expense_data ed
GROUP BY DATE_TRUNC('week', ed.date_occurred)

UNION ALL

SELECT
    'goal' AS type,
    CAST(DATE_TRUNC('week', gd.start_date) AS DATE)AS period_start,
    CAST(SUM(gd.amount) AS NUMERIC(15, 2)) AS total_amount
FROM goal_data gd
GROUP BY DATE_TRUNC('week', gd.start_date);

-- name: GetExpenseIncomeSummaryReport :many
WITH monthly_income AS (
    SELECT
        EXTRACT(MONTH FROM i.date_received) AS month,
        SUM(i.amount)::NUMERIC AS total_income
    FROM income i
    WHERE i.user_id = $1
      AND EXTRACT(YEAR FROM i.date_received) = EXTRACT(YEAR FROM CURRENT_DATE)
    GROUP BY month
),
monthly_expenses AS (
    SELECT
        EXTRACT(MONTH FROM e.date_occurred) AS month,
        SUM(e.amount)::NUMERIC AS total_expenses
    FROM expenses e
    WHERE e.user_id = $1
      AND EXTRACT(YEAR FROM e.date_occurred) = EXTRACT(YEAR FROM CURRENT_DATE)
    GROUP BY month
),
monthly_budgets AS (
    SELECT
        EXTRACT(MONTH FROM b.created_at) AS month,
        SUM(b.total_amount)::NUMERIC AS total_budget
    FROM budgets b
    WHERE b.user_id = $1
      AND EXTRACT(YEAR FROM b.created_at) = EXTRACT(YEAR FROM CURRENT_DATE)
    GROUP BY month
)
SELECT 
    COALESCE(i.month, e.month, b.month) AS month_value,  -- Alias to avoid ambiguity
    COALESCE(i.total_income, 0)::NUMERIC AS total_income,
    COALESCE(e.total_expenses, 0)::NUMERIC AS total_expenses,
    COALESCE(b.total_budget, 0)::NUMERIC AS total_budget
FROM monthly_income i
FULL OUTER JOIN monthly_expenses e ON i.month = e.month
FULL OUTER JOIN monthly_budgets b ON COALESCE(i.month, e.month) = b.month
ORDER BY month_value;
