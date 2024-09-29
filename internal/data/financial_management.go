package data

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"time"

	"github.com/Blue-Davinci/OptiVest/internal/database"
	"github.com/Blue-Davinci/OptiVest/internal/validator"
	"github.com/shopspring/decimal"
)

const (
	FinManEnumGoalStatusOngoing   = database.GoalStatusOngoing
	FinManEnumGoalStatusCompleted = database.GoalStatusCompleted
	FinManEnumGoalStatusFailed    = database.GoalStatusCancelled
)

type FinancialManagerModel struct {
	DB *database.Queries
}

const (
	DefaultFinManDBContextTimeout = 5 * time.Second
)

type Budget struct {
	Id             int64           `json:"id"`
	UserID         int64           `json:"user_id"`
	Name           string          `json:"name"`
	IsStrict       bool            `json:"is_strict"`
	Category       string          `json:"category"`
	TotalAmount    decimal.Decimal `json:"total_amount"`
	CurrencyCode   string          `json:"currency_code"`
	ConversionRate decimal.Decimal `json:"conversion_rate"`
	Description    string          `json:"description"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

type Goals struct {
	Id                  int64               `json:"id"`
	UserID              int64               `json:"user_id"`
	BudgetID            int64               `json:"budget_id"`
	Name                string              `json:"name"`
	CurrentAmount       decimal.Decimal     `json:"current_amount"`
	TargetAmount        decimal.Decimal     `json:"target_amount"`
	MonthlyContribution decimal.Decimal     `json:"monthly_contribution"`
	StartDate           time.Time           `json:"start_date"`
	EndDate             time.Time           `json:"end_date"`
	Status              database.GoalStatus `json:"status"`
	CreatedAt           time.Time           `json:"created_at"`
	UpdatedAt           time.Time           `json:"updated_at"`
}

func ValidateBudgetName(v *validator.Validator, name string) {
	v.Check(name != "", "name", "must be provided")
	v.Check(len(name) <= 255, "name", "must not be more than 255 bytes long")
}

func ValidateBudgetCategory(v *validator.Validator, category string) {
	v.Check(category != "", "category", "must be provided")
	v.Check(len(category) <= 255, "category", "must not be more than 255 bytes long")
}

func ValidateBudgetTotalAmount(v *validator.Validator, totalAmount decimal.Decimal) {
	v.Check(totalAmount.GreaterThan(decimal.NewFromInt(0)), "total_amount", "must be greater than 0")
}
func ValidateCurrencyCode(v *validator.Validator, currencyCode string) {
	v.Check(currencyCode != "", "currency_code", "must be provided")
	v.Check(len(currencyCode) != 3, "currency_code", "must not be 3 bytes long")
	// check if currency is in the list of supported currencies
}
func ValidateBudgetDescription(v *validator.Validator, description string) {
	v.Check(description != "", "description", "must be provided")
	v.Check(len(description) <= 500, "description", "must not be more than 500 bytes long")
}
func ValidateBudgetStrictness(v *validator.Validator, isStrict bool) {
	v.Check(reflect.TypeOf(isStrict).Kind() == reflect.Bool, "is_strict", "must be a boolean")
}

func ValidateBudget(v *validator.Validator, budget *Budget) {
	// Budget name
	ValidateBudgetName(v, budget.Name)
	// Budget category
	ValidateBudgetCategory(v, budget.Category)
	// Total amount
	ValidateBudgetTotalAmount(v, budget.TotalAmount)
	// Currency code
	ValidateCurrencyCode(v, budget.CurrencyCode)
	// Description
	ValidateBudgetDescription(v, budget.Description)
	// IsStrict
	ValidateBudgetStrictness(v, budget.IsStrict)
}

// CreateNewBudget() creates a new budget record in the database
// It takes a pointer to a Budget struct and returns an error if
// the operation fails.
func (m FinancialManagerModel) CreateNewBudget(newBudget *Budget) error {
	ctx, cancel := contextGenerator(context.Background(), DefaultFinManDBContextTimeout)
	defer cancel()
	budget, err := m.DB.CreateNewBudget(ctx, database.CreateNewBudgetParams{
		UserID:         newBudget.UserID,
		Name:           newBudget.Name,
		IsStrict:       newBudget.IsStrict,
		Category:       newBudget.Category,
		TotalAmount:    newBudget.TotalAmount.String(),
		CurrencyCode:   newBudget.CurrencyCode,
		ConversionRate: newBudget.ConversionRate.String(),
		Description:    sql.NullString{String: newBudget.Description, Valid: newBudget.Description != ""},
	})
	if err != nil {
		return err
	}
	// fill in the newBudget with the ID and timestamps
	newBudget.Id = budget.ID
	newBudget.CreatedAt = budget.CreatedAt
	newBudget.UpdatedAt = budget.UpdatedAt
	// everything went well
	return nil
}

// DeleteBudgetByID() deletes a budget record from the database by its ID
// It takes the budget ID as a parameter and returns an error if the operation fails.
func (m FinancialManagerModel) DeleteBudgetByID(userID, budgetID int64) (*int64, error) {
	ctx, cancel := contextGenerator(context.Background(), DefaultFinManDBContextTimeout)
	defer cancel()
	deletedID, err := m.DB.DeleteBudgetById(ctx, database.DeleteBudgetByIdParams{
		UserID: userID,
		ID:     budgetID,
	})
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrGeneralRecordNotFound
		default:
			return nil, err
		}
	}
	// everything went well
	return &deletedID, nil
}

// UpdateBudgetByID() updates a budget record in the database by its ID
// It takes the budget ID and a pointer to a Budget struct as parameters
// We do not allow users the CurrencyCode but they can change the conversion rate

// GetBudgetByID() retrieves a budget record from the database by its ID
// It takes the budget ID as a parameter and returns a pointer to a Budget struct
// and an error if the operation fails.
func (m FinancialManagerModel) GetBudgetByID(id int64) (*Budget, error) {
	ctx, cancel := contextGenerator(context.Background(), DefaultFinManDBContextTimeout)
	defer cancel()
	budget, err := m.DB.GetBudgetByID(ctx, id)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrGeneralRecordNotFound
		default:
			return nil, err
		}
	}
	// fill in the Budget struct with the data from the database
	idBudgets := populateBudget(budget)
	// everything went well
	return idBudgets, nil
}

// populateBudget() fills a Budget struct with data from the database
// It takes an interface{} as a parameter and returns a pointer to a Budget struct
// or nil if the input type does not match any supported types.
func populateBudget(budgetRow interface{}) *Budget {
	switch budget := budgetRow.(type) {
	case database.Budget:
		return &Budget{
			Id:             budget.ID,
			UserID:         budget.UserID,
			Name:           budget.Name,
			IsStrict:       budget.IsStrict,
			Category:       budget.Category,
			TotalAmount:    decimal.RequireFromString(budget.TotalAmount),
			CurrencyCode:   budget.CurrencyCode,
			ConversionRate: decimal.RequireFromString(budget.ConversionRate),
			Description:    budget.Description.String,
			CreatedAt:      budget.CreatedAt,
			UpdatedAt:      budget.UpdatedAt,
		}
		// Default case: Returns nil if the input type does not match any supported types.
	default:
		return nil
	}
}

// ============================================================================================================
// Goals
// ============================================================================================================

// CreateNewGoal() creates a new goal record in the database
// It takes a pointer to a Goals struct and returns an error if the operation fails.
func (m FinancialManagerModel) CreateNewGoal(newGoal *Goals) error {
	ctx, cancel := contextGenerator(context.Background(), DefaultFinManDBContextTimeout)
	defer cancel()
	goal, err := m.DB.CreateNewGoal(ctx, database.CreateNewGoalParams{
		UserID:              newGoal.UserID,
		BudgetID:            sql.NullInt64{Int64: newGoal.BudgetID, Valid: newGoal.BudgetID != 0},
		Name:                newGoal.Name,
		CurrentAmount:       sql.NullString{String: newGoal.CurrentAmount.String(), Valid: newGoal.CurrentAmount.String() != ""},
		TargetAmount:        newGoal.TargetAmount.String(),
		MonthlyContribution: newGoal.MonthlyContribution.String(),
		StartDate:           newGoal.StartDate,
		EndDate:             newGoal.EndDate,
		Status:              newGoal.Status,
	})
	if err != nil {
		return err
	}
	// fill in the newGoal with the ID and timestamps
	newGoal.Id = goal.ID
	newGoal.CreatedAt = goal.CreatedAt
	newGoal.UpdatedAt = goal.UpdatedAt
	// everything went well
	return nil
}
