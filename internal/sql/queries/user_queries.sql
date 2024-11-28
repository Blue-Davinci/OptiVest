-- name: CreateNewUser :one
INSERT INTO users (
    first_name,
    last_name,
    email,
    profile_avatar_url,
    password,
    phone_number,
    profile_completed,
    dob,
    address,
    country_code
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
)
RETURNING id, created_at, updated_at, role_level, last_login, version, mfa_enabled, mfa_secret, mfa_status, mfa_last_checked;

-- name: UpdateUser :one
UPDATE users
SET
    first_name = $1,
    last_name = $2,
    email = $3,
    profile_avatar_url = $4,
    password = $5,
    role_level = $6,
    phone_number = $7,
    activated = $8,
    version = version + 1,
    updated_at = NOW(),
    last_login = $9,
    profile_completed = $10,
    dob = $11,
    address = $12,
    country_code = $13,
    currency_code = $14,
    mfa_enabled = $15,
    mfa_secret = $16,
    mfa_status = $17,
    mfa_last_checked = $18,
    risk_tolerance = $19,
    time_horizon = $20
WHERE id = $21 AND version = $22
RETURNING updated_at, version;

-- name: GetUserByEmail :one
SELECT 
    id,
    first_name,
    last_name,
    email,
    profile_avatar_url,
    password,
    role_level,
    phone_number,
    activated,
    version,
    created_at,
    updated_at,
    last_login,
    profile_completed,
    dob,
    address,
    country_code,
    currency_code,
    mfa_enabled,
    mfa_secret,
    mfa_status,
    mfa_last_checked,
    risk_tolerance,
    time_horizon
FROM users
WHERE email = $1;

-- name: GetAccountStatisticsByUserId :one
WITH 
-- Calculate profile completion
profile_completion AS (
    SELECT
        id AS user_id,
        ROUND(
            (CASE WHEN profile_avatar_url IS NOT NULL THEN 1 ELSE 0 END +
             CASE WHEN phone_number IS NOT NULL THEN 1 ELSE 0 END +
             CASE WHEN dob IS NOT NULL THEN 1 ELSE 0 END +
             CASE WHEN address IS NOT NULL THEN 1 ELSE 0 END +
             CASE WHEN mfa_enabled THEN 1 ELSE 0 END +
             CASE WHEN risk_tolerance IS NOT NULL THEN 1 ELSE 0 END +
             CASE WHEN time_horizon IS NOT NULL THEN 1 ELSE 0 END
            )::DECIMAL / 7 * 100, 2) AS profile_completion
    FROM users
    WHERE id = $1
),
-- Aggregate budgets for the user
budget_stats AS (
    SELECT
        user_id,
        COUNT(*) AS total_budgets,
        COALESCE(SUM(total_amount), 0) AS total_budget_amount,
        COUNT(*) FILTER (WHERE is_strict) AS strict_budgets
    FROM budgets
    WHERE user_id = $1
    GROUP BY user_id
),
-- Peer statistics for budgets
peer_budget_stats AS (
    SELECT
        AVG(total_amount) AS avg_budget_amount,
        STDDEV(total_amount) AS stddev_budget_amount
    FROM budgets
),
-- Aggregate goals for the user
goal_stats AS (
    SELECT
        user_id,
        COUNT(*) AS total_goals,
        COUNT(*) FILTER (WHERE status = 'ongoing') AS ongoing_goals,
        COUNT(*) FILTER (WHERE status = 'completed') AS completed_goals,
        ROUND(
            COALESCE(SUM(current_amount)::DECIMAL / NULLIF(SUM(target_amount), 0), 0) * 100, 2
        ) AS average_goal_completion
    FROM goals
    WHERE user_id = $1
    GROUP BY user_id
),
-- Peer statistics for goals
peer_goal_stats AS (
    SELECT
        AVG(target_amount) AS avg_goal_amount,
        STDDEV(target_amount) AS stddev_goal_amount,
        AVG(current_amount) AS avg_goal_progress,
        STDDEV(current_amount) AS stddev_goal_progress
    FROM goals
),
-- Aggregate expenses for the user
expense_stats AS (
    SELECT
        user_id,
        COUNT(*) AS total_expenses,
        COALESCE(SUM(amount), 0) AS total_expense_amount,
        COUNT(*) FILTER (WHERE is_recurring) AS recurring_expenses
    FROM expenses
    WHERE user_id = $1
    GROUP BY user_id
),
-- Aggregate incomes for the user
income_stats AS (
    SELECT
        user_id,
        COUNT(*) AS total_income_sources,
        COALESCE(SUM(amount), 0) AS total_income_amount
    FROM income
    WHERE user_id = $1
    GROUP BY user_id
),
-- Aggregate groups for the user
group_stats AS (
    SELECT
        gm.user_id,
        COUNT(DISTINCT gm.group_id) AS groups_joined,
        COUNT(DISTINCT g.id) AS groups_created
    FROM group_memberships gm
    LEFT JOIN groups g ON g.id = gm.group_id AND g.creator_user_id = gm.user_id
    WHERE gm.user_id = $1
    GROUP BY gm.user_id
)
-- Combine user statistics and peer statistics
SELECT
    pc.profile_completion,
    COALESCE(bs.total_budgets, 0)::INTEGER AS total_budgets,
    COALESCE(bs.total_budget_amount, 0)::INTEGER AS total_budget_amount,
    COALESCE(gs.total_goals, 0)::INTEGER AS total_goals,
    COALESCE(gs.ongoing_goals, 0)::INTEGER AS ongoing_goals,
    COALESCE(gs.completed_goals, 0)::INTEGER AS completed_goals,
    COALESCE(gs.average_goal_completion, 0)::INTEGER AS average_goal_completion,
    COALESCE(es.total_expenses, 0)::INTEGER AS total_expenses,
    COALESCE(es.total_expense_amount, 0)::INTEGER AS total_expense_amount,
    COALESCE(ins.total_income_sources, 0)::INTEGER AS total_income_sources,
    COALESCE(ins.total_income_amount, 0)::INTEGER AS total_income_amount,
    COALESCE(gr.groups_joined, 0)::INTEGER AS groups_joined,
    COALESCE(gr.groups_created, 0)::INTEGER AS groups_created,
    -- Include peer statistics for budgets
    (SELECT avg_budget_amount FROM peer_budget_stats) AS avg_budget_amount,
    (SELECT stddev_budget_amount FROM peer_budget_stats) AS stddev_budget_amount,
    -- Include peer statistics for goals
    (SELECT avg_goal_amount FROM peer_goal_stats) AS avg_goal_amount,
    (SELECT stddev_goal_amount FROM peer_goal_stats) AS stddev_goal_amount,
    (SELECT avg_goal_progress FROM peer_goal_stats) AS avg_goal_progress,
    (SELECT stddev_goal_progress FROM peer_goal_stats) AS stddev_goal_progress
FROM users u
LEFT JOIN profile_completion pc ON u.id = pc.user_id
LEFT JOIN budget_stats bs ON u.id = bs.user_id
LEFT JOIN goal_stats gs ON u.id = gs.user_id
LEFT JOIN expense_stats es ON u.id = es.user_id
LEFT JOIN income_stats ins ON u.id = ins.user_id
LEFT JOIN group_stats gr ON u.id = gr.user_id
WHERE u.id = $1;
