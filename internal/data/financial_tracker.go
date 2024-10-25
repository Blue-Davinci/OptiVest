package data

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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

const (
	RedisFinTrackDebtSearchPrefix        = "fintrack_debt_search"
	RedisFinTrackDebtPaymentSearchPrefix = "fintrack_debt_payment_search"
)

var (
	DefaultFinTrackDBContextTimeout = 5 * time.Second
	DefaultFinTrackRedisDebtTTL     = 10 * time.Minute
)

var (
	ErrInvalidRecurringExpenseTime = errors.New("invalid recurring expense time")
	ErrDuplicateRecurringExpense   = errors.New("recurring expense already exists")
	ErrDuplicateDebt               = errors.New("debt with a similar description already exists")
	ErrInvalidRemainingBalance     = errors.New("remaining balance cannot be less than zero, please check your payment amount")
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

// DebtWithPayments represents a debt with its payments
type DebtWithPayments struct {
	Debt                   *Debt                  `json:"debt"`
	Payments               []*EnrichedDebtPayment `json:"payments"`
	PaymentMetadata        Metadata               `json:"payment_metadata"`
	TotalPaymentAmount     decimal.Decimal        `json:"total_payment_amount"`
	TotalInterestPayment   decimal.Decimal        `json:"total_interest_payment"`
	TotalRemainingBalances decimal.Decimal        `json:"total_principal_payment"`
}

// Represents a Debt
type Debt struct {
	ID                     int64           `json:"id"`
	UserID                 int64           `json:"user_id"`
	Name                   string          `json:"name"`
	Amount                 decimal.Decimal `json:"amount"`
	RemainingBalance       decimal.Decimal `json:"remaining_balance"`
	InterestRate           decimal.Decimal `json:"interest_rate"`
	Description            string          `json:"description,omitempty"`
	DueDate                time.Time       `json:"due_date"`
	MinimumPayment         decimal.Decimal `json:"minimum_payment"`
	CreatedAt              time.Time       `json:"created_at"`
	UpdatedAt              time.Time       `json:"updated_at"`
	NextPaymentDate        time.Time       `json:"next_payment_date"`
	EstimatedPayoffDate    time.Time       `json:"estimated_payoff_date,omitempty"`
	AccruedInterest        decimal.Decimal `json:"accrued_interest"`
	InterestLastCalculated time.Time       `json:"interest_last_calculated"`
	LastPaymentDate        time.Time       `json:"last_payment_date,omitempty"`
	TotalInterestPaid      decimal.Decimal `json:"total_interest_paid"`
}

// Enriched Debt Payment returns a debt payment with the debt details
type EnrichedDebtPayment struct {
	DebtPayment           *DebtRepayment  `json:"debt_payment"`
	TotalPaymentAmount    decimal.Decimal `json:"total_payment_amount"`
	TotalInterestPayment  decimal.Decimal `json:"total_interest_payment"`
	TotalPrincipalPayment decimal.Decimal `json:"total_principal_payment"`
}

// Represents a Debt repayment
type DebtRepayment struct {
	ID               int64           `json:"id"`
	DebtID           int64           `json:"debt_id"`
	UserID           int64           `json:"user_id"`
	PaymentAmount    decimal.Decimal `json:"payment_amount"`
	PaymentDate      time.Time       `json:"payment_date"`
	InterestPayment  decimal.Decimal `json:"interest_payment"`
	PrincipalPayment decimal.Decimal `json:"principal_payment"`
	CreatedAt        time.Time       `json:"created_at"`
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

// Validate Debt
func ValidateDebt(v *validator.Validator, debt *Debt) {
	ValidateAmount(v, debt.Amount, "amount")
	ValidateAmount(v, debt.MinimumPayment, "minimum_payment")
	ValidateBudgetDescription(v, debt.Description)
	ValidateName(v, debt.Name, "name")
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

// GetAllExpensesByUserID() gets all the expenses by a user ID
// This route supports pagination and a name search parameter for the
// expense's name.
// We return an array of expenses, metadata struct and an error if any was found
func (m *FinancialTrackingModel) GetAllExpensesByUserID(userID int64, expenseName string, filters Filters) ([]*Expense, Metadata, error) {
	// set our context
	ctx, cancel := contextGenerator(context.Background(), DefaultFinTrackDBContextTimeout)
	defer cancel()
	// get the expenses
	expenses, err := m.DB.GetAllExpensesByUserID(ctx, database.GetAllExpensesByUserIDParams{
		UserID:  userID,
		Column2: expenseName,
		Limit:   int32(filters.limit()),
		Offset:  int32(filters.offset()),
	})
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, Metadata{}, ErrGeneralRecordNotFound
		default:
			return nil, Metadata{}, err
		}
	}
	if len(expenses) == 0 {
		return nil, Metadata{}, ErrGeneralRecordNotFound
	}
	// set totals
	totalRecords := 0
	// populate the expenses
	var populatedExpenses []*Expense
	for _, expense := range expenses {
		totalRecords = int(expense.TotalCount)
		populatedExpenses = append(populatedExpenses, populateExpense(expense))
	}
	// calculate metadata
	metadata := calculateMetadata(totalRecords, filters.Page, filters.PageSize)
	// we are good
	return populatedExpenses, metadata, nil
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
	if len(recurringExpenses) == 0 {
		return nil, Metadata{}, ErrGeneralRecordNotFound
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

// UpdateIncome() Updates an existing income by th eincomes userID and ID
// We return an error if any and an updated Income
func (m *FinancialTrackingModel) UpdateIncomeByID(userID int64, income *Income) error {
	// set our context
	ctx, cancel := contextGenerator(context.Background(), DefaultFinTrackDBContextTimeout)
	defer cancel()
	// make the update
	updatedAT, err := m.DB.UpdateIncomeByID(ctx, database.UpdateIncomeByIDParams{
		Source:               income.Source,
		OriginalCurrencyCode: income.OriginalCurrencyCode,
		AmountOriginal:       income.AmountOriginal.String(),
		Amount:               income.AmountOriginal.String(),
		ExchangeRate:         income.ExchangeRate.String(),
		Description:          sql.NullString{String: income.Description, Valid: true},
		DateReceived:         income.DateReceived,
		ID:                   income.ID,
		UserID:               userID,
	})
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return ErrEditConflict
		default:
			return err
		}
	}
	// updated updated at field
	income.UpdatedAt = updatedAT.Time
	// we are good
	return nil
}

// GetIncomeByID() gets an income by its ID and user ID
// We return an error if any was found
func (m *FinancialTrackingModel) GetIncomeByID(userID, incomeID int64) (*Income, error) {
	// set our context
	ctx, cancel := contextGenerator(context.Background(), DefaultFinTrackDBContextTimeout)
	defer cancel()
	// get the income
	income, err := m.DB.GetIncomeByID(ctx, database.GetIncomeByIDParams{
		ID:     incomeID,
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
	// we are good, populate the income
	updatedIncome := populateIncome(income)
	// we are good
	return updatedIncome, nil
}

// =========================================================================================================
// Debts
// =========================================================================================================

// CreateNewDebt() creates a new debt in the debts table
// We get a user ID and a pointer to a debt struct
// We return an error if any was found
func (m *FinancialTrackingModel) CreateNewDebt(userID int64, debt *Debt) error {
	// set our context
	ctx, cancel := contextGenerator(context.Background(), DefaultFinTrackDBContextTimeout)
	defer cancel()
	// create the debt
	debtDetails, err := m.DB.CreateNewDebt(ctx, database.CreateNewDebtParams{
		UserID:                 userID,
		Name:                   debt.Name,
		Amount:                 debt.Amount.String(),
		RemainingBalance:       debt.RemainingBalance.String(),
		InterestRate:           sql.NullString{String: debt.InterestRate.String(), Valid: true},
		Description:            sql.NullString{String: debt.Description, Valid: true},
		DueDate:                debt.DueDate,
		MinimumPayment:         debt.MinimumPayment.String(),
		NextPaymentDate:        debt.NextPaymentDate,
		EstimatedPayoffDate:    sql.NullTime{Time: debt.EstimatedPayoffDate, Valid: true},
		AccruedInterest:        sql.NullString{String: debt.AccruedInterest.String(), Valid: true},
		InterestLastCalculated: sql.NullTime{Time: debt.InterestLastCalculated, Valid: true},
		TotalInterestPaid:      sql.NullString{String: debt.TotalInterestPaid.String(), Valid: true},
	})
	if err != nil {
		switch {
		case err.Error() == `pq: duplicate key value violates unique constraint "unique_debt_description_per_user`:
			return ErrDuplicateDebt
		default:
			return err
		}
	}
	// set the created debt
	debt.ID = debtDetails.ID
	debt.UserID = userID
	debt.CreatedAt = debtDetails.CreatedAt.Time
	debt.UpdatedAt = debtDetails.UpdatedAt.Time
	// we are good
	return nil
}

// UpdateDebtByID() updates a debt WHEN given both the ID and the user ID
// We return an error if any was found
func (m *FinancialTrackingModel) UpdateDebtByID(userID int64, debt *Debt) error {
	// set our context
	ctx, cancel := contextGenerator(context.Background(), DefaultFinTrackDBContextTimeout)
	defer cancel()
	// create the debt
	updatedAt, err := m.DB.UpdateDebtByID(ctx, database.UpdateDebtByIDParams{
		UserID:                 userID,
		Name:                   debt.Name,
		Amount:                 debt.Amount.String(),
		RemainingBalance:       debt.RemainingBalance.String(),
		InterestRate:           sql.NullString{String: debt.InterestRate.String(), Valid: true},
		Description:            sql.NullString{String: debt.Description, Valid: true},
		DueDate:                debt.DueDate,
		MinimumPayment:         debt.MinimumPayment.String(),
		NextPaymentDate:        debt.NextPaymentDate,
		EstimatedPayoffDate:    sql.NullTime{Time: debt.EstimatedPayoffDate, Valid: true},
		AccruedInterest:        sql.NullString{String: debt.AccruedInterest.String(), Valid: true},
		InterestLastCalculated: sql.NullTime{Time: debt.InterestLastCalculated, Valid: true},
		TotalInterestPaid:      sql.NullString{String: debt.TotalInterestPaid.String(), Valid: true},
		LastPaymentDate:        sql.NullTime{Time: debt.LastPaymentDate, Valid: !debt.LastPaymentDate.IsZero()},
		ID:                     debt.ID,
	})
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return ErrEditConflict
		case err.Error() == `pq: new row for relation "debts" violates check constraint "debts_remaining_balance_check"`:
			return ErrInvalidRemainingBalance
		default:
			return err
		}
	}
	// set the updated debt
	debt.UpdatedAt = updatedAt.Time
	// we are good
	return nil
}

// GetDebtByID() gets a debt by its ID and user ID
// We return an error if any was found
func (m *FinancialTrackingModel) GetDebtByID(userID, debtID int64) (*Debt, error) {
	// set our context
	ctx, cancel := contextGenerator(context.Background(), DefaultFinTrackDBContextTimeout)
	defer cancel()
	// get the debt
	debt, err := m.DB.GetDebtByID(ctx, debtID)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrGeneralRecordNotFound
		default:
			return nil, err
		}
	}
	// we are good, populate the debt
	updatedDebt := populateDebt(debt)
	// we are good
	return updatedDebt, nil
}

// GetAllDebtsByUserID() gets all the debts by a user ID
// This will will support both pagination and a name search parameter.
// We return an array of []*DebtWithPayments, passing each debt id to GetDebtPaymentsByDebtUserID() to get the payments
// a metadata struct and an error if any was found
func (m *FinancialTrackingModel) GetAllDebtsByUserID(userID int64, debtName string, filters Filters) ([]*DebtWithPayments, Metadata, error) {
	// set our context
	ctx, cancel := contextGenerator(context.Background(), DefaultFinTrackDBContextTimeout)
	defer cancel()
	// get the debts
	debts, err := m.DB.GetAllDebtsByUserID(ctx, database.GetAllDebtsByUserIDParams{
		UserID:  userID,
		Column2: debtName,
		Limit:   int32(filters.limit()),
		Offset:  int32(filters.offset()),
	})
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, Metadata{}, ErrGeneralRecordNotFound
		default:
			return nil, Metadata{}, err
		}
	}
	if len(debts) == 0 {
		return nil, Metadata{}, ErrGeneralRecordNotFound
	}
	// set totals
	totalRecords := 0
	// populate the debts
	var populatedDebts []*DebtWithPayments
	for _, debt := range debts {
		totalRecords = int(debt.TotalDebts)
		// get the payments
		payments, metadata, err := m.GetDebtPaymentsByDebtUserID(
			userID,
			debt.ID,
			time.Time{}, // use the earliest time as the start date
			time.Now(),  // use today as the end date
			filters)
		if err != nil {
			fmt.Println("Error getting debt payments", err)
		}
		populatedDebts = append(populatedDebts, &DebtWithPayments{
			Debt:                   populateDebt(debt),
			Payments:               payments,
			PaymentMetadata:        metadata,
			TotalPaymentAmount:     decimal.RequireFromString(debt.TotalAmounts),
			TotalInterestPayment:   decimal.RequireFromString(debt.TotalInterestPaid.String),
			TotalRemainingBalances: decimal.RequireFromString(debt.TotalRemainingBalances),
		})
	}
	// calculate metadata
	metadata := calculateMetadata(totalRecords, filters.Page, filters.PageSize)
	// we are good
	return populatedDebts, metadata, nil
}

// GetDebtPaymentsByDebtUserID() gets all the debt payments by a debt ID and user ID
// This route supports both pagination as well as date search parameters (start and end date)
// We return an []*EnrichedDebtPayment, a metadata struct and an error if any was found
func (m *FinancialTrackingModel) GetDebtPaymentsByDebtUserID(userID, debtID int64, startDate, endDate time.Time, filters Filters) ([]*EnrichedDebtPayment, Metadata, error) {
	// set our context
	ctx, cancel := contextGenerator(context.Background(), DefaultFinTrackDBContextTimeout)
	defer cancel()
	// get the debt payments
	debtPayments, err := m.DB.GetDebtPaymentsByDebtUserID(ctx, database.GetDebtPaymentsByDebtUserIDParams{
		DebtID:  debtID,
		UserID:  userID,
		Column3: startDate,
		Column4: endDate,
		Limit:   int32(filters.limit()),
		Offset:  int32(filters.offset()),
	})
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, Metadata{}, ErrGeneralRecordNotFound
		default:
			return nil, Metadata{}, err
		}
	}
	if len(debtPayments) == 0 {
		return nil, Metadata{}, ErrGeneralRecordNotFound
	}
	// set totals
	totalRecords := 0
	// populate the debt payments
	var populatedDebtPayments []*EnrichedDebtPayment
	for _, debtPayment := range debtPayments {
		totalRecords = int(debtPayment.TotalPayments)
		populatedDebtPayments = append(populatedDebtPayments,
			&EnrichedDebtPayment{
				DebtPayment:           populateEnrichedDebtPayment(debtPayment),
				TotalPaymentAmount:    decimal.RequireFromString(debtPayment.TotalPaymentAmount),
				TotalInterestPayment:  decimal.RequireFromString(debtPayment.TotalInterestPayment),
				TotalPrincipalPayment: decimal.RequireFromString(debtPayment.TotalPrincipalPayment),
			})
	}
	// calculate metadata
	metadata := calculateMetadata(totalRecords, filters.Page, filters.PageSize)
	// we are good
	return populatedDebtPayments, metadata, nil
}

// CreateNewDebtPayment() creates a new debt payment in the debt payments table
// We return an error if any was found
func (m *FinancialTrackingModel) CreateNewDebtPayment(userID int64, debtRepayment *DebtRepayment) error {
	// set our context
	ctx, cancel := contextGenerator(context.Background(), DefaultFinTrackDBContextTimeout)
	defer cancel()
	// create the debt payment
	debtPayment, err := m.DB.CreateNewDebtPayment(ctx, database.CreateNewDebtPaymentParams{
		DebtID:           debtRepayment.DebtID,
		UserID:           userID,
		PaymentAmount:    debtRepayment.PaymentAmount.String(),
		PaymentDate:      debtRepayment.PaymentDate,
		InterestPayment:  debtRepayment.InterestPayment.String(),
		PrincipalPayment: debtRepayment.PrincipalPayment.String(),
	})
	if err != nil {
		return err
	}
	// set the debt payment
	debtRepayment.ID = debtPayment.ID
	debtRepayment.CreatedAt = debtPayment.CreatedAt.Time
	// we are good
	return nil
}

// GetAllOverdueDebts() gets all the debts that are overdue
// This is meant to be used in tandem with a cron job
// We also return a Metadata struct and an error if any was found
func (m *FinancialTrackingModel) GetAllOverdueDebts(filters Filters) ([]*Debt, Metadata, error) {
	// set our context
	ctx, cancel := contextGenerator(context.Background(), DefaultFinTrackDBContextTimeout)
	defer cancel()
	// get the debts
	debts, err := m.DB.GetAllOverdueDebts(ctx, database.GetAllOverdueDebtsParams{
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
	if len(debts) == 0 {
		return nil, Metadata{}, ErrGeneralRecordNotFound
	}
	// set totals
	totalRecords := 0
	// populate the debts
	var populatedDebts []*Debt
	for _, debt := range debts {
		totalRecords = int(debt.TotalCount)
		populatedDebts = append(populatedDebts, populateDebt(debt))
	}

	fmt.Println("Length of populated debts", len(populatedDebts))
	fmt.Printf("Populated debts: %v\n", populatedDebts)
	// calculate metadata
	metadata := calculateMetadata(totalRecords, filters.Page, filters.PageSize)
	fmt.Println("Id of first debt", populatedDebts[0].ID)
	// we are good
	return populatedDebts, metadata, nil
}

// populateDebtPayment() populates a debt payment
func populateEnrichedDebtPayment(debtPaymentRow interface{}) *DebtRepayment {
	switch debtPayment := debtPaymentRow.(type) {
	case database.GetDebtPaymentsByDebtUserIDRow:
		return &DebtRepayment{
			ID:               debtPayment.ID,
			DebtID:           debtPayment.DebtID,
			UserID:           debtPayment.UserID,
			PaymentAmount:    decimal.RequireFromString(debtPayment.PaymentAmount),
			PaymentDate:      debtPayment.PaymentDate,
			InterestPayment:  decimal.RequireFromString(debtPayment.InterestPayment),
			PrincipalPayment: decimal.RequireFromString(debtPayment.PrincipalPayment),
			CreatedAt:        debtPayment.CreatedAt.Time,
		}
	default:
		return nil
	}
}

// PopulateDebt populates a debt
func populateDebt(debtRow interface{}) *Debt {
	switch debt := debtRow.(type) {
	case database.Debt:
		return &Debt{
			ID:                     debt.ID,
			UserID:                 debt.UserID,
			Name:                   debt.Name,
			Amount:                 decimal.RequireFromString(debt.Amount),
			RemainingBalance:       decimal.RequireFromString(debt.RemainingBalance),
			InterestRate:           decimal.RequireFromString(debt.InterestRate.String),
			Description:            debt.Description.String,
			DueDate:                debt.DueDate,
			MinimumPayment:         decimal.RequireFromString(debt.MinimumPayment),
			CreatedAt:              debt.CreatedAt.Time,
			UpdatedAt:              debt.UpdatedAt.Time,
			NextPaymentDate:        debt.NextPaymentDate,
			EstimatedPayoffDate:    debt.EstimatedPayoffDate.Time,
			AccruedInterest:        decimal.RequireFromString(debt.AccruedInterest.String),
			InterestLastCalculated: debt.InterestLastCalculated.Time,
			TotalInterestPaid:      decimal.RequireFromString(debt.TotalInterestPaid.String),
		}
	case database.GetAllOverdueDebtsRow:
		return &Debt{
			ID:                     debt.ID,
			UserID:                 debt.UserID,
			Name:                   debt.Name,
			Amount:                 decimal.RequireFromString(debt.Amount),
			RemainingBalance:       decimal.RequireFromString(debt.RemainingBalance),
			InterestRate:           decimal.RequireFromString(debt.InterestRate.String),
			Description:            debt.Description.String,
			DueDate:                debt.DueDate,
			MinimumPayment:         decimal.RequireFromString(debt.MinimumPayment),
			CreatedAt:              debt.CreatedAt.Time,
			UpdatedAt:              debt.UpdatedAt.Time,
			NextPaymentDate:        debt.NextPaymentDate,
			EstimatedPayoffDate:    debt.EstimatedPayoffDate.Time,
			AccruedInterest:        decimal.RequireFromString(debt.AccruedInterest.String),
			InterestLastCalculated: debt.InterestLastCalculated.Time,
			TotalInterestPaid:      decimal.RequireFromString(debt.TotalInterestPaid.String),
		}
	case database.GetAllDebtsByUserIDRow:
		return &Debt{
			ID:                     debt.ID,
			UserID:                 debt.UserID,
			Name:                   debt.Name,
			Amount:                 decimal.RequireFromString(debt.Amount),
			RemainingBalance:       decimal.RequireFromString(debt.RemainingBalance),
			InterestRate:           decimal.RequireFromString(debt.InterestRate.String),
			Description:            debt.Description.String,
			DueDate:                debt.DueDate,
			MinimumPayment:         decimal.RequireFromString(debt.MinimumPayment),
			CreatedAt:              debt.CreatedAt.Time,
			UpdatedAt:              debt.UpdatedAt.Time,
			NextPaymentDate:        debt.NextPaymentDate,
			EstimatedPayoffDate:    debt.EstimatedPayoffDate.Time,
			AccruedInterest:        decimal.RequireFromString(debt.AccruedInterest.String),
			InterestLastCalculated: debt.InterestLastCalculated.Time,
			TotalInterestPaid:      decimal.RequireFromString(debt.TotalInterestPaid.String),
		}

	default:
		return nil
	}
}

// Populate Income
func populateIncome(incomeRow interface{}) *Income {
	switch income := incomeRow.(type) {
	case database.Income:
		return &Income{
			ID:                   income.ID,
			UserID:               income.UserID,
			Source:               income.Source,
			OriginalCurrencyCode: income.OriginalCurrencyCode,
			AmountOriginal:       decimal.RequireFromString(income.AmountOriginal),
			Amount:               decimal.RequireFromString(income.Amount),
			ExchangeRate:         decimal.RequireFromString(income.ExchangeRate),
			Description:          income.Description.String,
			DateReceived:         income.DateReceived,
			CreatedAt:            income.CreatedAt.Time,
			UpdatedAt:            income.UpdatedAt.Time,
		}
	default:
		return nil
	}
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
	case database.GetAllRecurringExpensesDueForProcessingRow:
		return &RecurringExpense{
			ID:                 recurringExpense.ID,
			UserID:             recurringExpense.UserID,
			BudgetID:           recurringExpense.BudgetID,
			Amount:             decimal.RequireFromString(recurringExpense.Amount),
			Name:               recurringExpense.Name,
			Description:        recurringExpense.Description.String,
			RecurrenceInterval: recurringExpense.RecurrenceInterval,
			ProjectedAmount:    decimal.RequireFromString(recurringExpense.ProjectedAmount),
			NextOccurrence:     recurringExpense.NextOccurrence,
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
	case database.GetAllExpensesByUserIDRow:
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
