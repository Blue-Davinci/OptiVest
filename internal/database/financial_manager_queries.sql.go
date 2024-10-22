// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.26.0
// source: financial_manager_queries.sql

package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

const createNewBudget = `-- name: CreateNewBudget :one
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
RETURNING id, created_at, updated_at
`

type CreateNewBudgetParams struct {
	UserID         int64
	Name           string
	IsStrict       bool
	Category       string
	TotalAmount    string
	CurrencyCode   string
	ConversionRate string
	Description    sql.NullString
}

type CreateNewBudgetRow struct {
	ID        int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

//-----------------------------------------------------------------------------------------------------
//----------------------- Budgets
//-----------------------------------------------------------------------------------------------------
func (q *Queries) CreateNewBudget(ctx context.Context, arg CreateNewBudgetParams) (CreateNewBudgetRow, error) {
	row := q.db.QueryRowContext(ctx, createNewBudget,
		arg.UserID,
		arg.Name,
		arg.IsStrict,
		arg.Category,
		arg.TotalAmount,
		arg.CurrencyCode,
		arg.ConversionRate,
		arg.Description,
	)
	var i CreateNewBudgetRow
	err := row.Scan(&i.ID, &i.CreatedAt, &i.UpdatedAt)
	return i, err
}

const createNewGoal = `-- name: CreateNewGoal :one
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
RETURNING id, created_at, updated_at
`

type CreateNewGoalParams struct {
	UserID              int64
	BudgetID            sql.NullInt64
	Name                string
	CurrentAmount       sql.NullString
	TargetAmount        string
	MonthlyContribution string
	StartDate           time.Time
	EndDate             time.Time
	Status              GoalStatus
}

type CreateNewGoalRow struct {
	ID        int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

//-----------------------------------------------------------------------------------------------------
//----------------------- Goals
//-----------------------------------------------------------------------------------------------------
func (q *Queries) CreateNewGoal(ctx context.Context, arg CreateNewGoalParams) (CreateNewGoalRow, error) {
	row := q.db.QueryRowContext(ctx, createNewGoal,
		arg.UserID,
		arg.BudgetID,
		arg.Name,
		arg.CurrentAmount,
		arg.TargetAmount,
		arg.MonthlyContribution,
		arg.StartDate,
		arg.EndDate,
		arg.Status,
	)
	var i CreateNewGoalRow
	err := row.Scan(&i.ID, &i.CreatedAt, &i.UpdatedAt)
	return i, err
}

const createNewGoalPlan = `-- name: CreateNewGoalPlan :one


INSERT INTO goal_plans (
    user_id, 
    name, 
    description, 
    target_amount, 
    monthly_contribution, 
    duration_in_months, 
    is_strict
) VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, created_at, updated_at
`

type CreateNewGoalPlanParams struct {
	UserID              int64
	Name                string
	Description         sql.NullString
	TargetAmount        sql.NullString
	MonthlyContribution sql.NullString
	DurationInMonths    sql.NullInt32
	IsStrict            bool
}

type CreateNewGoalPlanRow struct {
	ID        int64
	CreatedAt sql.NullTime
	UpdatedAt sql.NullTime
}

// Add filtering for a specific user (use user_id placeholder)
func (q *Queries) CreateNewGoalPlan(ctx context.Context, arg CreateNewGoalPlanParams) (CreateNewGoalPlanRow, error) {
	row := q.db.QueryRowContext(ctx, createNewGoalPlan,
		arg.UserID,
		arg.Name,
		arg.Description,
		arg.TargetAmount,
		arg.MonthlyContribution,
		arg.DurationInMonths,
		arg.IsStrict,
	)
	var i CreateNewGoalPlanRow
	err := row.Scan(&i.ID, &i.CreatedAt, &i.UpdatedAt)
	return i, err
}

const deleteBudgetById = `-- name: DeleteBudgetById :one
DELETE FROM budgets
WHERE id = $1 AND user_id = $2
RETURNING id
`

type DeleteBudgetByIdParams struct {
	ID     int64
	UserID int64
}

func (q *Queries) DeleteBudgetById(ctx context.Context, arg DeleteBudgetByIdParams) (int64, error) {
	row := q.db.QueryRowContext(ctx, deleteBudgetById, arg.ID, arg.UserID)
	var id int64
	err := row.Scan(&id)
	return id, err
}

const getAllGoalSummaryByBudgetID = `-- name: GetAllGoalSummaryByBudgetID :many
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
AND b.user_id = $2
`

type GetAllGoalSummaryByBudgetIDParams struct {
	ID     int64
	UserID int64
}

type GetAllGoalSummaryByBudgetIDRow struct {
	TotalAmount                string
	TotalMonthlyContributions  interface{}
	TotalExpenses              interface{}
	ProjectedRecurringExpenses interface{}
	BudgetSurplus              string
}

// Final query combining all the calculations
func (q *Queries) GetAllGoalSummaryByBudgetID(ctx context.Context, arg GetAllGoalSummaryByBudgetIDParams) ([]GetAllGoalSummaryByBudgetIDRow, error) {
	rows, err := q.db.QueryContext(ctx, getAllGoalSummaryByBudgetID, arg.ID, arg.UserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []GetAllGoalSummaryByBudgetIDRow
	for rows.Next() {
		var i GetAllGoalSummaryByBudgetIDRow
		if err := rows.Scan(
			&i.TotalAmount,
			&i.TotalMonthlyContributions,
			&i.TotalExpenses,
			&i.ProjectedRecurringExpenses,
			&i.BudgetSurplus,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const getAllGoalsWithProgressionByUserID = `-- name: GetAllGoalsWithProgressionByUserID :many
WITH goal_contributions AS (
    -- Aggregate total contributed amount for each goal
    SELECT 
        gt.goal_id,
        COALESCE(SUM(gt.contributed_amount), 0)::NUMERIC AS total_contributed_amount
    FROM goal_tracking gt
    GROUP BY gt.goal_id
)
SELECT 
    g.id, 
    g.user_id,
    g.budget_id,
    g.name, 
    g.current_amount, 
    g.target_amount, 
    g.monthly_contribution, 
    g.start_date, 
    g.end_date, 
    g.status, 
    g.created_at, 
    g.updated_at,
    -- Join with aggregated contribution data
    COALESCE(gc.total_contributed_amount , 0)::NUMERIC AS total_contributed_amount,
    -- Calculate and cast the percentage progress
    COALESCE((gc.total_contributed_amount / g.target_amount) * 100, 0)::NUMERIC AS progress_percentage
FROM goals g
LEFT JOIN goal_contributions gc ON g.id = gc.goal_id
WHERE g.user_id = $1
`

type GetAllGoalsWithProgressionByUserIDRow struct {
	ID                     int64
	UserID                 int64
	BudgetID               sql.NullInt64
	Name                   string
	CurrentAmount          sql.NullString
	TargetAmount           string
	MonthlyContribution    string
	StartDate              time.Time
	EndDate                time.Time
	Status                 GoalStatus
	CreatedAt              time.Time
	UpdatedAt              time.Time
	TotalContributedAmount string
	ProgressPercentage     string
}

func (q *Queries) GetAllGoalsWithProgressionByUserID(ctx context.Context, userID int64) ([]GetAllGoalsWithProgressionByUserIDRow, error) {
	rows, err := q.db.QueryContext(ctx, getAllGoalsWithProgressionByUserID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []GetAllGoalsWithProgressionByUserIDRow
	for rows.Next() {
		var i GetAllGoalsWithProgressionByUserIDRow
		if err := rows.Scan(
			&i.ID,
			&i.UserID,
			&i.BudgetID,
			&i.Name,
			&i.CurrentAmount,
			&i.TargetAmount,
			&i.MonthlyContribution,
			&i.StartDate,
			&i.EndDate,
			&i.Status,
			&i.CreatedAt,
			&i.UpdatedAt,
			&i.TotalContributedAmount,
			&i.ProgressPercentage,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const getAndSaveAllGoalsForTracking = `-- name: GetAndSaveAllGoalsForTracking :many
INSERT INTO goal_tracking (user_id, goal_id, contributed_amount, tracking_type)
SELECT g.user_id, g.id, g.monthly_contribution, 'monthly'
FROM goals g
LEFT JOIN goal_tracking gt ON g.id = gt.goal_id 
   AND gt.truncated_tracking_date = date_trunc('month', CURRENT_DATE)::date
WHERE gt.goal_id IS NULL
  AND g.status = 'ongoing' 
  AND g.start_date < CURRENT_DATE
ORDER BY truncated_tracking_date ASC
RETURNING id, user_id, goal_id, contributed_amount
`

type GetAndSaveAllGoalsForTrackingRow struct {
	ID                int64
	UserID            int64
	GoalID            sql.NullInt64
	ContributedAmount string
}

// Insert tracked goals that haven't been tracked for more than 1 month
func (q *Queries) GetAndSaveAllGoalsForTracking(ctx context.Context) ([]GetAndSaveAllGoalsForTrackingRow, error) {
	rows, err := q.db.QueryContext(ctx, getAndSaveAllGoalsForTracking)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []GetAndSaveAllGoalsForTrackingRow
	for rows.Next() {
		var i GetAndSaveAllGoalsForTrackingRow
		if err := rows.Scan(
			&i.ID,
			&i.UserID,
			&i.GoalID,
			&i.ContributedAmount,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const getBudgetByID = `-- name: GetBudgetByID :one
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
WHERE id = $1
`

func (q *Queries) GetBudgetByID(ctx context.Context, id int64) (Budget, error) {
	row := q.db.QueryRowContext(ctx, getBudgetByID, id)
	var i Budget
	err := row.Scan(
		&i.ID,
		&i.UserID,
		&i.Name,
		&i.IsStrict,
		&i.Category,
		&i.TotalAmount,
		&i.CurrencyCode,
		&i.ConversionRate,
		&i.Description,
		&i.CreatedAt,
		&i.UpdatedAt,
	)
	return i, err
}

const getBudgetGoalExpenseSummary = `-- name: GetBudgetGoalExpenseSummary :many
WITH goal_summaries AS (
    -- Sum the contributed amounts for each goal in goal_tracking
    SELECT 
        g.id AS goal_id,
        SUM(gt.contributed_amount) AS total_goal_contribution
    FROM goals g
    LEFT JOIN goal_tracking gt ON g.id = gt.goal_id
    WHERE g.user_id = $1  -- Filter by user_id
    GROUP BY g.id
),
expense_summaries AS (
    -- Sum the amounts for each budget in the expenses table
    SELECT 
        budget_id,
        SUM(e.amount) AS total_expenses
    FROM expenses e
    WHERE e.user_id = $1  -- Filter by user_id
    GROUP BY budget_id
),
recurring_expense_summaries AS (
    -- Group the recurring expenses by budget and sum their projected amounts
    SELECT 
        budget_id,
        SUM(re.projected_amount) AS total_projected_recurring_expenses,
        jsonb_agg(
            jsonb_build_object(
                'recurring_expense_name', re.name,
                'recurrence_interval', re.recurrence_interval,
                'projected_amount', re.projected_amount
            )
        ) AS recurring_expenses
    FROM recurring_expenses re
    WHERE re.user_id = $1  -- Filter by user_id
    GROUP BY budget_id
)
SELECT 
    b.id AS budget_id,
    b.name AS budget_name,
    b.category AS budget_category,
    b.total_amount AS budget_total_amount,
    b.is_strict AS budget_is_strict,  -- Add is_strict field

    -- Include the goal details for each budget
    jsonb_agg(
        jsonb_build_object(
            'goal_id', g.id,
            'goal_name', g.name,
            'current_amount', g.current_amount,
            'target_amount', g.target_amount,
            'monthly_contribution', g.monthly_contribution,
            'total_contributed', COALESCE(gs.total_goal_contribution, 0)
        )
    ) AS goals,

    -- Include the recurring expense details for each budget
    COALESCE(res.recurring_expenses, '[]'::jsonb) AS recurring_expenses,

    -- Total projected recurring expenses for each budget
    COALESCE(res.total_projected_recurring_expenses, 0)::NUMERIC AS total_projected_recurring_expenses,

    -- Total non-recurring expenses for each budget
    COALESCE(es.total_expenses, 0)::NUMERIC AS total_expenses
    
FROM budgets b
LEFT JOIN goals g ON b.id = g.budget_id
LEFT JOIN goal_summaries gs ON g.id = gs.goal_id
LEFT JOIN expense_summaries es ON b.id = es.budget_id
LEFT JOIN recurring_expense_summaries res ON b.id = res.budget_id

WHERE b.user_id = $1

GROUP BY b.id, es.total_expenses, res.total_projected_recurring_expenses, res.recurring_expenses
`

type GetBudgetGoalExpenseSummaryRow struct {
	BudgetID                        int64
	BudgetName                      string
	BudgetCategory                  string
	BudgetTotalAmount               string
	BudgetIsStrict                  bool
	Goals                           json.RawMessage
	RecurringExpenses               json.RawMessage
	TotalProjectedRecurringExpenses string
	TotalExpenses                   string
}

// Filter budgets by user_id
// Group by budget to allow aggregation for goals, recurring expenses, and total expenses
func (q *Queries) GetBudgetGoalExpenseSummary(ctx context.Context, userID int64) ([]GetBudgetGoalExpenseSummaryRow, error) {
	rows, err := q.db.QueryContext(ctx, getBudgetGoalExpenseSummary, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []GetBudgetGoalExpenseSummaryRow
	for rows.Next() {
		var i GetBudgetGoalExpenseSummaryRow
		if err := rows.Scan(
			&i.BudgetID,
			&i.BudgetName,
			&i.BudgetCategory,
			&i.BudgetTotalAmount,
			&i.BudgetIsStrict,
			&i.Goals,
			&i.RecurringExpenses,
			&i.TotalProjectedRecurringExpenses,
			&i.TotalExpenses,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const getBudgetsForUser = `-- name: GetBudgetsForUser :many
SELECT COUNT(*) OVER() AS total_budgets,
    b.id, 
    b.user_id, 
    b.name, 
    b.is_strict, 
    b.category, 
    b.total_amount, 
    b.currency_code, 
    b.conversion_rate, 
    b.description, 
    b.created_at, 
    b.updated_at,

    -- Aggregate goals into JSON array
    COALESCE(goals.goals, '[]'::json) AS goals,

    -- Aggregate recurring expenses into JSON array without duplication
    COALESCE(recurring_expenses.expenses, '[]'::json) AS recurring_expenses,

    -- Total sums
    COALESCE(goals.total_monthly_contributions, 0) AS total_monthly_contributions,
    COALESCE(recurring_expenses.total_recurring_expenses, 0) AS total_recurring_expenses

FROM budgets b

LEFT JOIN LATERAL (
    SELECT 
        json_agg(
            json_build_object(
                'id', g.id,
                'name', g.name,
                'monthly_contribution', g.monthly_contribution,
                'target_amount', g.target_amount
            )
        ) AS goals,
        SUM(g.monthly_contribution)::NUMERIC AS total_monthly_contributions
    FROM goals g
    WHERE g.budget_id = b.id
) AS goals ON true

LEFT JOIN LATERAL (
    SELECT 
        json_agg(
            json_build_object(
                'id', e.id,
                'name', e.name,
                'projected_amount', e.projected_amount,
                'next_occurrence', e.next_occurrence
            )
        ) AS expenses,
        SUM(e.projected_amount)::NUMERIC AS total_recurring_expenses
    FROM (
        SELECT DISTINCT ON (re.name, re.budget_id)
            re.id,
            re.name,
            re.projected_amount,
            re.next_occurrence
        FROM recurring_expenses re
        WHERE re.budget_id = b.id
        ORDER BY re.name, re.budget_id, re.next_occurrence DESC
    ) e
) AS recurring_expenses ON true

WHERE b.user_id = $1
AND ($2 = '' OR to_tsvector('simple', b.name) @@ plainto_tsquery('simple', $2))
ORDER BY b.created_at DESC
LIMIT $3 OFFSET $4
`

type GetBudgetsForUserParams struct {
	UserID  int64
	Column2 interface{}
	Limit   int32
	Offset  int32
}

type GetBudgetsForUserRow struct {
	TotalBudgets              int64
	ID                        int64
	UserID                    int64
	Name                      string
	IsStrict                  bool
	Category                  string
	TotalAmount               string
	CurrencyCode              string
	ConversionRate            string
	Description               sql.NullString
	CreatedAt                 time.Time
	UpdatedAt                 time.Time
	Goals                     json.RawMessage
	RecurringExpenses         json.RawMessage
	TotalMonthlyContributions string
	TotalRecurringExpenses    string
}

// LATERAL subquery for goals
// LATERAL subquery for recurring expenses
func (q *Queries) GetBudgetsForUser(ctx context.Context, arg GetBudgetsForUserParams) ([]GetBudgetsForUserRow, error) {
	rows, err := q.db.QueryContext(ctx, getBudgetsForUser,
		arg.UserID,
		arg.Column2,
		arg.Limit,
		arg.Offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []GetBudgetsForUserRow
	for rows.Next() {
		var i GetBudgetsForUserRow
		if err := rows.Scan(
			&i.TotalBudgets,
			&i.ID,
			&i.UserID,
			&i.Name,
			&i.IsStrict,
			&i.Category,
			&i.TotalAmount,
			&i.CurrencyCode,
			&i.ConversionRate,
			&i.Description,
			&i.CreatedAt,
			&i.UpdatedAt,
			&i.Goals,
			&i.RecurringExpenses,
			&i.TotalMonthlyContributions,
			&i.TotalRecurringExpenses,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const getGoalByID = `-- name: GetGoalByID :one
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
WHERE id = $1 AND user_id = $2
`

type GetGoalByIDParams struct {
	ID     int64
	UserID int64
}

type GetGoalByIDRow struct {
	ID                  int64
	UserID              int64
	BudgetID            sql.NullInt64
	Name                string
	CurrentAmount       sql.NullString
	TargetAmount        string
	MonthlyContribution string
	StartDate           time.Time
	EndDate             time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
	Status              GoalStatus
}

func (q *Queries) GetGoalByID(ctx context.Context, arg GetGoalByIDParams) (GetGoalByIDRow, error) {
	row := q.db.QueryRowContext(ctx, getGoalByID, arg.ID, arg.UserID)
	var i GetGoalByIDRow
	err := row.Scan(
		&i.ID,
		&i.UserID,
		&i.BudgetID,
		&i.Name,
		&i.CurrentAmount,
		&i.TargetAmount,
		&i.MonthlyContribution,
		&i.StartDate,
		&i.EndDate,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.Status,
	)
	return i, err
}

const getGoalPlanByID = `-- name: GetGoalPlanByID :one
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
WHERE id = $1 AND user_id = $2
`

type GetGoalPlanByIDParams struct {
	ID     int64
	UserID int64
}

func (q *Queries) GetGoalPlanByID(ctx context.Context, arg GetGoalPlanByIDParams) (GoalPlan, error) {
	row := q.db.QueryRowContext(ctx, getGoalPlanByID, arg.ID, arg.UserID)
	var i GoalPlan
	err := row.Scan(
		&i.ID,
		&i.UserID,
		&i.Name,
		&i.Description,
		&i.TargetAmount,
		&i.MonthlyContribution,
		&i.DurationInMonths,
		&i.IsStrict,
		&i.CreatedAt,
		&i.UpdatedAt,
	)
	return i, err
}

const getGoalPlansForUser = `-- name: GetGoalPlansForUser :many
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
LIMIT $2 OFFSET $3
`

type GetGoalPlansForUserParams struct {
	UserID int64
	Limit  int32
	Offset int32
}

type GetGoalPlansForUserRow struct {
	TotalGoalPlans      int64
	ID                  int64
	UserID              int64
	Name                string
	Description         sql.NullString
	TargetAmount        sql.NullString
	MonthlyContribution sql.NullString
	DurationInMonths    sql.NullInt32
	IsStrict            bool
	CreatedAt           sql.NullTime
	UpdatedAt           sql.NullTime
}

func (q *Queries) GetGoalPlansForUser(ctx context.Context, arg GetGoalPlansForUserParams) ([]GetGoalPlansForUserRow, error) {
	rows, err := q.db.QueryContext(ctx, getGoalPlansForUser, arg.UserID, arg.Limit, arg.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []GetGoalPlansForUserRow
	for rows.Next() {
		var i GetGoalPlansForUserRow
		if err := rows.Scan(
			&i.TotalGoalPlans,
			&i.ID,
			&i.UserID,
			&i.Name,
			&i.Description,
			&i.TargetAmount,
			&i.MonthlyContribution,
			&i.DurationInMonths,
			&i.IsStrict,
			&i.CreatedAt,
			&i.UpdatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const getGoalsForUserInvestmentHelper = `-- name: GetGoalsForUserInvestmentHelper :many
SELECT
    name,
    current_amount,
    target_amount,
    monthly_contribution,
    start_date,
    end_date
FROM goals
WHERE user_id = $1
AND status = 'ongoing'
`

type GetGoalsForUserInvestmentHelperRow struct {
	Name                string
	CurrentAmount       sql.NullString
	TargetAmount        string
	MonthlyContribution string
	StartDate           time.Time
	EndDate             time.Time
}

func (q *Queries) GetGoalsForUserInvestmentHelper(ctx context.Context, userID int64) ([]GetGoalsForUserInvestmentHelperRow, error) {
	rows, err := q.db.QueryContext(ctx, getGoalsForUserInvestmentHelper, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []GetGoalsForUserInvestmentHelperRow
	for rows.Next() {
		var i GetGoalsForUserInvestmentHelperRow
		if err := rows.Scan(
			&i.Name,
			&i.CurrentAmount,
			&i.TargetAmount,
			&i.MonthlyContribution,
			&i.StartDate,
			&i.EndDate,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const updateBudgetById = `-- name: UpdateBudgetById :one
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
RETURNING updated_at
`

type UpdateBudgetByIdParams struct {
	ID             int64
	Name           string
	IsStrict       bool
	Category       string
	TotalAmount    string
	CurrencyCode   string
	ConversionRate string
	Description    sql.NullString
	UserID         int64
}

func (q *Queries) UpdateBudgetById(ctx context.Context, arg UpdateBudgetByIdParams) (time.Time, error) {
	row := q.db.QueryRowContext(ctx, updateBudgetById,
		arg.ID,
		arg.Name,
		arg.IsStrict,
		arg.Category,
		arg.TotalAmount,
		arg.CurrencyCode,
		arg.ConversionRate,
		arg.Description,
		arg.UserID,
	)
	var updated_at time.Time
	err := row.Scan(&updated_at)
	return updated_at, err
}

const updateGoalByID = `-- name: UpdateGoalByID :one
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
RETURNING updated_at
`

type UpdateGoalByIDParams struct {
	Name                string
	CurrentAmount       sql.NullString
	TargetAmount        string
	MonthlyContribution string
	StartDate           time.Time
	EndDate             time.Time
	Status              GoalStatus
	ID                  int64
	UserID              int64
}

func (q *Queries) UpdateGoalByID(ctx context.Context, arg UpdateGoalByIDParams) (time.Time, error) {
	row := q.db.QueryRowContext(ctx, updateGoalByID,
		arg.Name,
		arg.CurrentAmount,
		arg.TargetAmount,
		arg.MonthlyContribution,
		arg.StartDate,
		arg.EndDate,
		arg.Status,
		arg.ID,
		arg.UserID,
	)
	var updated_at time.Time
	err := row.Scan(&updated_at)
	return updated_at, err
}

const updateGoalPlanByID = `-- name: UpdateGoalPlanByID :one
UPDATE goal_plans SET
    name = $1,
    description = $2,
    target_amount = $3,
    monthly_contribution = $4,
    duration_in_months = $5,
    is_strict = $6
WHERE
    id = $7 AND user_id = $8
RETURNING updated_at
`

type UpdateGoalPlanByIDParams struct {
	Name                string
	Description         sql.NullString
	TargetAmount        sql.NullString
	MonthlyContribution sql.NullString
	DurationInMonths    sql.NullInt32
	IsStrict            bool
	ID                  int64
	UserID              int64
}

func (q *Queries) UpdateGoalPlanByID(ctx context.Context, arg UpdateGoalPlanByIDParams) (sql.NullTime, error) {
	row := q.db.QueryRowContext(ctx, updateGoalPlanByID,
		arg.Name,
		arg.Description,
		arg.TargetAmount,
		arg.MonthlyContribution,
		arg.DurationInMonths,
		arg.IsStrict,
		arg.ID,
		arg.UserID,
	)
	var updated_at sql.NullTime
	err := row.Scan(&updated_at)
	return updated_at, err
}

const updateGoalProgressOnExpiredGoals = `-- name: UpdateGoalProgressOnExpiredGoals :exec
UPDATE goals
SET status = 'completed',
    updated_at = NOW()
WHERE current_amount >= target_amount
  AND status = 'ongoing'
`

func (q *Queries) UpdateGoalProgressOnExpiredGoals(ctx context.Context) error {
	_, err := q.db.ExecContext(ctx, updateGoalProgressOnExpiredGoals)
	return err
}
