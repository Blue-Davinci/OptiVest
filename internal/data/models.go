package data

import (
	"errors"

	"github.com/Blue-Davinci/OptiVest/internal/database"
)

var (
	ErrFailedToSaveRecordToRedis = errors.New("failed to save record to database")
	ErrUnableToQueryDatabase     = errors.New("unable to query database")
	ErrGeneralRecordNotFound     = errors.New("feeds record not found")
	ErrGeneralEditConflict       = errors.New("edit conflict")
)

type Models struct {
	Users                 UserModel
	Tokens                TokenModel
	ApiManager            ApiManagerModel
	FinancialManager      FinancialManagerModel
	FinancialGroupManager FinancialGroupManagerModel
}

func NewModels(db *database.Queries) Models {
	return Models{
		Users:                 UserModel{DB: db},
		Tokens:                TokenModel{DB: db},
		ApiManager:            ApiManagerModel{DB: db},
		FinancialManager:      FinancialManagerModel{DB: db},
		FinancialGroupManager: FinancialGroupManagerModel{DB: db},
	}
}
