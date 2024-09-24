package data

import (
	"errors"

	"github.com/Blue-Davinci/OptiVest/internal/database"
)

var (
	ErrGeneralRecordNotFound = errors.New("feeds record not found")
	ErrGeneralEditConflict   = errors.New("edit conflict")
)

type Models struct {
	Users UserModel
}

func NewModels(db *database.Queries) Models {
	return Models{
		Users: UserModel{DB: db},
	}
}
