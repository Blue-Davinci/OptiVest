// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.26.0
// source: auth_token_queries.sql

package database

import (
	"context"
	"database/sql"
	"time"
)

const createNewToken = `-- name: CreateNewToken :one
INSERT INTO tokens (hash, user_id, expiry, scope)
VALUES ($1, $2, $3, $4)
RETURNING user_id
`

type CreateNewTokenParams struct {
	Hash   []byte
	UserID int64
	Expiry time.Time
	Scope  string
}

func (q *Queries) CreateNewToken(ctx context.Context, arg CreateNewTokenParams) (int64, error) {
	row := q.db.QueryRowContext(ctx, createNewToken,
		arg.Hash,
		arg.UserID,
		arg.Expiry,
		arg.Scope,
	)
	var user_id int64
	err := row.Scan(&user_id)
	return user_id, err
}

const deletAllTokensForUser = `-- name: DeletAllTokensForUser :exec
DELETE FROM tokens
WHERE scope = $1 AND user_id = $2
`

type DeletAllTokensForUserParams struct {
	Scope  string
	UserID int64
}

func (q *Queries) DeletAllTokensForUser(ctx context.Context, arg DeletAllTokensForUserParams) error {
	_, err := q.db.ExecContext(ctx, deletAllTokensForUser, arg.Scope, arg.UserID)
	return err
}

const getForToken = `-- name: GetForToken :one
SELECT
    users.id,
    users.first_name,
    users.last_name,
    users.email,
    users.profile_avatar_url,
    users.password,
    users.phone_number,
    users.activated,
    users.version,
    users.created_at,
    users.updated_at,
    users.last_login,
    users.profile_completed,
    users.dob,
    users.address,
    users.country_code,
    users.currency_code,
    users.mfa_enabled,
    users.mfa_secret,
    users.mfa_status,
    users.mfa_last_checked
FROM users
INNER JOIN tokens
ON users.id = tokens.user_id
WHERE tokens.hash = $1
AND tokens.scope = $2
AND tokens.expiry > $3
`

type GetForTokenParams struct {
	Hash   []byte
	Scope  string
	Expiry time.Time
}

type GetForTokenRow struct {
	ID               int64
	FirstName        string
	LastName         string
	Email            string
	ProfileAvatarUrl string
	Password         []byte
	PhoneNumber      string
	Activated        bool
	Version          int32
	CreatedAt        time.Time
	UpdatedAt        time.Time
	LastLogin        time.Time
	ProfileCompleted bool
	Dob              time.Time
	Address          sql.NullString
	CountryCode      sql.NullString
	CurrencyCode     sql.NullString
	MfaEnabled       bool
	MfaSecret        sql.NullString
	MfaStatus        NullMfaStatusType
	MfaLastChecked   sql.NullTime
}

func (q *Queries) GetForToken(ctx context.Context, arg GetForTokenParams) (GetForTokenRow, error) {
	row := q.db.QueryRowContext(ctx, getForToken, arg.Hash, arg.Scope, arg.Expiry)
	var i GetForTokenRow
	err := row.Scan(
		&i.ID,
		&i.FirstName,
		&i.LastName,
		&i.Email,
		&i.ProfileAvatarUrl,
		&i.Password,
		&i.PhoneNumber,
		&i.Activated,
		&i.Version,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.LastLogin,
		&i.ProfileCompleted,
		&i.Dob,
		&i.Address,
		&i.CountryCode,
		&i.CurrencyCode,
		&i.MfaEnabled,
		&i.MfaSecret,
		&i.MfaStatus,
		&i.MfaLastChecked,
	)
	return i, err
}
