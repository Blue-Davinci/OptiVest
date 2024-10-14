// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.26.0
// source: award_queries.sql

package database

import (
	"context"
	"time"
)

const createNewUserAward = `-- name: CreateNewUserAward :one
INSERT INTO user_awards (user_id, award_id)
VALUES ($1, $2)
RETURNING created_at
`

type CreateNewUserAwardParams struct {
	UserID  int64
	AwardID int32
}

func (q *Queries) CreateNewUserAward(ctx context.Context, arg CreateNewUserAwardParams) (time.Time, error) {
	row := q.db.QueryRowContext(ctx, createNewUserAward, arg.UserID, arg.AwardID)
	var created_at time.Time
	err := row.Scan(&created_at)
	return created_at, err
}

const getAllAwards = `-- name: GetAllAwards :many
SELECT 
    id,
    code,
    description,
    created_at,
    updated_at
FROM awards
`

type GetAllAwardsRow struct {
	ID          int32
	Code        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (q *Queries) GetAllAwards(ctx context.Context) ([]GetAllAwardsRow, error) {
	rows, err := q.db.QueryContext(ctx, getAllAwards)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []GetAllAwardsRow
	for rows.Next() {
		var i GetAllAwardsRow
		if err := rows.Scan(
			&i.ID,
			&i.Code,
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

const getAllAwardsForUserByID = `-- name: GetAllAwardsForUserByID :many
SELECT a.id, a.code, a.description, a.point, a.created_at, a.updated_at
FROM awards a
INNER JOIN user_awards ua ON ua.award_id = a.id
WHERE ua.user_id = $1
`

func (q *Queries) GetAllAwardsForUserByID(ctx context.Context, userID int64) ([]Award, error) {
	rows, err := q.db.QueryContext(ctx, getAllAwardsForUserByID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Award
	for rows.Next() {
		var i Award
		if err := rows.Scan(
			&i.ID,
			&i.Code,
			&i.Description,
			&i.Point,
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
