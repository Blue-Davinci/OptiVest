// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.27.0
// source: search_option_queries.sql

package database

import (
	"context"
)

const getDistinctBudgetCategory = `-- name: GetDistinctBudgetCategory :many
SELECT DISTINCT category
FROM Budgets
WHERE user_id = $1
`

func (q *Queries) GetDistinctBudgetCategory(ctx context.Context, userID int64) ([]string, error) {
	rows, err := q.db.QueryContext(ctx, getDistinctBudgetCategory, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []string
	for rows.Next() {
		var category string
		if err := rows.Scan(&category); err != nil {
			return nil, err
		}
		items = append(items, category)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const getDistinctBudgetIdBudgetName = `-- name: GetDistinctBudgetIdBudgetName :many
SELECT id, name
FROM Budgets
WHERE user_id = $1
`

type GetDistinctBudgetIdBudgetNameRow struct {
	ID   int64
	Name string
}

func (q *Queries) GetDistinctBudgetIdBudgetName(ctx context.Context, userID int64) ([]GetDistinctBudgetIdBudgetNameRow, error) {
	rows, err := q.db.QueryContext(ctx, getDistinctBudgetIdBudgetName, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []GetDistinctBudgetIdBudgetNameRow
	for rows.Next() {
		var i GetDistinctBudgetIdBudgetNameRow
		if err := rows.Scan(&i.ID, &i.Name); err != nil {
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
