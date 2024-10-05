// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.26.0
// source: financial_manager_queries.sql

package database

import (
	"context"
	"database/sql"
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

const getBudgetsForUser = `-- name: GetBudgetsForUser :many
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
LIMIT $3 OFFSET $4
`

type GetBudgetsForUserParams struct {
	UserID  int64
	Column2 interface{}
	Limit   int32
	Offset  int32
}

type GetBudgetsForUserRow struct {
	TotalBudgets   int64
	ID             int64
	UserID         int64
	Name           string
	IsStrict       bool
	Category       string
	TotalAmount    string
	CurrencyCode   string
	ConversionRate string
	Description    sql.NullString
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

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
