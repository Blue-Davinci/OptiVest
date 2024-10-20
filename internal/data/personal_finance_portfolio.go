package data

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Blue-Davinci/OptiVest/internal/database"
	"github.com/Blue-Davinci/OptiVest/internal/validator"
	"github.com/shopspring/decimal"
)

type PersonalFinancePortfolioModel struct {
	DB *database.Queries
}

const (
	DefaultPerFinPortDBContextTimeout   = 5 * time.Second
	DefaultRedisOCRTTL                  = 24 * time.Hour
	DefaultRedisExpenseIncomeSummaryTTL = 12 * time.Hour
)
const (
	// redis prefix
	RedisOCRRedeiptPrefix           = "ocr_receipt_analysis:"
	RedisExpenseIncomeSummaryPrefix = "expense_income_summary:"
)
const (
	DataUserHasEnoughPredictionDataPerMonth = "sufficient_data_monthly"
	DataUserHasEnoughPredictionDataPerWeek  = "sufficient_data_weekly"
	DataUserInsufficientPredictionData      = "insufficient_data"
)

// Struct for parsed OCR result
type OCRResponse struct {
	ParsedResults []struct {
		ParsedText string `json:"ParsedText"`
	} `json:"ParsedResults"`
}

// UnifiedFinanceAnalysis is a struct that contains all the finance analysis data
type UnifiedFinanceAnalysis struct {
	GoalAnalysis             TotalGoalAnalysis             `json:"goal_analysis"`
	IncomeAnalysis           TotalIncomeAnalysis           `json:"income_analysis"`
	ExpenseAnalysis          TotalExpenseAnalysis          `json:"expense_analysis"`
	BudgetAnalysis           TotalBudgetAnalysis           `json:"budget_analysis"`
	DebtAnalysis             TotalDebtAnalysis             `json:"debt_analysis"`
	RecurringExpenseAnalysis TotalRecurringExpenseAnalysis `json:"recurring_expense_analysis"`
}

// IncomeAnalysis is a struct that represents an income source
type IncomeAnalysis struct {
	IncomeSource string          `json:"income_source"`
	Amount       decimal.Decimal `json:"amount"`
	DateReceived string          `json:"date_received"` // You could use time.Time if necessary
}

// TotalIncomeAnalysis is a struct that contains all the income analysis data
type TotalIncomeAnalysis struct {
	Type        string           `json:"type"`         // Always "income"
	Details     []IncomeAnalysis `json:"details"`      // List of income details
	TotalAmount decimal.Decimal  `json:"total_amount"` // Total income sum
}

// ExpenseAnalysis is a struct that represents an expense
type ExpenseAnalysis struct {
	ExpenseName string          `json:"expense_name"`
	Category    string          `json:"category"`
	Amount      decimal.Decimal `json:"amount"`
	IsRecurring bool            `json:"is_recurring"`
	BudgetName  string          `json:"budget_name"`
}

// TotalExpenseAnalysis is a struct that contains all the expense analysis data
type TotalExpenseAnalysis struct {
	Type        string            `json:"type"`         // Always "expense"
	Details     []ExpenseAnalysis `json:"details"`      // List of expense details
	TotalAmount decimal.Decimal   `json:"total_amount"` // Total expense sum
}

// RecurringExpenseAnalysis is a struct that represents a recurring expense
type RecurringExpenseAnalysis struct {
	ExpenseName                 string          `json:"expense_name"`
	Amount                      decimal.Decimal `json:"amount"`
	TotalMonthlyProjectedAmount decimal.Decimal `json:"projected_monthly_amount"`
	RecurrenceInterval          string          `json:"recurrence_interval"`
	BudgetName                  string          `json:"budget_name"`
}

type TotalRecurringExpenseAnalysis struct {
	Type        string                     `json:"type"`         // Always "recurring_expense"
	Details     []RecurringExpenseAnalysis `json:"details"`      // List of recurring expense details
	TotalAmount decimal.Decimal            `json:"total_amount"` // Total recurring expense sum
}

// Budget is a struct that represents a budget
type BudgetAnalysis struct {
	BudgetName  string          `json:"budget_name"`
	Category    string          `json:"category"`
	TotalAmount decimal.Decimal `json:"total_amount"`
}

// TotalBudgetAnalysis is a struct that contains all the budget analysis data
type TotalBudgetAnalysis struct {
	Type        string           `json:"type"`         // Always "budget"
	Details     []BudgetAnalysis `json:"details"`      // List of budget details
	TotalAmount decimal.Decimal  `json:"total_amount"` // This will always be 0
}

// Debt is a struct that represents a debt
type DebtAnalysis struct {
	DebtName         string          `json:"debt_name"`
	DueDate          string          `json:"due_date"` // You could use time.Time if necessary
	InterestRate     decimal.Decimal `json:"interest_rate"`
	RemainingBalance decimal.Decimal `json:"remaining_balance"`
}

// TotalDebtAnalysis is a struct that contains all the debt analysis data
type TotalDebtAnalysis struct {
	Type        string          `json:"type"`         // Always "debt"
	Details     []DebtAnalysis  `json:"details"`      // List of debt details
	TotalAmount decimal.Decimal `json:"total_amount"` // Total remaining balance sum
}

// GoalAnalysis is a struct that represents a goal
type GoalAnalysis struct {
	GoalName   string          `json:"goal_name"`
	Amount     decimal.Decimal `json:"amount"`
	TargetDate CustomTime1     `json:"target_date"` // You could use time.Time if necessary
	BudgetName string          `json:"budget_name"`
}

// TotalGoalAnalysis is a struct that contains all the goal analysis data
type TotalGoalAnalysis struct {
	Type        string          `json:"type"`         // Always "goal"
	Details     []GoalAnalysis  `json:"details"`      // List of goal details
	TotalAmount decimal.Decimal `json:"total_amount"` // Total goal sum
}

// PredictionPersonalFinanceData is a struct that represents a personal finance data for prediction
type PredictionPersonalFinanceData struct {
	Type        string          `json:"type"`
	PeriodStart time.Time       `json:"period_start"`
	TotalAmount decimal.Decimal `json:"total_amount"`
}

// ExpensesIncomesMonthlySummary is a struct that represents the expenses and incomes summary per month
type ExpensesIncomesMonthlySummary struct {
	Month         string          `json:"month"`
	TotalIncome   decimal.Decimal `json:"total_income"`
	TotalExpenses decimal.Decimal `json:"total_expenses"`
}

// Prediction is a struct that represents a prediction
type Prediction struct {
	Ds               CustomTime1 `json:"ds"`
	Yhat             float64     `json:"yhat"`
	YhatLower        float64     `json:"yhat_lower"`
	YhatUpper        float64     `json:"yhat_upper"`
	GoalMet          string      `json:"goal_met,omitempty"`
	SurplusOrDeficit float64     `json:"surplus_or_deficit,omitempty"`
}
type DateOnly struct {
	time.Time
}

// MarshalJSON to format time.Time as "YYYY-MM-DD"
func (d DateOnly) MarshalJSON() ([]byte, error) {
	formattedDate := fmt.Sprintf("\"%s\"", d.Format("2006-01-02"))
	return []byte(formattedDate), nil
}

// UnmarshalJSON to parse both "YYYY-MM-DD" and "YYYY-MM-DDTHH:MM:SSZ" formats
func (d *DateOnly) UnmarshalJSON(data []byte) error {
	strDate := strings.Trim(string(data), "\"")

	// Try to parse as "2006-01-02"
	parsedDate, err := time.Parse("2006-01-02", strDate)
	if err == nil {
		*d = DateOnly{parsedDate}
		return nil
	}

	// Try to parse as "2006-01-02T15:04:05Z07:00"
	parsedDate, err = time.Parse(time.RFC3339, strDate)
	if err == nil {
		*d = DateOnly{parsedDate}
		return nil
	}

	return fmt.Errorf("unable to parse date: %v", strDate)
}

type PersonalFinancePredictionResponse struct {
	ExpensePredictions []Prediction `json:"expense_predictions"`
	IncomePredictions  []Prediction `json:"income_predictions"`
	SavingsPredictions []Prediction `json:"savings_predictions"`
}

type PersonalFinancePredictionRequest struct {
	Expenses            []float64   `json:"expenses"`
	ExpensesStartDate   string      `json:"expenses_start_date"`
	Incomes             []float64   `json:"incomes"`
	IncomesStartDate    string      `json:"incomes_start_date"`
	Savings             SavingsData `json:"savings"`
	SavingsStartDate    string      `json:"savings_start_date"`
	Frequency           string      `json:"frequency"`
	Country             string      `json:"country"`
	PredictionPeriod    int         `json:"prediction_period"`
	EnableTaxDeductions bool        `json:"enable_tax_deductions"`
	TaxRate             float64     `json:"tax_rate"`
	EnableSeasonality   bool        `json:"enable_seasonality"`
	EnableHolidays      bool        `json:"enable_holidays"`
}

type SavingsData struct {
	CurrentSavings      float64 `json:"current_savings"`
	MonthlyContribution float64 `json:"monthly_contribution"`
	Goal                float64 `json:"goal"`
}

// ValidatePredictionParameters() validates the prediction parameters
func ValidatePredictionParameters(v *validator.Validator, timeline string) {
	v.Check(timeline == "monthly" || timeline == "weekly", "timeline", "must be either 'monthly' or 'weekly'")
}

// GetAllFinanceDetailsForAnalysisByUserID() returns all the finance details for a user
// The data is returned in JSON format and includes income, expenses, budgets, and debts.
// We will need to unmarshal this data into the appropriate structs in the frontend based
// on the "type" field in the JSON.
// We return a UnifiedFinanceAnalysis struct that contains all the finance analysis data and
// an error if the operation fails.
func (m PersonalFinancePortfolioModel) GetAllFinanceDetailsForAnalysisByUserID(userID int64) (*UnifiedFinanceAnalysis, error) {
	ctx, cancel := contextGenerator(context.Background(), DefaultPerFinPortDBContextTimeout)
	defer cancel()

	// Get the personal finance rows for the user
	personalFinanceRows, err := m.DB.GetAllFinanceDetailsForAnalysisByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrGeneralRecordNotFound
		}
		return nil, err
	}

	if len(personalFinanceRows) == 0 {
		return nil, ErrGeneralRecordNotFound
	}

	// Initialize the total analysis structs
	totalIncomeAnalysis := TotalIncomeAnalysis{}
	totalExpenseAnalysis := TotalExpenseAnalysis{}
	totalRecurringExpenseAnalysis := TotalRecurringExpenseAnalysis{}
	totalBudgetAnalysis := TotalBudgetAnalysis{}
	totalDebtAnalysis := TotalDebtAnalysis{}
	totalGoalAnalysis := TotalGoalAnalysis{}

	// Populate each finance type using the helper function
	for _, personalFinanceRow := range personalFinanceRows {
		err := populatePersonalFinancePortfolio(personalFinanceRow,
			&totalIncomeAnalysis,
			&totalExpenseAnalysis,
			&totalBudgetAnalysis,
			&totalDebtAnalysis,
			&totalGoalAnalysis,
			&totalRecurringExpenseAnalysis)
		if err != nil {
			return nil, err
		}
	}

	// Create a unified analysis struct and return it
	unifiedFinanceAnalysis := &UnifiedFinanceAnalysis{
		IncomeAnalysis:           totalIncomeAnalysis,
		ExpenseAnalysis:          totalExpenseAnalysis,
		RecurringExpenseAnalysis: totalRecurringExpenseAnalysis,
		BudgetAnalysis:           totalBudgetAnalysis,
		DebtAnalysis:             totalDebtAnalysis,
		GoalAnalysis:             totalGoalAnalysis,
	}

	return unifiedFinanceAnalysis, nil
}

// CheckIfUserHasEnoughPredictionData() checks if the user has enough prediction data
// to make a prediction. We will send a constant that will alert the caller whether to use
// per week in the call to the micro service or per month. We will return a string constant
// and an error if the operation fails.
func (m PersonalFinancePortfolioModel) CheckIfUserHasEnoughPredictionData(userID int64, timeline string, dateOccurred time.Time) (string, error) {
	ctx, cancel := contextGenerator(context.Background(), DefaultPerFinPortDBContextTimeout)
	defer cancel()

	// Get the total number of finance rows for the user
	totalFinanceRows, err := m.DB.CheckIfUserHasEnoughPredictionData(ctx, database.CheckIfUserHasEnoughPredictionDataParams{
		UserID:       userID,
		Column2:      timeline,
		DateOccurred: dateOccurred,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return DataUserInsufficientPredictionData, nil
		}
		return "", err
	}
	// Map returned totalFinanceRows string returning the const
	if totalFinanceRows == "sufficient_data_monthly" {
		return DataUserHasEnoughPredictionDataPerMonth, nil
	} else if totalFinanceRows == "sufficient_data_weekly" {
		return DataUserHasEnoughPredictionDataPerWeek, nil
	} else {
		return DataUserInsufficientPredictionData, nil
	}
}

// GetPersonalFinanceDataForMonthByUserID() returns all the finance details for a user from
// a given start date to today. We take in the user id and the start date and return a
// A PredictionPersonalFinanceData struct that contains all the finance analysis data and
// an error if the operation fails.
func (m PersonalFinancePortfolioModel) GetPersonalFinanceDataForMonthByUserID(userID int64, startDate time.Time) ([]*PredictionPersonalFinanceData, error) {
	ctx, cancel := contextGenerator(context.Background(), DefaultPerFinPortDBContextTimeout)
	defer cancel()

	// Get the personal finance rows for the user
	personalFinanceRows, err := m.DB.GetPersonalFinanceDataForMonthByUserID(ctx, database.GetPersonalFinanceDataForMonthByUserIDParams{
		UserID:       userID,
		DateReceived: startDate,
	})
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrGeneralRecordNotFound
		default:
			return nil, err
		}
	}

	if len(personalFinanceRows) == 0 {
		return nil, ErrGeneralRecordNotFound
	}
	predictionPersonalFinanceData := []*PredictionPersonalFinanceData{}

	for _, personalFinanceRow := range personalFinanceRows {
		totalAmount, err := decimal.NewFromString(personalFinanceRow.TotalAmount)
		if err != nil {
			return nil, err
		}
		predictionPersonalFinanceData = append(predictionPersonalFinanceData, &PredictionPersonalFinanceData{
			Type:        personalFinanceRow.Type,
			PeriodStart: personalFinanceRow.PeriodStart,
			TotalAmount: totalAmount,
		})
	}

	return predictionPersonalFinanceData, nil
}

// GetPersonalFinanceDataForWeeklyByUserID() returns all the finance details for a user from
// a given start date to today. We take in the user id and the start date and return a
// A PredictionPersonalFinanceData struct that contains all the finance analysis data and
// an error if the operation fails.
// This is different from the monthly version because it returns data for a week instead of a month.
func (m PersonalFinancePortfolioModel) GetPersonalFinanceDataForWeeklyByUserID(userID int64, startDate time.Time) ([]*PredictionPersonalFinanceData, error) {
	ctx, cancel := contextGenerator(context.Background(), DefaultPerFinPortDBContextTimeout)
	defer cancel()

	// Get the personal finance rows for the user
	personalFinanceRows, err := m.DB.GetPersonalFinanceDataForWeeklyByUserID(ctx, database.GetPersonalFinanceDataForWeeklyByUserIDParams{
		UserID:       userID,
		DateReceived: startDate,
	})
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrGeneralRecordNotFound
		default:
			return nil, err
		}
	}

	if len(personalFinanceRows) == 0 {
		return nil, ErrGeneralRecordNotFound
	}
	predictionPersonalFinanceData := []*PredictionPersonalFinanceData{}

	for _, personalFinanceRow := range personalFinanceRows {
		totalAmount, err := decimal.NewFromString(personalFinanceRow.TotalAmount)
		if err != nil {
			return nil, err
		}
		predictionPersonalFinanceData = append(predictionPersonalFinanceData, &PredictionPersonalFinanceData{
			Type:        personalFinanceRow.Type,
			PeriodStart: personalFinanceRow.PeriodStart,
			TotalAmount: totalAmount,
		})
	}

	return predictionPersonalFinanceData, nil
}

// ProcessPersonalFinanceData() processes the personal finance data and returns a structured
// request body with float64 values. We take in a slice of PredictionPersonalFinanceData and
// return a PersonalFinancePredictionRequest struct and an error if the operation fails.
func (m PersonalFinancePortfolioModel) ProcessPersonalFinanceData(predictionData []*PredictionPersonalFinanceData,
	frequency string, country string, predictionPeriod int, enableTaxDeductions bool, taxRate float64,
	enableSeasonality bool, enableHolidays bool) (*PersonalFinancePredictionRequest, error) {
	// Initialize slices to hold expenses and incomes as float64
	var expenses []float64
	var incomes []float64
	var savings SavingsData

	var expensesStartDate, incomesStartDate, savingsStartDate time.Time

	// Track if start dates are set
	expensesDateSet, incomesDateSet, savingsDateSet := false, false, false

	// Process each prediction data entry
	for _, data := range predictionData {
		switch data.Type {
		case "expense":
			// Convert decimal.Decimal to float64 and ignore `exact`
			expenseAmount, _ := data.TotalAmount.Float64()
			expenses = append(expenses, expenseAmount)
			if !expensesDateSet || data.PeriodStart.Before(expensesStartDate) {
				expensesStartDate = data.PeriodStart
				expensesDateSet = true
			}
		case "income":
			// Convert decimal.Decimal to float64
			incomeAmount, _ := data.TotalAmount.Float64()
			incomes = append(incomes, incomeAmount)
			if !incomesDateSet || data.PeriodStart.Before(incomesStartDate) {
				incomesStartDate = data.PeriodStart
				incomesDateSet = true
			}
		case "goal":
			// Convert decimal.Decimal to float64 for savings contribution
			monthlyContribution, _ := data.TotalAmount.Float64()
			savings.MonthlyContribution = monthlyContribution
			if !savingsDateSet || data.PeriodStart.Before(savingsStartDate) {
				savingsStartDate = data.PeriodStart
				savingsDateSet = true
			}
		}
	}

	// Helper function to remove timezone from a time.Time value
	removeTimezone := func(t time.Time) string {
		return t.Format("2006-01-02")
	}

	// Return the structured request body with float64 values
	return &PersonalFinancePredictionRequest{
		Expenses:            expenses,
		ExpensesStartDate:   removeTimezone(expensesStartDate),
		Incomes:             incomes,
		IncomesStartDate:    removeTimezone(incomesStartDate),
		Savings:             savings,
		SavingsStartDate:    removeTimezone(savingsStartDate),
		Frequency:           frequency,
		Country:             country,
		PredictionPeriod:    predictionPeriod,
		EnableTaxDeductions: enableTaxDeductions,
		TaxRate:             taxRate,
		EnableSeasonality:   enableSeasonality,
		EnableHolidays:      enableHolidays,
	}, nil
}

// GetExpenseIncomeSummaryReport() returns a summary report of the expenses and incomes
// It returns all expenses and incomes per month for a specific user
func (m PersonalFinancePortfolioModel) GetExpenseIncomeSummaryReport(userID int64) ([]*ExpensesIncomesMonthlySummary, error) {
	ctx, cancel := contextGenerator(context.Background(), DefaultPerFinPortDBContextTimeout)
	defer cancel()

	// Get the personal finance rows for the user
	personalFinanceRows, err := m.DB.GetExpenseIncomeSummaryReport(ctx, userID)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrGeneralRecordNotFound
		default:
			return nil, err
		}
	}

	if len(personalFinanceRows) == 0 {
		return nil, ErrGeneralRecordNotFound
	}
	// Initialize the slice to hold the summaries
	expenseIncomeSummaries := []*ExpensesIncomesMonthlySummary{}

	for _, personalFinanceRow := range personalFinanceRows {
		// Convert decimal.Decimal to float64
		monthStr, err := convertNumberToMonth(decimal.RequireFromString(personalFinanceRow.MonthValue))
		if err != nil {
			monthStr = "January"
			fmt.Println("An error occurred while converting the month number to a string:", err)
		}
		expenseIncomeSummary := &ExpensesIncomesMonthlySummary{
			Month:         monthStr,
			TotalIncome:   decimal.RequireFromString(personalFinanceRow.TotalIncome),
			TotalExpenses: decimal.RequireFromString(personalFinanceRow.TotalExpenses),
		}
		expenseIncomeSummaries = append(expenseIncomeSummaries, expenseIncomeSummary)
	}

	//  return it
	return expenseIncomeSummaries, nil
}

// populatePersonalFinancePortfolio() is a helper function that populates the personal finance portfolio
func populatePersonalFinancePortfolio(
	personalFinanceRow database.GetAllFinanceDetailsForAnalysisByUserIDRow,
	totalIncomeAnalysis *TotalIncomeAnalysis,
	totalExpenseAnalysis *TotalExpenseAnalysis,
	totalBudgetAnalysis *TotalBudgetAnalysis,
	totalDebtAnalysis *TotalDebtAnalysis,
	totalGoalAnalysis *TotalGoalAnalysis,
	totalRecurringExpenseAnalysis *TotalRecurringExpenseAnalysis) error {

	// Determine the type and unmarshal accordingly
	switch personalFinanceRow.Type {
	case "income":
		// Unmarshal to a slice of IncomeAnalysis
		var incomeDetails []IncomeAnalysis
		err := json.Unmarshal(personalFinanceRow.Details, &incomeDetails)
		if err != nil {
			return err
		}
		totalIncomeAnalysis.Details = append(totalIncomeAnalysis.Details, incomeDetails...)
		for _, income := range incomeDetails {
			totalIncomeAnalysis.TotalAmount = totalIncomeAnalysis.TotalAmount.Add(income.Amount)
		}
		totalIncomeAnalysis.Type = "income"

	case "expense":
		// Unmarshal to a slice of ExpenseAnalysis
		var expenseDetails []ExpenseAnalysis
		err := json.Unmarshal(personalFinanceRow.Details, &expenseDetails)
		if err != nil {
			return err
		}
		totalExpenseAnalysis.Details = append(totalExpenseAnalysis.Details, expenseDetails...)
		for _, expense := range expenseDetails {
			totalExpenseAnalysis.TotalAmount = totalExpenseAnalysis.TotalAmount.Add(expense.Amount)
		}
		totalExpenseAnalysis.Type = "expense"

	case "recurring_expense":
		// Unmarshal to a slice of RecurringExpenseAnalysis
		var recurringExpenseDetails []RecurringExpenseAnalysis
		err := json.Unmarshal(personalFinanceRow.Details, &recurringExpenseDetails)
		if err != nil {
			return err
		}
		totalRecurringExpenseAnalysis.Details = append(totalRecurringExpenseAnalysis.Details, recurringExpenseDetails...)
		for _, recurringExpense := range recurringExpenseDetails {
			totalRecurringExpenseAnalysis.TotalAmount = totalRecurringExpenseAnalysis.TotalAmount.Add(recurringExpense.TotalMonthlyProjectedAmount)
		}
		totalRecurringExpenseAnalysis.Type = "recurring_expense"

	case "budget":
		// Unmarshal to a slice of BudgetAnalysis
		var budgetDetails []BudgetAnalysis
		err := json.Unmarshal(personalFinanceRow.Details, &budgetDetails)
		if err != nil {
			return err
		}
		totalBudgetAnalysis.Details = append(totalBudgetAnalysis.Details, budgetDetails...)
		for _, budget := range budgetDetails {
			totalBudgetAnalysis.TotalAmount = totalBudgetAnalysis.TotalAmount.Add(budget.TotalAmount)
		}
		totalBudgetAnalysis.Type = "budget"

	case "debt":
		// Unmarshal to a slice of DebtAnalysis
		var debtDetails []DebtAnalysis
		err := json.Unmarshal(personalFinanceRow.Details, &debtDetails)
		if err != nil {
			return err
		}
		totalDebtAnalysis.Details = append(totalDebtAnalysis.Details, debtDetails...)
		for _, debt := range debtDetails {
			totalDebtAnalysis.TotalAmount = totalDebtAnalysis.TotalAmount.Add(debt.RemainingBalance)
		}
		totalDebtAnalysis.Type = "debt"
	case "goal":
		// Unmarshal to a slice of GoalAnalysis
		var goalDetails []GoalAnalysis
		err := json.Unmarshal(personalFinanceRow.Details, &goalDetails)
		if err != nil {
			return err
		}
		totalGoalAnalysis.Details = append(totalGoalAnalysis.Details, goalDetails...)
		for _, goal := range goalDetails {
			totalGoalAnalysis.TotalAmount = totalGoalAnalysis.TotalAmount.Add(goal.Amount)
		}
		totalGoalAnalysis.Type = "goal"
	default:
		// Unknown type
		return errors.New("unknown finance type")
	}

	return nil
}
