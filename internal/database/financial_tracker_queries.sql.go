// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.27.0
// source: financial_tracker_queries.sql

package database

import (
	"context"
	"database/sql"
	"time"
)

const createNewDebt = `-- name: CreateNewDebt :one
INSERT INTO debts (
    user_id, name, amount, remaining_balance, interest_rate, description, 
    due_date, minimum_payment, next_payment_date, estimated_payoff_date, 
    accrued_interest, interest_last_calculated, total_interest_paid
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
RETURNING id, created_at, updated_at
`

type CreateNewDebtParams struct {
	UserID                 int64
	Name                   string
	Amount                 string
	RemainingBalance       string
	InterestRate           sql.NullString
	Description            sql.NullString
	DueDate                time.Time
	MinimumPayment         string
	NextPaymentDate        time.Time
	EstimatedPayoffDate    sql.NullTime
	AccruedInterest        sql.NullString
	InterestLastCalculated sql.NullTime
	TotalInterestPaid      sql.NullString
}

type CreateNewDebtRow struct {
	ID        int64
	CreatedAt sql.NullTime
	UpdatedAt sql.NullTime
}

func (q *Queries) CreateNewDebt(ctx context.Context, arg CreateNewDebtParams) (CreateNewDebtRow, error) {
	row := q.db.QueryRowContext(ctx, createNewDebt,
		arg.UserID,
		arg.Name,
		arg.Amount,
		arg.RemainingBalance,
		arg.InterestRate,
		arg.Description,
		arg.DueDate,
		arg.MinimumPayment,
		arg.NextPaymentDate,
		arg.EstimatedPayoffDate,
		arg.AccruedInterest,
		arg.InterestLastCalculated,
		arg.TotalInterestPaid,
	)
	var i CreateNewDebtRow
	err := row.Scan(&i.ID, &i.CreatedAt, &i.UpdatedAt)
	return i, err
}

const createNewDebtPayment = `-- name: CreateNewDebtPayment :one
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
RETURNING id, created_at
`

type CreateNewDebtPaymentParams struct {
	DebtID           int64
	UserID           int64
	PaymentAmount    string
	PaymentDate      time.Time
	InterestPayment  string
	PrincipalPayment string
}

type CreateNewDebtPaymentRow struct {
	ID        int64
	CreatedAt sql.NullTime
}

func (q *Queries) CreateNewDebtPayment(ctx context.Context, arg CreateNewDebtPaymentParams) (CreateNewDebtPaymentRow, error) {
	row := q.db.QueryRowContext(ctx, createNewDebtPayment,
		arg.DebtID,
		arg.UserID,
		arg.PaymentAmount,
		arg.PaymentDate,
		arg.InterestPayment,
		arg.PrincipalPayment,
	)
	var i CreateNewDebtPaymentRow
	err := row.Scan(&i.ID, &i.CreatedAt)
	return i, err
}

const createNewExpense = `-- name: CreateNewExpense :one
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
RETURNING id, created_at, updated_at
`

type CreateNewExpenseParams struct {
	UserID       int64
	BudgetID     int64
	Name         string
	Category     string
	Amount       string
	IsRecurring  bool
	Description  sql.NullString
	DateOccurred time.Time
}

type CreateNewExpenseRow struct {
	ID        int64
	CreatedAt sql.NullTime
	UpdatedAt sql.NullTime
}

func (q *Queries) CreateNewExpense(ctx context.Context, arg CreateNewExpenseParams) (CreateNewExpenseRow, error) {
	row := q.db.QueryRowContext(ctx, createNewExpense,
		arg.UserID,
		arg.BudgetID,
		arg.Name,
		arg.Category,
		arg.Amount,
		arg.IsRecurring,
		arg.Description,
		arg.DateOccurred,
	)
	var i CreateNewExpenseRow
	err := row.Scan(&i.ID, &i.CreatedAt, &i.UpdatedAt)
	return i, err
}

const createNewIncome = `-- name: CreateNewIncome :one
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
) RETURNING id, created_at, updated_at
`

type CreateNewIncomeParams struct {
	UserID               int64
	Source               string
	OriginalCurrencyCode string
	AmountOriginal       string
	Amount               string
	ExchangeRate         string
	Description          sql.NullString
	DateReceived         time.Time
}

type CreateNewIncomeRow struct {
	ID        int64
	CreatedAt sql.NullTime
	UpdatedAt sql.NullTime
}

func (q *Queries) CreateNewIncome(ctx context.Context, arg CreateNewIncomeParams) (CreateNewIncomeRow, error) {
	row := q.db.QueryRowContext(ctx, createNewIncome,
		arg.UserID,
		arg.Source,
		arg.OriginalCurrencyCode,
		arg.AmountOriginal,
		arg.Amount,
		arg.ExchangeRate,
		arg.Description,
		arg.DateReceived,
	)
	var i CreateNewIncomeRow
	err := row.Scan(&i.ID, &i.CreatedAt, &i.UpdatedAt)
	return i, err
}

const createNewRecurringExpense = `-- name: CreateNewRecurringExpense :one
INSERT INTO recurring_expenses (
    user_id, budget_id, amount,name, description, recurrence_interval,projected_amount, next_occurrence
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
)
RETURNING id, created_at, updated_at
`

type CreateNewRecurringExpenseParams struct {
	UserID             int64
	BudgetID           int64
	Amount             string
	Name               string
	Description        sql.NullString
	RecurrenceInterval RecurrenceIntervalEnum
	ProjectedAmount    string
	NextOccurrence     time.Time
}

type CreateNewRecurringExpenseRow struct {
	ID        int64
	CreatedAt sql.NullTime
	UpdatedAt sql.NullTime
}

func (q *Queries) CreateNewRecurringExpense(ctx context.Context, arg CreateNewRecurringExpenseParams) (CreateNewRecurringExpenseRow, error) {
	row := q.db.QueryRowContext(ctx, createNewRecurringExpense,
		arg.UserID,
		arg.BudgetID,
		arg.Amount,
		arg.Name,
		arg.Description,
		arg.RecurrenceInterval,
		arg.ProjectedAmount,
		arg.NextOccurrence,
	)
	var i CreateNewRecurringExpenseRow
	err := row.Scan(&i.ID, &i.CreatedAt, &i.UpdatedAt)
	return i, err
}

const getAllDebtsByUserID = `-- name: GetAllDebtsByUserID :many
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
LIMIT $3 OFFSET $4
`

type GetAllDebtsByUserIDParams struct {
	UserID  int64
	Column2 interface{}
	Limit   int32
	Offset  int32
}

type GetAllDebtsByUserIDRow struct {
	ID                     int64
	UserID                 int64
	Name                   string
	Amount                 string
	RemainingBalance       string
	InterestRate           sql.NullString
	Description            sql.NullString
	DueDate                time.Time
	MinimumPayment         string
	CreatedAt              sql.NullTime
	UpdatedAt              sql.NullTime
	NextPaymentDate        time.Time
	EstimatedPayoffDate    sql.NullTime
	AccruedInterest        sql.NullString
	InterestLastCalculated sql.NullTime
	LastPaymentDate        sql.NullTime
	TotalInterestPaid      sql.NullString
	TotalDebts             int64
	TotalAmounts           string
	TotalRemainingBalances string
}

func (q *Queries) GetAllDebtsByUserID(ctx context.Context, arg GetAllDebtsByUserIDParams) ([]GetAllDebtsByUserIDRow, error) {
	rows, err := q.db.QueryContext(ctx, getAllDebtsByUserID,
		arg.UserID,
		arg.Column2,
		arg.Limit,
		arg.Offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []GetAllDebtsByUserIDRow
	for rows.Next() {
		var i GetAllDebtsByUserIDRow
		if err := rows.Scan(
			&i.ID,
			&i.UserID,
			&i.Name,
			&i.Amount,
			&i.RemainingBalance,
			&i.InterestRate,
			&i.Description,
			&i.DueDate,
			&i.MinimumPayment,
			&i.CreatedAt,
			&i.UpdatedAt,
			&i.NextPaymentDate,
			&i.EstimatedPayoffDate,
			&i.AccruedInterest,
			&i.InterestLastCalculated,
			&i.LastPaymentDate,
			&i.TotalInterestPaid,
			&i.TotalDebts,
			&i.TotalAmounts,
			&i.TotalRemainingBalances,
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

const getAllExpensesByUserID = `-- name: GetAllExpensesByUserID :many
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
    $3 OFFSET $4
`

type GetAllExpensesByUserIDParams struct {
	UserID  int64
	Column2 interface{}
	Limit   int32
	Offset  int32
}

type GetAllExpensesByUserIDRow struct {
	ID           int64
	UserID       int64
	BudgetID     int64
	Name         string
	Category     string
	Amount       string
	IsRecurring  bool
	Description  sql.NullString
	DateOccurred time.Time
	CreatedAt    sql.NullTime
	UpdatedAt    sql.NullTime
	TotalCount   int64
}

func (q *Queries) GetAllExpensesByUserID(ctx context.Context, arg GetAllExpensesByUserIDParams) ([]GetAllExpensesByUserIDRow, error) {
	rows, err := q.db.QueryContext(ctx, getAllExpensesByUserID,
		arg.UserID,
		arg.Column2,
		arg.Limit,
		arg.Offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []GetAllExpensesByUserIDRow
	for rows.Next() {
		var i GetAllExpensesByUserIDRow
		if err := rows.Scan(
			&i.ID,
			&i.UserID,
			&i.BudgetID,
			&i.Name,
			&i.Category,
			&i.Amount,
			&i.IsRecurring,
			&i.Description,
			&i.DateOccurred,
			&i.CreatedAt,
			&i.UpdatedAt,
			&i.TotalCount,
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

const getAllIncomesByUserID = `-- name: GetAllIncomesByUserID :many
WITH income_data AS (
    SELECT 
        income.id,
        income.user_id,
        income.source,
        income.original_currency_code,
        income.amount_original,
        income.amount,
        income.exchange_rate,
        income.description,
        income.date_received,
        income.created_at,
        income.updated_at
    FROM 
        income
    WHERE 
        income.user_id = $1
        AND ($2 = '' OR to_tsvector('simple', income.source) @@ plainto_tsquery('simple', $2))
    ORDER BY 
        income.date_received DESC
    LIMIT $3 OFFSET $4
),
total_amount AS (
    SELECT SUM(amount)::NUMERIC AS total_income_amount
    FROM income
    WHERE income.user_id = $1
),
most_used_currency AS (
    SELECT original_currency_code
    FROM income
    WHERE income.user_id = $1
    GROUP BY income.original_currency_code
    ORDER BY COUNT(*) DESC
    LIMIT 1
)
SELECT 
    i.id, i.user_id, i.source, i.original_currency_code, i.amount_original, i.amount, i.exchange_rate, i.description, i.date_received, i.created_at, i.updated_at,
    t.total_income_amount,
    m.original_currency_code AS most_used_currency,
    COUNT(*) OVER () AS total_rows
FROM 
    income_data i
    CROSS JOIN total_amount t
    CROSS JOIN most_used_currency m
`

type GetAllIncomesByUserIDParams struct {
	UserID  int64
	Column2 interface{}
	Limit   int32
	Offset  int32
}

type GetAllIncomesByUserIDRow struct {
	ID                   int64
	UserID               int64
	Source               string
	OriginalCurrencyCode string
	AmountOriginal       string
	Amount               string
	ExchangeRate         string
	Description          sql.NullString
	DateReceived         time.Time
	CreatedAt            sql.NullTime
	UpdatedAt            sql.NullTime
	TotalIncomeAmount    string
	MostUsedCurrency     string
	TotalRows            int64
}

func (q *Queries) GetAllIncomesByUserID(ctx context.Context, arg GetAllIncomesByUserIDParams) ([]GetAllIncomesByUserIDRow, error) {
	rows, err := q.db.QueryContext(ctx, getAllIncomesByUserID,
		arg.UserID,
		arg.Column2,
		arg.Limit,
		arg.Offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []GetAllIncomesByUserIDRow
	for rows.Next() {
		var i GetAllIncomesByUserIDRow
		if err := rows.Scan(
			&i.ID,
			&i.UserID,
			&i.Source,
			&i.OriginalCurrencyCode,
			&i.AmountOriginal,
			&i.Amount,
			&i.ExchangeRate,
			&i.Description,
			&i.DateReceived,
			&i.CreatedAt,
			&i.UpdatedAt,
			&i.TotalIncomeAmount,
			&i.MostUsedCurrency,
			&i.TotalRows,
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

const getAllOverdueDebts = `-- name: GetAllOverdueDebts :many
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
LIMIT $1 OFFSET $2
`

type GetAllOverdueDebtsParams struct {
	Limit  int32
	Offset int32
}

type GetAllOverdueDebtsRow struct {
	TotalCount             int64
	ID                     int64
	UserID                 int64
	Name                   string
	Amount                 string
	RemainingBalance       string
	InterestRate           sql.NullString
	Description            sql.NullString
	DueDate                time.Time
	MinimumPayment         string
	CreatedAt              sql.NullTime
	UpdatedAt              sql.NullTime
	NextPaymentDate        time.Time
	EstimatedPayoffDate    sql.NullTime
	AccruedInterest        sql.NullString
	InterestLastCalculated sql.NullTime
	LastPaymentDate        sql.NullTime
	TotalInterestPaid      sql.NullString
}

func (q *Queries) GetAllOverdueDebts(ctx context.Context, arg GetAllOverdueDebtsParams) ([]GetAllOverdueDebtsRow, error) {
	rows, err := q.db.QueryContext(ctx, getAllOverdueDebts, arg.Limit, arg.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []GetAllOverdueDebtsRow
	for rows.Next() {
		var i GetAllOverdueDebtsRow
		if err := rows.Scan(
			&i.TotalCount,
			&i.ID,
			&i.UserID,
			&i.Name,
			&i.Amount,
			&i.RemainingBalance,
			&i.InterestRate,
			&i.Description,
			&i.DueDate,
			&i.MinimumPayment,
			&i.CreatedAt,
			&i.UpdatedAt,
			&i.NextPaymentDate,
			&i.EstimatedPayoffDate,
			&i.AccruedInterest,
			&i.InterestLastCalculated,
			&i.LastPaymentDate,
			&i.TotalInterestPaid,
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

const getAllRecurringExpensesByUserID = `-- name: GetAllRecurringExpensesByUserID :many
SELECT 
    re.id,
    re.user_id,
    re.budget_id,
    b.name AS budget_name,
    re.amount,
    re.name,
    re.description,
    re.recurrence_interval,
    re.projected_amount,
    re.next_occurrence,
    re.created_at,
    re.updated_at,
    COALESCE(SUM(e.amount), 0)::NUMERIC AS total_expenses,
    COUNT(*) OVER() AS total_count
FROM 
    recurring_expenses re
JOIN 
    budgets b 
ON 
    re.budget_id = b.id 
    AND re.user_id = b.user_id  -- Ensures budget belongs to the same user
LEFT JOIN 
    expenses e 
ON 
    re.user_id = e.user_id
    AND re.budget_id = e.budget_id
    AND re.name = e.name 
    AND e.is_recurring = TRUE
WHERE 
    re.user_id = $1 
AND ($2 = '' OR to_tsvector('simple', re.name) @@ plainto_tsquery('simple', $2))
GROUP BY 
    re.id, re.user_id, re.budget_id, b.name, re.amount, 
    re.name, re.description, re.recurrence_interval, 
    re.projected_amount, re.next_occurrence, 
    re.created_at, re.updated_at
ORDER BY 
    re.created_at DESC   
LIMIT 
    $3  -- Limit value for pagination
OFFSET 
    $4
`

type GetAllRecurringExpensesByUserIDParams struct {
	UserID  int64
	Column2 interface{}
	Limit   int32
	Offset  int32
}

type GetAllRecurringExpensesByUserIDRow struct {
	ID                 int64
	UserID             int64
	BudgetID           int64
	BudgetName         string
	Amount             string
	Name               string
	Description        sql.NullString
	RecurrenceInterval RecurrenceIntervalEnum
	ProjectedAmount    string
	NextOccurrence     time.Time
	CreatedAt          sql.NullTime
	UpdatedAt          sql.NullTime
	TotalExpenses      string
	TotalCount         int64
}

func (q *Queries) GetAllRecurringExpensesByUserID(ctx context.Context, arg GetAllRecurringExpensesByUserIDParams) ([]GetAllRecurringExpensesByUserIDRow, error) {
	rows, err := q.db.QueryContext(ctx, getAllRecurringExpensesByUserID,
		arg.UserID,
		arg.Column2,
		arg.Limit,
		arg.Offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []GetAllRecurringExpensesByUserIDRow
	for rows.Next() {
		var i GetAllRecurringExpensesByUserIDRow
		if err := rows.Scan(
			&i.ID,
			&i.UserID,
			&i.BudgetID,
			&i.BudgetName,
			&i.Amount,
			&i.Name,
			&i.Description,
			&i.RecurrenceInterval,
			&i.ProjectedAmount,
			&i.NextOccurrence,
			&i.CreatedAt,
			&i.UpdatedAt,
			&i.TotalExpenses,
			&i.TotalCount,
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

const getAllRecurringExpensesDueForProcessing = `-- name: GetAllRecurringExpensesDueForProcessing :many
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
LIMIT $1 OFFSET $2
`

type GetAllRecurringExpensesDueForProcessingParams struct {
	Limit  int32
	Offset int32
}

type GetAllRecurringExpensesDueForProcessingRow struct {
	TotalCount         int64
	ID                 int64
	UserID             int64
	BudgetID           int64
	Amount             string
	Name               string
	Description        sql.NullString
	RecurrenceInterval RecurrenceIntervalEnum
	ProjectedAmount    string
	NextOccurrence     time.Time
	CreatedAt          sql.NullTime
	UpdatedAt          sql.NullTime
}

func (q *Queries) GetAllRecurringExpensesDueForProcessing(ctx context.Context, arg GetAllRecurringExpensesDueForProcessingParams) ([]GetAllRecurringExpensesDueForProcessingRow, error) {
	rows, err := q.db.QueryContext(ctx, getAllRecurringExpensesDueForProcessing, arg.Limit, arg.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []GetAllRecurringExpensesDueForProcessingRow
	for rows.Next() {
		var i GetAllRecurringExpensesDueForProcessingRow
		if err := rows.Scan(
			&i.TotalCount,
			&i.ID,
			&i.UserID,
			&i.BudgetID,
			&i.Amount,
			&i.Name,
			&i.Description,
			&i.RecurrenceInterval,
			&i.ProjectedAmount,
			&i.NextOccurrence,
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

const getDebtByID = `-- name: GetDebtByID :one
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
WHERE id = $1
`

func (q *Queries) GetDebtByID(ctx context.Context, id int64) (Debt, error) {
	row := q.db.QueryRowContext(ctx, getDebtByID, id)
	var i Debt
	err := row.Scan(
		&i.ID,
		&i.UserID,
		&i.Name,
		&i.Amount,
		&i.RemainingBalance,
		&i.InterestRate,
		&i.Description,
		&i.DueDate,
		&i.MinimumPayment,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.NextPaymentDate,
		&i.EstimatedPayoffDate,
		&i.AccruedInterest,
		&i.InterestLastCalculated,
		&i.LastPaymentDate,
		&i.TotalInterestPaid,
	)
	return i, err
}

const getDebtPaymentsByDebtUserID = `-- name: GetDebtPaymentsByDebtUserID :many
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
LIMIT $5 OFFSET $6
`

type GetDebtPaymentsByDebtUserIDParams struct {
	UserID  int64
	DebtID  int64
	Column3 time.Time
	Column4 time.Time
	Limit   int32
	Offset  int32
}

type GetDebtPaymentsByDebtUserIDRow struct {
	ID                    int64
	DebtID                int64
	UserID                int64
	PaymentAmount         string
	PaymentDate           time.Time
	InterestPayment       string
	PrincipalPayment      string
	CreatedAt             sql.NullTime
	TotalPayments         int64
	TotalPaymentAmount    string
	TotalInterestPayment  string
	TotalPrincipalPayment string
}

func (q *Queries) GetDebtPaymentsByDebtUserID(ctx context.Context, arg GetDebtPaymentsByDebtUserIDParams) ([]GetDebtPaymentsByDebtUserIDRow, error) {
	rows, err := q.db.QueryContext(ctx, getDebtPaymentsByDebtUserID,
		arg.UserID,
		arg.DebtID,
		arg.Column3,
		arg.Column4,
		arg.Limit,
		arg.Offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []GetDebtPaymentsByDebtUserIDRow
	for rows.Next() {
		var i GetDebtPaymentsByDebtUserIDRow
		if err := rows.Scan(
			&i.ID,
			&i.DebtID,
			&i.UserID,
			&i.PaymentAmount,
			&i.PaymentDate,
			&i.InterestPayment,
			&i.PrincipalPayment,
			&i.CreatedAt,
			&i.TotalPayments,
			&i.TotalPaymentAmount,
			&i.TotalInterestPayment,
			&i.TotalPrincipalPayment,
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

const getExpenseByID = `-- name: GetExpenseByID :one
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
WHERE id = $1 AND user_id = $2
`

type GetExpenseByIDParams struct {
	ID     int64
	UserID int64
}

func (q *Queries) GetExpenseByID(ctx context.Context, arg GetExpenseByIDParams) (Expense, error) {
	row := q.db.QueryRowContext(ctx, getExpenseByID, arg.ID, arg.UserID)
	var i Expense
	err := row.Scan(
		&i.ID,
		&i.UserID,
		&i.BudgetID,
		&i.Name,
		&i.Category,
		&i.Amount,
		&i.IsRecurring,
		&i.Description,
		&i.DateOccurred,
		&i.CreatedAt,
		&i.UpdatedAt,
	)
	return i, err
}

const getIncomeByID = `-- name: GetIncomeByID :one
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
WHERE id = $1 AND user_id = $2
`

type GetIncomeByIDParams struct {
	ID     int64
	UserID int64
}

func (q *Queries) GetIncomeByID(ctx context.Context, arg GetIncomeByIDParams) (Income, error) {
	row := q.db.QueryRowContext(ctx, getIncomeByID, arg.ID, arg.UserID)
	var i Income
	err := row.Scan(
		&i.ID,
		&i.UserID,
		&i.Source,
		&i.OriginalCurrencyCode,
		&i.AmountOriginal,
		&i.Amount,
		&i.ExchangeRate,
		&i.Description,
		&i.DateReceived,
		&i.CreatedAt,
		&i.UpdatedAt,
	)
	return i, err
}

const getRecurringExpenseByID = `-- name: GetRecurringExpenseByID :one
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
WHERE id = $1 AND user_id = $2
`

type GetRecurringExpenseByIDParams struct {
	ID     int64
	UserID int64
}

func (q *Queries) GetRecurringExpenseByID(ctx context.Context, arg GetRecurringExpenseByIDParams) (RecurringExpense, error) {
	row := q.db.QueryRowContext(ctx, getRecurringExpenseByID, arg.ID, arg.UserID)
	var i RecurringExpense
	err := row.Scan(
		&i.ID,
		&i.UserID,
		&i.BudgetID,
		&i.Amount,
		&i.Name,
		&i.Description,
		&i.RecurrenceInterval,
		&i.ProjectedAmount,
		&i.NextOccurrence,
		&i.CreatedAt,
		&i.UpdatedAt,
	)
	return i, err
}

const updateDebtByID = `-- name: UpdateDebtByID :one
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
RETURNING updated_at
`

type UpdateDebtByIDParams struct {
	ID                     int64
	Name                   string
	Amount                 string
	RemainingBalance       string
	InterestRate           sql.NullString
	Description            sql.NullString
	DueDate                time.Time
	MinimumPayment         string
	NextPaymentDate        time.Time
	AccruedInterest        sql.NullString
	TotalInterestPaid      sql.NullString
	EstimatedPayoffDate    sql.NullTime
	InterestLastCalculated sql.NullTime
	LastPaymentDate        sql.NullTime
	UserID                 int64
}

func (q *Queries) UpdateDebtByID(ctx context.Context, arg UpdateDebtByIDParams) (sql.NullTime, error) {
	row := q.db.QueryRowContext(ctx, updateDebtByID,
		arg.ID,
		arg.Name,
		arg.Amount,
		arg.RemainingBalance,
		arg.InterestRate,
		arg.Description,
		arg.DueDate,
		arg.MinimumPayment,
		arg.NextPaymentDate,
		arg.AccruedInterest,
		arg.TotalInterestPaid,
		arg.EstimatedPayoffDate,
		arg.InterestLastCalculated,
		arg.LastPaymentDate,
		arg.UserID,
	)
	var updated_at sql.NullTime
	err := row.Scan(&updated_at)
	return updated_at, err
}

const updateExpenseByID = `-- name: UpdateExpenseByID :one
UPDATE expenses SET
    name = $1,
    category = $2,
    amount = $3,
    is_recurring = $4,
    description = $5,
    date_occurred = $6
WHERE
    id = $7 AND user_id = $8
RETURNING updated_at
`

type UpdateExpenseByIDParams struct {
	Name         string
	Category     string
	Amount       string
	IsRecurring  bool
	Description  sql.NullString
	DateOccurred time.Time
	ID           int64
	UserID       int64
}

func (q *Queries) UpdateExpenseByID(ctx context.Context, arg UpdateExpenseByIDParams) (sql.NullTime, error) {
	row := q.db.QueryRowContext(ctx, updateExpenseByID,
		arg.Name,
		arg.Category,
		arg.Amount,
		arg.IsRecurring,
		arg.Description,
		arg.DateOccurred,
		arg.ID,
		arg.UserID,
	)
	var updated_at sql.NullTime
	err := row.Scan(&updated_at)
	return updated_at, err
}

const updateIncomeByID = `-- name: UpdateIncomeByID :one
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
RETURNING updated_at
`

type UpdateIncomeByIDParams struct {
	Source               string
	OriginalCurrencyCode string
	AmountOriginal       string
	Amount               string
	ExchangeRate         string
	Description          sql.NullString
	DateReceived         time.Time
	ID                   int64
	UserID               int64
}

func (q *Queries) UpdateIncomeByID(ctx context.Context, arg UpdateIncomeByIDParams) (sql.NullTime, error) {
	row := q.db.QueryRowContext(ctx, updateIncomeByID,
		arg.Source,
		arg.OriginalCurrencyCode,
		arg.AmountOriginal,
		arg.Amount,
		arg.ExchangeRate,
		arg.Description,
		arg.DateReceived,
		arg.ID,
		arg.UserID,
	)
	var updated_at sql.NullTime
	err := row.Scan(&updated_at)
	return updated_at, err
}

const updateRecurringExpenseByID = `-- name: UpdateRecurringExpenseByID :one
UPDATE recurring_expenses SET
    amount = $1,
    name = $2,
    description = $3,
    recurrence_interval = $4,
    projected_amount = $5,
    next_occurrence = $6
WHERE
    id = $7 AND user_id = $8
RETURNING  updated_at
`

type UpdateRecurringExpenseByIDParams struct {
	Amount             string
	Name               string
	Description        sql.NullString
	RecurrenceInterval RecurrenceIntervalEnum
	ProjectedAmount    string
	NextOccurrence     time.Time
	ID                 int64
	UserID             int64
}

func (q *Queries) UpdateRecurringExpenseByID(ctx context.Context, arg UpdateRecurringExpenseByIDParams) (sql.NullTime, error) {
	row := q.db.QueryRowContext(ctx, updateRecurringExpenseByID,
		arg.Amount,
		arg.Name,
		arg.Description,
		arg.RecurrenceInterval,
		arg.ProjectedAmount,
		arg.NextOccurrence,
		arg.ID,
		arg.UserID,
	)
	var updated_at sql.NullTime
	err := row.Scan(&updated_at)
	return updated_at, err
}
