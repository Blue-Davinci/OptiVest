// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.27.0
// source: general_queries.sql

package database

import (
	"context"
	"database/sql"
)

const createContactUs = `-- name: CreateContactUs :one
INSERT INTO contact_us(
    user_id,
    name,
    email,
    subject,
    message
) VALUES ($1, $2, $3, $4, $5) 
RETURNING id,status,created_at,updated_at
`

type CreateContactUsParams struct {
	UserID  sql.NullInt64
	Name    string
	Email   string
	Subject string
	Message string
}

type CreateContactUsRow struct {
	ID        int64
	Status    NullContactUsStatus
	CreatedAt sql.NullTime
	UpdatedAt sql.NullTime
}

func (q *Queries) CreateContactUs(ctx context.Context, arg CreateContactUsParams) (CreateContactUsRow, error) {
	row := q.db.QueryRowContext(ctx, createContactUs,
		arg.UserID,
		arg.Name,
		arg.Email,
		arg.Subject,
		arg.Message,
	)
	var i CreateContactUsRow
	err := row.Scan(
		&i.ID,
		&i.Status,
		&i.CreatedAt,
		&i.UpdatedAt,
	)
	return i, err
}
