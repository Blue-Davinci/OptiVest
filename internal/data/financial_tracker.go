package data

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Blue-Davinci/OptiVest/internal/database"
	"github.com/Blue-Davinci/OptiVest/internal/validator"
	"github.com/shopspring/decimal"
)

type FinancialTrackingModel struct {
	DB *database.Queries
}

const (
	FinTrackEnumRecurremceEnumDaily = database.RecurrenceIntervalEnumDaily
	FinTrackEnumRecurrenceWeekly    = database.RecurrenceIntervalEnumWeekly
	FinTrackEnumRecurremceMonthly   = database.RecurrenceIntervalEnumMonthly
	FinTrackEnumRecurenceYearly     = database.RecurrenceIntervalEnumYearly
)

var (
	DefaultFinTrackDBContextTimeout = 5 * time.Second
)

var (
	ErrInvalidRecurringExpenseTime = errors.New("invalid recurring expense time")
	ErrDuplicateRecurringExpense   = errors.New("recurring expense already exists")
)

// Represents an expense
type Expense struct {
	ID           int64           `json:"id"`
	UserID       int64           `json:"user_id"`
	BudgetID     int64           `json:"budget_id"`
	Name         string          `json:"name"`
	Category     string          `json:"category"`
	Amount       decimal.Decimal `json:"amount"`
	IsRecurring  bool            `json:"is_recurring"`
	Description  string          `json:"description"`
	DateOccurred time.Time       `json:"date_occurred"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

// Represents a recurring expense
type RecurringExpense struct {
	ID                 int64                           `json:"id"`                  // Unique ID for the recurring expense
	UserID             int64                           `json:"user_id"`             // Reference to the user
	BudgetID           int64                           `json:"budget_id"`           // Link to the budget
	Amount             decimal.Decimal                 `json:"amount"`              // Amount of the recurring expense
	Name               string                          `json:"name"`                // Name of the expense
	Description        string                          `json:"description"`         // Description of the expense
	RecurrenceInterval database.RecurrenceIntervalEnum `json:"recurrence_interval"` // Interval type (e.g., daily, weekly, monthly, etc.)
	ProjectedAmount    decimal.Decimal                 `json:"projected_amount"`    // The total amount of the expense per month
	NextOccurrence     time.Time                       `json:"next_occurrence"`     // The next date the expense should be added
	CreatedAt          time.Time                       `json:"created_at"`          // Creation timestamp
	UpdatedAt          time.Time                       `json:"updated_at"`          // Last updated timestamp
}

// Represents an income
type Income struct {
	ID                   int64           `json:"id"`
	UserID               int64           `json:"user_id"`
	Source               string          `json:"source"`
	OriginalCurrencyCode string          `json:"original_currency_code"`
	AmountOriginal       decimal.Decimal `json:"amount_original"`
	Amount               decimal.Decimal `json:"amount"`
	ExchangeRate         decimal.Decimal `json:"exchange_rate"`
	Description          string          `json:"description"`
	DateReceived         time.Time       `json:"date_received"`
	CreatedAt            time.Time       `json:"created_at"`
	UpdatedAt            time.Time       `json:"updated_at"`
}

// Map a recurring expense to a corresponding constant
func (expense *FinancialTrackingModel) MapToDatabaseRecurringExpense(interval string) (database.RecurrenceIntervalEnum, error) {
	switch interval {
	case "daily":
		return database.RecurrenceIntervalEnumDaily, nil
	case "weekly":
		return database.RecurrenceIntervalEnumWeekly, nil
	case "monthly":
		return database.RecurrenceIntervalEnumMonthly, nil
	case "yearly":
		return database.RecurrenceIntervalEnumYearly, nil
	default:
		return "", ErrInvalidRecurringExpenseTime
	}
}

// Per month, calculate the total amount of an expense based on the recurrence interval
// If a recurring expense is set to monthly, the total amount will be the amount of the expense
// If a recurring expense is set to weekly, the total amount will be the amount of the expense * 4
// If a recurring expense is set to daily, the total amount will be the amount of the expense * number of days in a month
func (re *RecurringExpense) CalculateTotalAmountPerMonth() decimal.Decimal {
	switch re.RecurrenceInterval {
	case database.RecurrenceIntervalEnumDaily:
		return re.Amount.Mul(decimal.NewFromFloat(30)) // Monthly projection
	case database.RecurrenceIntervalEnumWeekly:
		return re.Amount.Mul(decimal.NewFromFloat(4)) // Monthly projection
	case database.RecurrenceIntervalEnumMonthly:
		return re.Amount
	case database.RecurrenceIntervalEnumYearly:
		return re.Amount.Div(decimal.NewFromFloat(12)) // Monthly projection
	default:
		return decimal.Zero
	}
}

// Calculate the next occurrence of a recurring expense
// We will get the start date and frequency of the expense
// We will then calculate the next occurrence based on the frequency
func (re *RecurringExpense) CalculateNextOccurrence() {
	now := time.Now()
	switch re.RecurrenceInterval {
	case database.RecurrenceIntervalEnumDaily:
		re.NextOccurrence = now.Add(24 * time.Hour)
	case database.RecurrenceIntervalEnumWeekly:
		re.NextOccurrence = now.Add(7 * 24 * time.Hour)
	case database.RecurrenceIntervalEnumMonthly:
		re.NextOccurrence = now.AddDate(0, 1, 0)
	case database.RecurrenceIntervalEnumYearly:
		re.NextOccurrence = now.AddDate(1, 0, 0)
	}
}
func ValidateNextOccurrence(v *validator.Validator, nextOccurrence time.Time) {
	v.Check(nextOccurrence.Before(time.Now()), "next_occurrence", "cannot be in the past")
}

// validate a recurring expense
func ValidateRecurringExpense(v *validator.Validator, expense *RecurringExpense) {
	ValidateAmount(v, expense.Amount, "amount")
	ValidateBudgetDescription(v, expense.Description)
	ValidateNextOccurrence(v, expense.NextOccurrence)
	ValidateName(v, expense.Name, "name")
}

// validate expense
func ValidateExpense(v *validator.Validator, expense *Expense) {
	ValidateAmount(v, expense.Amount, "amount")
	ValidateBudgetDescription(v, expense.Description)
	ValidateName(v, expense.Name, "name")
	ValidateName(v, expense.Category, "category")
}

// validate an income
func ValidateIncome(v *validator.Validator, income *Income) {
	ValidateAmount(v, income.Amount, "amount")
	ValidateAmount(v, income.AmountOriginal, "amount_original")
	ValidateAmount(v, income.ExchangeRate, "exchange_rate")
	ValidateBudgetDescription(v, income.Description)
	ValidateName(v, income.Source, "source")
	ValidateName(v, income.OriginalCurrencyCode, "original_currency_code")
}

// CreateNewRecurringExpense() Creates a new recurrent expens in the recurrence table
// A trigger automatically adds it to the expenses table to be tracked
func (m *FinancialTrackingModel) CreateNewRecurringExpense(userID int64, recurringExpense *RecurringExpense) error {
	// set our context
	ctx, cancel := contextGenerator(context.Background(), DefaultFinTrackDBContextTimeout)
	defer cancel()
	// create the expense
	updatedDetails, err := m.DB.CreateNewRecurringExpense(ctx, database.CreateNewRecurringExpenseParams{
		UserID:             userID,
		BudgetID:           recurringExpense.BudgetID,
		Amount:             recurringExpense.Amount.String(),
		Name:               recurringExpense.Name,
		Description:        sql.NullString{String: recurringExpense.Description, Valid: true},
		RecurrenceInterval: recurringExpense.RecurrenceInterval,
		ProjectedAmount:    recurringExpense.CalculateTotalAmountPerMonth().String(),
		NextOccurrence:     recurringExpense.NextOccurrence,
	})
	if err != nil {
		switch {
		case err.Error() == `pq: duplicate key value violates unique constraint "unique_recurring_expense"`:
			return ErrDuplicateRecurringExpense
		default:
			return err
		}

	}
	// update the expense with the new details
	recurringExpense.ID = updatedDetails.ID
	recurringExpense.UserID = userID
	recurringExpense.CreatedAt = updatedDetails.CreatedAt.Time
	recurringExpense.UpdatedAt = updatedDetails.UpdatedAt.Time
	// we are good
	return nil
}

// UpdateRecurringExpenseByID() updates a recurring expense by its ID
func (m *FinancialTrackingModel) UpdateRecurringExpenseByID(userID int64, recurringExpense *RecurringExpense) error {
	// set our context
	ctx, cancel := contextGenerator(context.Background(), DefaultFinTrackDBContextTimeout)
	defer cancel()
	// create the expense
	updatedAt, err := m.DB.UpdateRecurringExpenseByID(ctx, database.UpdateRecurringExpenseByIDParams{
		ID:                 recurringExpense.ID,
		UserID:             userID,
		Amount:             recurringExpense.Amount.String(),
		Name:               recurringExpense.Name,
		Description:        sql.NullString{String: recurringExpense.Description, Valid: true},
		RecurrenceInterval: recurringExpense.RecurrenceInterval,
		ProjectedAmount:    recurringExpense.CalculateTotalAmountPerMonth().String(),
		NextOccurrence:     recurringExpense.NextOccurrence,
	})
	if err != nil {
		switch {
		case err.Error() == `pq: duplicate key value violates unique constraint "unique_recurring_expense"`:
			return ErrDuplicateRecurringExpense
		case errors.Is(err, sql.ErrNoRows):
			return ErrEditConflict
		default:
			return err
		}
	}
	// update the expense with the new details
	recurringExpense.UpdatedAt = updatedAt.Time
	// we are good
	return nil
}

// GetRecurringExpenseByID() gets a recurring expense by its ID
func (m *FinancialTrackingModel) GetRecurringExpenseByID(userID, recurringExpenseID int64) (*RecurringExpense, error) {
	// set our context
	ctx, cancel := contextGenerator(context.Background(), DefaultFinTrackDBContextTimeout)
	defer cancel()
	// get the expense
	recurringExpense, err := m.DB.GetRecurringExpenseByID(ctx, database.GetRecurringExpenseByIDParams{
		ID:     recurringExpenseID,
		UserID: userID,
	})
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrGeneralRecordNotFound
		default:
			return nil, err
		}
	}
	// we are good, populate the expense
	populatedExpens := populateRecurringExpense(recurringExpense)
	// we are good
	return populatedExpens, nil
}

// GetAllRecurringExpensesDueForProcessing() gets all the recurring expenses that are due for processing
// That is, we get all recurring expenses that have a next occurrence that is less than or equal to the current time
// We will need an offset and a limit so that we support batch processing.
// This method is made to work in tandem with our cron job
func (m *FinancialTrackingModel) GetAllRecurringExpensesDueForProcessing(filters Filters) ([]*RecurringExpense, Metadata, error) {
	// set our context
	ctx, cancel := contextGenerator(context.Background(), DefaultFinTrackDBContextTimeout)
	defer cancel()
	// get the expenses
	recurringExpenses, err := m.DB.GetAllRecurringExpensesDueForProcessing(ctx, database.GetAllRecurringExpensesDueForProcessingParams{
		Limit:  int32(filters.limit()),
		Offset: int32(filters.offset()),
	})
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, Metadata{}, ErrGeneralRecordNotFound
		default:
			return nil, Metadata{}, err
		}
	}
	// set totals
	totalRecords := 0
	// populate the expenses
	var populatedExpenses []*RecurringExpense
	for _, expense := range recurringExpenses {
		totalRecords = int(expense.TotalCount)
		populatedExpenses = append(populatedExpenses, populateRecurringExpense(expense))
	}
	// calculate metadata
	metadata := calculateMetadata(totalRecords, filters.Page, filters.PageSize)
	// we are good
	return populatedExpenses, metadata, nil

}

// CreateNewExpense() creates a new expense in the expenses table
// This expense is a one way expense in that it is not recurring
// But the caller still needs to verify that the surplus is enough to cover the expense
func (m *FinancialTrackingModel) CreateNewExpense(userID int64, expense *Expense) error {
	// make context
	ctx, cancel := contextGenerator(context.Background(), DefaultFinTrackDBContextTimeout)
	defer cancel()
	// create the expense
	createdExpense, err := m.DB.CreateNewExpense(ctx, database.CreateNewExpenseParams{
		UserID:       userID,
		BudgetID:     expense.BudgetID,
		Name:         expense.Name,
		Category:     expense.Category,
		Amount:       expense.Amount.String(),
		IsRecurring:  expense.IsRecurring,
		Description:  sql.NullString{String: expense.Description, Valid: true},
		DateOccurred: expense.DateOccurred,
	})
	if err != nil {
		return err
	}
	// set the created expense
	expense.ID = createdExpense.ID
	expense.UserID = userID
	expense.CreatedAt = createdExpense.CreatedAt.Time
	expense.UpdatedAt = createdExpense.UpdatedAt.Time
	// we are good
	return nil
}

// UpdateExpenseByID() is a method that updates an expense by its ID and user ID
// We enrich it back with the updated at timestamp
func (m *FinancialTrackingModel) UpdateExpenseByID(userID int64, expense *Expense) error {
	// set our context
	ctx, cancel := contextGenerator(context.Background(), DefaultFinTrackDBContextTimeout)
	defer cancel()
	// create the expense
	updatedAt, err := m.DB.UpdateExpenseByID(ctx, database.UpdateExpenseByIDParams{
		Name:         expense.Name,
		Category:     expense.Category,
		Amount:       expense.Amount.String(),
		IsRecurring:  expense.IsRecurring,
		Description:  sql.NullString{String: expense.Description, Valid: true},
		DateOccurred: expense.DateOccurred,
		ID:           expense.ID,
		UserID:       userID,
	})
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return ErrEditConflict
		default:
			return err
		}
	}
	// set the updated expense
	expense.UpdatedAt = updatedAt.Time
	// we are good
	return nil
}

// GetExpenseByID() gets an expense by both the ID and the user ID
// We return back an expense and an error if any was found
func (m *FinancialTrackingModel) GetExpenseByID(userID, expenseID int64) (*Expense, error) {
	// set our context
	ctx, cancel := contextGenerator(context.Background(), DefaultFinTrackDBContextTimeout)
	defer cancel()
	// get the expense
	expense, err := m.DB.GetExpenseByID(ctx, database.GetExpenseByIDParams{
		ID:     expenseID,
		UserID: userID,
	})
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrGeneralRecordNotFound
		default:
			return nil, err
		}
	}
	// we are good, populate the expense
	updatedExpense := populateExpense(expense)
	// we are good
	return updatedExpense, nil
}

// =========================================================================================================
// Income
// =========================================================================================================

// CreateNewIncome() creates a new income in the income table
// We get a user ID and a pointer to an income struct
// We return an error if any was found
func (m *FinancialTrackingModel) CreateNewIncome(userID int64, income *Income) error {
	// set our context
	ctx, cancel := contextGenerator(context.Background(), DefaultFinTrackDBContextTimeout)
	defer cancel()
	// create the income
	incomeDetails, err := m.DB.CreateNewIncome(ctx, database.CreateNewIncomeParams{
		UserID:               userID,
		Source:               income.Source,
		OriginalCurrencyCode: income.OriginalCurrencyCode,
		AmountOriginal:       income.AmountOriginal.String(),
		Amount:               income.Amount.String(),
		ExchangeRate:         income.ExchangeRate.String(),
		Description:          sql.NullString{String: income.Description, Valid: true},
		DateReceived:         income.DateReceived,
	})
	if err != nil {
		return err
	}
	// set the created income
	income.ID = incomeDetails.ID
	income.UserID = userID
	income.CreatedAt = incomeDetails.CreatedAt.Time
	income.UpdatedAt = incomeDetails.UpdatedAt.Time
	// we are good
	return nil
}

// Populate the recurring expense
func populateRecurringExpense(recurringExpensRow interface{}) *RecurringExpense {
	switch recurringExpense := recurringExpensRow.(type) {
	case database.RecurringExpense:
		return &RecurringExpense{
			ID:                 recurringExpense.ID,
			UserID:             recurringExpense.UserID,
			BudgetID:           recurringExpense.BudgetID,
			Amount:             decimal.RequireFromString(recurringExpense.Amount),
			Name:               recurringExpense.Name,
			Description:        recurringExpense.Description.String,
			RecurrenceInterval: recurringExpense.RecurrenceInterval,
			NextOccurrence:     recurringExpense.NextOccurrence,
			ProjectedAmount:    decimal.RequireFromString(recurringExpense.ProjectedAmount),
			CreatedAt:          recurringExpense.CreatedAt.Time,
			UpdatedAt:          recurringExpense.UpdatedAt.Time,
		}
	default:
		return nil
	}
}

func populateExpense(expenseRow interface{}) *Expense {
	switch expense := expenseRow.(type) {
	case database.Expense:
		return &Expense{
			ID:           expense.ID,
			UserID:       expense.UserID,
			BudgetID:     expense.BudgetID,
			Name:         expense.Name,
			Category:     expense.Category,
			Amount:       decimal.RequireFromString(expense.Amount),
			IsRecurring:  expense.IsRecurring,
			Description:  expense.Description.String,
			DateOccurred: expense.DateOccurred,
			CreatedAt:    expense.CreatedAt.Time,
			UpdatedAt:    expense.UpdatedAt.Time,
		}
	default:
		return nil
	}
}
