package data

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
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
	// Enum For Tracking Type Enumeration
	FinManTrackingTypeEnumMonthly = database.TrackingTypeEnumMonthly
	FinManTrackingTypeEnumBonus   = database.TrackingTypeEnumBonus
	FinManTrackingTypeEnumOther   = database.TrackingTypeEnumOther
)

var (
	RedisFinManSurplusPrefix  = "finman_nudget_surplus:"
	RedisFinManGoalPlanPrefix = "finman_nudget_goal_plan:"
)

var (
	ErrInvalidOCFStatus  = errors.New("invalid status")
	ErrDuplicateGoal     = errors.New("your goal has a duplicate field")
	ErrDuplicateGoalPlan = errors.New("your goal saving plan has a duplicate field")
)

// MapStatusToConstant maps a status string to the corresponding constant
func (m FinancialManagerModel) MapStatusToOCFConstant(status string) (database.GoalStatus, error) {
	switch status {
	case "ongoing":
		return FinManEnumGoalStatusOngoing, nil
	case "completed":
		return FinManEnumGoalStatusCompleted, nil
	case "failed":
		return FinManEnumGoalStatusFailed, nil
	default:
		return "", ErrInvalidOCFStatus
	}
}

// MapTrackingTypeToConstant maps a tracking type string to the corresponding constant
func (m FinancialManagerModel) MapTrackingTypeToConstant(trackingType string) (database.TrackingTypeEnum, error) {
	switch trackingType {
	case "monthly":
		return FinManTrackingTypeEnumMonthly, nil
	case "bonus":
		return FinManTrackingTypeEnumBonus, nil
	case "other":
		return FinManTrackingTypeEnumOther, nil
	default:
		return "", ErrInvalidOCFStatus
	}
}

type FinancialManagerModel struct {
	DB *database.Queries
}

const (
	DefaultFinManDBContextTimeout = 5 * time.Second
	DefaultFinManRedisTTL         = 24 * time.Hour
)

// Enriched budget
type EnrichedBudget struct {
	Budget              Budget              `json:"budget"`
	Goal_Summary        []*Goal_Summary     `json:"goal_summary"`
	Goal_Summary_Totals Goal_Summary_Totals `json:"goal_summary_totals"`
}

type EnrichedBudgetSummary struct {
	BudgetID                        int64                    `json:"budget_id"`
	BudgetName                      string                   `json:"budget_name"`
	BudgetCategory                  string                   `json:"budget_category"`
	BudgetTotalAmount               decimal.Decimal          `json:"budget_total_amount"`
	BudgetIsStrict                  bool                     `json:"budget_is_strict"`
	Goals                           []map[string]interface{} `json:"goals"`
	RecurringExpenses               []map[string]interface{} `json:"recurring_expenses"`
	TotalProjectedRecurringExpenses decimal.Decimal          `json:"total_projected_recurring_expenses"`
	TotalExpenses                   decimal.Decimal          `json:"total_expenses"`
}

// Budget struct represents a user's Budget
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

// Goals struct represents a user's Goal
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

// GoalsWithProgression struct represents goals enriched with progressions and total amount contrivbuted
type GoalsWithProgression struct {
	Goals                   Goals           `json:"goals"`
	TotalContributedAmounts decimal.Decimal `json:"total_contributed_amounts"`
	ProgressPercentage      decimal.Decimal `json:"progress_percentage"`
}

// iInvestment Goal will hold an array of goals that will be used
// in our final LLM request to track user goals
type InvestmentGoal struct {
	Goals []InvestmentGoalHelper `json:"goals"`
}

// get the sum of target amount, current amount and monthly contribution
func (i InvestmentGoal) GetSumAnalysis() (decimal.Decimal, decimal.Decimal, decimal.Decimal) {
	var targetAmountSum decimal.Decimal
	var currentAmountSum decimal.Decimal
	var monthlyContributionSum decimal.Decimal
	// if we have no goals, we return 0 for all
	if len(i.Goals) == 0 {
		return decimal.NewFromInt(0), decimal.NewFromInt(0), decimal.NewFromInt(0)
	}

	for _, goal := range i.Goals {
		targetAmountSum = targetAmountSum.Add(goal.TargetAmount)
		currentAmountSum = currentAmountSum.Add(goal.CurrentAmount)
		monthlyContributionSum = monthlyContributionSum.Add(goal.MonthlyContribution)
	}
	return targetAmountSum, currentAmountSum, monthlyContributionSum
}

// Investment Goal Helper struct
type InvestmentGoalHelper struct {
	Name                string          `json:"name"`
	CurrentAmount       decimal.Decimal `json:"current_amount"`
	TargetAmount        decimal.Decimal `json:"target_amount"`
	MonthlyContribution decimal.Decimal `json:"monthly_contribution"`
	StartDate           time.Time       `json:"start_date"`
	EndDate             time.Time       `json:"end_date"`
}

// Goal_Summary struct represents a summary of a goal
type Goal_Summary struct {
	Id                  int64           `json:"id"`
	Name                string          `json:"name"`
	MonthlyContribution decimal.Decimal `json:"monthly_contribution"`
	TargetAmount        decimal.Decimal `json:"target_amount"`
}

// Goal_Summary_Totals struct represents the totals for a goal summary
type Goal_Summary_Totals struct {
	TotalBudgetAmount          decimal.Decimal `json:"total_budget_amount"`
	TotalMonthlyContribution   decimal.Decimal `json:"total_monthly_contribution"`
	TotalExpenses              decimal.Decimal `json:"total_expenses"`
	ProjectedRecurringExpenses decimal.Decimal `json:"projected_recurring_expenses"`
	TotalSurplus               decimal.Decimal `json:"total_surplus"`
}

// Goal Tracking struct represents howwe track our goals
type TrackedGoal struct {
	ID                    int64                     `json:"id"`
	UserID                int64                     `json:"user_id"`
	GoalID                int64                     `json:"goal_id"`
	TrackingDate          time.Time                 `json:"tracking_date"`
	ContributedAmount     decimal.Decimal           `json:"contributed_amount"`
	TrackingType          database.TrackingTypeEnum `json:"tracking_type"` // What type, if monthly, bonus or other
	CreatedAt             time.Time                 `json:"created_at"`
	UpdatedAt             time.Time                 `json:"updated_at"`
	TruncatedTrackingDate time.Time                 `json:"truncated_tracking_date"`
}

// Goal Plans struct represents how we plan our goals
type GoalPlan struct {
	ID                  int64           `json:"id"`
	UserID              int64           `json:"user_id"`
	Name                string          `json:"name"`
	Description         string          `json:"description"`
	TargetAmount        decimal.Decimal `json:"target_amount"`
	MonthlyContribution decimal.Decimal `json:"monthly_contribution"`
	DurationInMonths    int             `json:"duration_in_months"`
	IsStrict            bool            `json:"is_strict"`
	CreatedAt           time.Time       `json:"created_at"`
	UpdatedAt           time.Time       `json:"updated_at"`
}

type UnifiedGoalPlanMetadata struct {
	GoalPlan []*GoalPlan `json:"goal_plan"`
	Metadata Metadata    `json:"metadata"`
}

var Warning_Messages struct {
	Message []string `json:"message"`
}

// Nudget Validators
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
	v.Check(len(currencyCode) == 3, "currency_code", "must be 3 bytes long")
	// check if currency is in the list of supported currencies
}
func ValidateConversionRate(v *validator.Validator, conversionRate decimal.Decimal) {
	v.Check(conversionRate.GreaterThan(decimal.NewFromInt(0)), "conversion_rate", "must be greater than 0")
}
func ValidateBudgetDescription(v *validator.Validator, description string) {
	v.Check(description != "", "description", "must be provided")
	v.Check(len(description) <= 500, "description", "must not be more than 500 bytes long")
}
func ValidateBudgetStrictness(v *validator.Validator, isStrict bool) {
	v.Check(reflect.TypeOf(isStrict).Kind() == reflect.Bool, "is_strict", "must be a boolean")
}

// Goal Plan Template Validation
func ValidateGoalPlan(v *validator.Validator, goalPlan *GoalPlan) {
	// We only validate the Goal Plan name
	ValidateBudgetName(v, goalPlan.Name)
}

// ValidateBudget() validates a budget when we are ypdating it
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

// ValidateBudgetUpdate() validates a budget when we are updating it
// it has a very slight change from the ValidateBudget() function
func ValidateBudgetUpdate(v *validator.Validator, budget *Budget) {
	// Budget name
	ValidateBudgetName(v, budget.Name)
	// Budget category
	ValidateBudgetCategory(v, budget.Category)
	// Total amount
	ValidateBudgetTotalAmount(v, budget.TotalAmount)
	// Currency code
	ValidateConversionRate(v, budget.ConversionRate)
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

// getBudgetGoalExpenseSummary() is a helper method that will be used to retrieve
// a summary of a user's budget, goals and expenses
// We will return a *EnrichedBudget struct and an error if the operation fails.
func (m FinancialManagerModel) GetBudgetGoalExpenseSummary(userID int64) ([]*EnrichedBudgetSummary, error) {
	ctx, cancel := contextGenerator(context.Background(), DefaultFinManDBContextTimeout)
	defer cancel()
	// Fetch the budget from the database
	enrichedBugetRows, err := m.DB.GetBudgetGoalExpenseSummary(ctx, userID)
	if err != nil {
		return nil, err
	}
	// check if empty result
	if len(enrichedBugetRows) == 0 {
		return nil, ErrGeneralRecordNotFound
	}
	// initialize our enriched budget
	enrichedBudgets := []*EnrichedBudgetSummary{}
	// Process each budget
	for _, enrichedBudgetRow := range enrichedBugetRows {
		// make a budget
		budget := populateEnrichedBudgetSummary(enrichedBudgetRow)
		// append the enriched budget to the slice
		enrichedBudgets = append(enrichedBudgets, budget)
	}

	// everything went well
	return enrichedBudgets, nil
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
// It takes the user ID and a pointer to a Budget struct as parameters
// We do not allow users the CurrencyCode but they can change the conversion rate
func (m FinancialManagerModel) UpdateUserBudget(userID int64, updatedBudget *Budget) error {
	ctx, cancel := contextGenerator(context.Background(), DefaultFinManDBContextTimeout)
	defer cancel()
	updatedAt, err := m.DB.UpdateBudgetById(ctx, database.UpdateBudgetByIdParams{
		ID:             updatedBudget.Id,
		Name:           updatedBudget.Name,
		IsStrict:       updatedBudget.IsStrict,
		Category:       updatedBudget.Category,
		TotalAmount:    updatedBudget.TotalAmount.String(),
		ConversionRate: updatedBudget.ConversionRate.String(),
		Description:    sql.NullString{String: updatedBudget.Description, Valid: updatedBudget.Description != ""},
		UserID:         userID,
	})
	// check for an error
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return ErrEditConflict
		default:
			return err
		}
	}
	// fill in the updatedBudget with the timestamps
	updatedBudget.UpdatedAt = updatedAt
	// everything went well
	return nil
}

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

// GetBudgetsForUser() retrieves all budget records associated with a user
// This supports pagination and filtering by date created as well as a budget name search query
// It takes the user ID, search query, and pagination parameters as parameters
// We also return a summary of each budget by invoking our GetAllGoalSummaryBudgetID() for each budget
func (m FinancialManagerModel) GetBudgetsForUser(userID int64, searchQuery string, filters Filters) ([]*EnrichedBudget, Metadata, error) {
	ctx, cancel := contextGenerator(context.Background(), DefaultFinManDBContextTimeout)
	defer cancel()
	// Fetch budgets from the database
	budgets, err := m.DB.GetBudgetsForUser(ctx, database.GetBudgetsForUserParams{
		UserID:  userID,
		Column2: searchQuery,
		Limit:   int32(filters.limit()),
		Offset:  int32(filters.offset()),
	})
	if err != nil {
		return nil, Metadata{}, err
	}
	if len(budgets) == 0 {
		return nil, Metadata{}, ErrGeneralRecordNotFound
	}
	// initialize our total values
	totalBudgets := 0
	enrichedBudgets := []*EnrichedBudget{}
	// Process each budget
	for _, row := range budgets {
		var enrichedBudget EnrichedBudget
		totalBudgets = int(row.TotalBudgets)
		// make a budget
		budget := populateBudget(row)
		// return a goal summary and totals for each budget
		goalSummaryTotals, err := m.GetAllGoalSummaryBudgetID(budget.Id, userID)
		if err != nil {
			return nil, Metadata{}, err
		}
		// account for 0 goals by checking for the monthly contribution, if 0, we set empty structs {}
		if goalSummaryTotals.TotalMonthlyContribution.Equal(decimal.NewFromInt(0)) {
			goalSummaryTotals = &Goal_Summary_Totals{}
		}
		//enrich our budget
		enrichedBudget.Budget = *budget
		enrichedBudget.Goal_Summary_Totals = *goalSummaryTotals
		// append the enriched budget to the slice
		enrichedBudgets = append(enrichedBudgets, &enrichedBudget)
	}
	// create a metadata
	metadata := calculateMetadata(totalBudgets, filters.Page, filters.PageSize)
	// return the enriched budgets
	return enrichedBudgets, metadata, nil

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
	case database.GetBudgetsForUserRow:
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
func ValidateBudgetID(v *validator.Validator, budgetID int64) {
	v.Check(budgetID != 0, "budget_id", "must be provided")
}
func ValidateTargetAmountMoreThanCurrentAmount(v *validator.Validator, currentAmount, targetAmount decimal.Decimal) {
	v.Check(targetAmount.GreaterThan(currentAmount), "target_amount", "must be greater than the current amount")
}
func ValidateDates(v *validator.Validator, startDate, endDate time.Time,
	currentAmount, monthlyContribution, targetAmount decimal.Decimal) {

	// Ensure start date is before end date
	v.Check(startDate.Before(endDate), "start_date", "must be before end date")

	// Calculate the number of months between startDate and endDate
	monthsDifference := int(endDate.Sub(startDate).Hours() / (24 * 30)) // approximate months

	// Check if we can reach the target amount with the current and monthly contributions
	totalAmount := currentAmount.Add(monthlyContribution.Mul(decimal.NewFromInt(int64(monthsDifference))))

	v.Check(totalAmount.Cmp(targetAmount) >= 0, "target_amount",
		"the current amount and monthly contributions do not reach the target by the end date")
	fmt.Println("Total amount: ", totalAmount, "| Target amount: ", targetAmount, "| Month Difference", monthsDifference)
}

// ValidateGoal() validates a goal when we are adding and updating it
func ValidateGoal(v *validator.Validator, goal *Goals) {
	// Budget name
	ValidateBudgetName(v, goal.Name)
	// Budget category
	ValidateBudgetID(v, goal.BudgetID)
	// Total amount
	ValidateBudgetTotalAmount(v, goal.CurrentAmount)
	// Target amount
	ValidateBudgetTotalAmount(v, goal.TargetAmount)
	// Target amount must be more than current amount
	ValidateTargetAmountMoreThanCurrentAmount(v, goal.CurrentAmount, goal.TargetAmount)
	// Monthly Contribution
	ValidateBudgetTotalAmount(v, goal.MonthlyContribution)
	// Currency code
	ValidateDates(v, goal.StartDate, goal.EndDate, goal.CurrentAmount, goal.MonthlyContribution, goal.TargetAmount)
}

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
		switch {
		case err.Error() == `pq: duplicate key value violates unique constraint "unique_user_goal_name"`:
			return ErrDuplicateGoal
		default:
			return err
		}
	}
	// fill in the newGoal with the ID and timestamps
	newGoal.Id = goal.ID
	newGoal.CreatedAt = goal.CreatedAt
	newGoal.UpdatedAt = goal.UpdatedAt
	// everything went well
	return nil
}

// UpdateGoalByID() updates a goal record in the database by its ID
// It takes the user ID and a pointer to a Goals struct as parameters
func (m FinancialManagerModel) UpdateGoalByID(userID int64, updatedGoal *Goals) error {
	ctx, cancel := contextGenerator(context.Background(), DefaultFinManDBContextTimeout)
	defer cancel()
	updatedAt, err := m.DB.UpdateGoalByID(ctx, database.UpdateGoalByIDParams{
		Name:                updatedGoal.Name,
		CurrentAmount:       sql.NullString{String: updatedGoal.CurrentAmount.String(), Valid: updatedGoal.CurrentAmount.String() != ""},
		TargetAmount:        updatedGoal.TargetAmount.String(),
		MonthlyContribution: updatedGoal.MonthlyContribution.String(),
		StartDate:           updatedGoal.StartDate,
		EndDate:             updatedGoal.EndDate,
		Status:              updatedGoal.Status,
		ID:                  updatedGoal.Id,
		UserID:              userID,
	})
	// check for an error
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return ErrEditConflict
		case err.Error() == `pq: duplicate key value violates unique constraint "unique_user_goal_name"`:
			fmt.Println("---- Duplicate key error")
			return ErrDuplicateGoal
		default:
			fmt.Println("---- 1Error: ", err)
			return err
		}
	}
	// fill in the updatedGoal with the timestamps
	updatedGoal.UpdatedAt = updatedAt
	// everything went well
	return nil
}

// GetGoalByID() retrieves a goal record from the database by its ID
// It takes the goal ID as a parameter and returns a pointer to a Goals struct
// and an error if the operation fails.
func (m FinancialManagerModel) GetGoalByID(userID, goalID int64) (*Goals, error) {
	ctx, cancel := contextGenerator(context.Background(), DefaultFinManDBContextTimeout)
	defer cancel()
	goal, err := m.DB.GetGoalByID(ctx, database.GetGoalByIDParams{
		ID:     goalID,
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
	// fill in the Goals struct with the data from the database
	idGoals := populateGoal(goal)
	// everything went well
	return idGoals, nil
}

// GetGoalsForUserInvestmentHelper() is an investment helper method, and will be used to retieve
// a sample subset of a users goals for investment purposes
// These items are: name, current amount, target amount, monthly contribution, start date and the end date
// We will return a *InvestmentGoal struct and an error if the operation fails.
func (m FinancialManagerModel) GetGoalsForUserInvestmentHelper(userID int64) (*InvestmentGoal, error) {
	ctx, cancel := contextGenerator(context.Background(), DefaultFinManDBContextTimeout)
	defer cancel()
	goals, err := m.DB.GetGoalsForUserInvestmentHelper(ctx, userID)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrGeneralRecordNotFound
		default:
			return nil, err
		}
	}
	// check if empty result
	if len(goals) == 0 {
		return nil, ErrGeneralRecordNotFound
	}
	// initialize goal of helpers
	investmentGoalHelperSlice := []InvestmentGoalHelper{}
	// Process each goal appending it to the slice
	for _, row := range goals {
		// populate the InvestmentGoalHelper struct using populateInvestmentGoalHelper
		investmentGoal := populateInvestmentGoalHelper(row)
		// append the investment goal to the slice
		investmentGoalHelperSlice = append(investmentGoalHelperSlice, *investmentGoal)
	}
	// create a new InvestmentGoal struct
	investmentGoal := &InvestmentGoal{
		Goals: investmentGoalHelperSlice,
	}
	// everything went well
	return investmentGoal, nil
}

// GetAllGoalsWithProgressionByUserID() retrieves all goal records associated with a user ID
func (m FinancialManagerModel) GetAllGoalsWithProgressionByUserID(userID int64) ([]*GoalsWithProgression, error) {
	ctx, cancel := contextGenerator(context.Background(), DefaultFinManDBContextTimeout)
	defer cancel()
	// Fetch goals from the database
	goals, err := m.DB.GetAllGoalsWithProgressionByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if len(goals) == 0 {
		return nil, ErrGeneralRecordNotFound
	}
	// initialize our goals with progression
	goalsWithProgressions := []*GoalsWithProgression{}
	// Process each goal
	for _, goal := range goals {
		var goalsWithProgression GoalsWithProgression
		// populate the goal struct
		populatedGoal := populateGoal(goal)
		// fill in the total contributed amount
		goalsWithProgression = GoalsWithProgression{
			Goals:                   *populatedGoal,
			TotalContributedAmounts: decimal.RequireFromString(goal.TotalContributedAmount),
			ProgressPercentage:      decimal.RequireFromString(goal.ProgressPercentage),
		}
		goalsWithProgressions = append(goalsWithProgressions, &goalsWithProgression)
	}
	// everything went well
	return goalsWithProgressions, nil
}

// ============================================================================================================
// Goal Tracking
// ============================================================================================================

// CreateNewGoalPlan() creates a new goal plan record in the database
// This are essentially templates for goals that users can create
// so validation need not be as strict as when creating a goal
// It takes a pointer to a GoalPlan struct and a user ID
// We return an error if the operation fails.
func (m FinancialManagerModel) CreateNewGoalPlan(userID int64, newGoalPlan *GoalPlan) error {
	ctx, cancel := contextGenerator(context.Background(), DefaultFinManDBContextTimeout)
	defer cancel()
	goalPlan, err := m.DB.CreateNewGoalPlan(ctx, database.CreateNewGoalPlanParams{
		UserID:              userID,
		Name:                newGoalPlan.Name,
		Description:         sql.NullString{String: newGoalPlan.Description, Valid: newGoalPlan.Description != ""},
		TargetAmount:        sql.NullString{String: newGoalPlan.TargetAmount.String(), Valid: newGoalPlan.TargetAmount.String() != ""},
		MonthlyContribution: sql.NullString{String: newGoalPlan.MonthlyContribution.String(), Valid: newGoalPlan.MonthlyContribution.String() != ""},
		DurationInMonths:    sql.NullInt32{Int32: int32(newGoalPlan.DurationInMonths), Valid: newGoalPlan.DurationInMonths != 0},
		IsStrict:            newGoalPlan.IsStrict,
	})
	if err != nil {
		switch {
		case err.Error() == `pq: duplicate key value violates unique constraint "idx_unique_user_goal_plan_name"`:
			return ErrDuplicateGoalPlan
		default:
			return err
		}
	}
	// fill in the newGoalPlan with the ID and timestamps
	newGoalPlan.ID = goalPlan.ID
	newGoalPlan.UserID = userID
	newGoalPlan.CreatedAt = goalPlan.CreatedAt.Time
	newGoalPlan.UpdatedAt = goalPlan.UpdatedAt.Time
	// everything went well
	return nil
}

// UpdateGoalPlanByID() updates a goal plan record in the database by its ID and User ID
// It takes the user ID and a pointer to a GoalPlan struct as parameters
func (m FinancialManagerModel) UpdateGoalPlanByID(userID int64, updatedGoalPlan *GoalPlan) error {
	ctx, cancel := contextGenerator(context.Background(), DefaultFinManDBContextTimeout)
	defer cancel()
	updatedAt, err := m.DB.UpdateGoalPlanByID(ctx, database.UpdateGoalPlanByIDParams{
		Name:                updatedGoalPlan.Name,
		Description:         sql.NullString{String: updatedGoalPlan.Description, Valid: updatedGoalPlan.Description != ""},
		TargetAmount:        sql.NullString{String: updatedGoalPlan.TargetAmount.String(), Valid: updatedGoalPlan.TargetAmount.String() != ""},
		MonthlyContribution: sql.NullString{String: updatedGoalPlan.MonthlyContribution.String(), Valid: updatedGoalPlan.MonthlyContribution.String() != ""},
		DurationInMonths:    sql.NullInt32{Int32: int32(updatedGoalPlan.DurationInMonths), Valid: updatedGoalPlan.DurationInMonths != 0},
		IsStrict:            updatedGoalPlan.IsStrict,
		ID:                  updatedGoalPlan.ID,
		UserID:              userID,
	})
	// check for an error
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return ErrEditConflict
		case err.Error() == `pq: duplicate key value violates unique constraint "idx_unique_user_goal_plan_name"`:
			return ErrDuplicateGoalPlan
		default:
			return err
		}
	}
	// fill in the updatedGoalPlan with the timestamps
	updatedGoalPlan.UpdatedAt = updatedAt.Time
	// everything went well
	return nil
}

// GetGoalPlanByID() retrieves a goal plan record from the database by its ID and User ID
// It takes the goal plan ID and user ID as parameters and returns a pointer to a GoalPlan struct
// and an error if the operation fails.
func (m FinancialManagerModel) GetGoalPlanByID(userID, goalPlanID int64) (*GoalPlan, error) {
	ctx, cancel := contextGenerator(context.Background(), DefaultFinManDBContextTimeout)
	defer cancel()
	goalPlan, err := m.DB.GetGoalPlanByID(ctx, database.GetGoalPlanByIDParams{
		ID:     goalPlanID,
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
	// fill in the GoalPlan struct with the data from the database
	idGoalPlan := populateGoalPlan(goalPlan)
	// everything went well
	return idGoalPlan, nil
}

// GetGoalPlansForUser() retrieves all goal plan records associated with a user
// This supports pagination and filtering by date created.
// It takes the user ID and pagination parameters as parameters
func (m FinancialManagerModel) GetGoalPlansForUser(userID int64, filters Filters) ([]*GoalPlan, Metadata, error) {
	ctx, cancel := contextGenerator(context.Background(), DefaultFinManDBContextTimeout)
	defer cancel()
	// Fetch goal plans from the database
	goalPlans, err := m.DB.GetGoalPlansForUser(ctx, database.GetGoalPlansForUserParams{
		UserID: userID,
		Limit:  int32(filters.limit()),
		Offset: int32(filters.offset()),
	})
	if err != nil {
		return nil, Metadata{}, err
	}
	if len(goalPlans) == 0 {
		return nil, Metadata{}, ErrGeneralRecordNotFound
	}
	// initialize our total values
	totalGoalPlans := 0
	goalPlanSlice := []*GoalPlan{}
	// Process each goal plan
	for _, row := range goalPlans {
		var goalPlan GoalPlan
		// fill in the GoalPlan struct with the data from the database
		goalPlan.ID = row.ID
		goalPlan.UserID = row.UserID
		goalPlan.Name = row.Name
		goalPlan.Description = row.Description.String
		goalPlan.TargetAmount = decimal.RequireFromString(row.TargetAmount.String)
		goalPlan.MonthlyContribution = decimal.RequireFromString(row.MonthlyContribution.String)
		goalPlan.DurationInMonths = int(row.DurationInMonths.Int32)
		goalPlan.IsStrict = row.IsStrict
		goalPlan.CreatedAt = row.CreatedAt.Time
		goalPlan.UpdatedAt = row.UpdatedAt.Time
		// append the goal plan to the slice
		goalPlanSlice = append(goalPlanSlice, &goalPlan)
	}
	// create a metadata
	metadata := calculateMetadata(totalGoalPlans, filters.Page, filters.PageSize)
	// return the goal plans
	return goalPlanSlice, metadata, nil
}

// GetAndSaveAllGoalsForTracking() is the main tracking function that will be used to track goals
// It is designed to work in tandem with the cron job scheduler whih will be running to
// check goals that are due for tracking and track them
// We get a limit as we will be running this in batches.
// We return a pointer to a TrackedGoal struct and an error if the operation fails.
func (m FinancialManagerModel) GetAndSaveAllGoalsForTracking() ([]*TrackedGoal, error) {
	ctx, cancel := contextGenerator(context.Background(), DefaultFinManDBContextTimeout)
	defer cancel()
	trackedGoals, err := m.DB.GetAndSaveAllGoalsForTracking(ctx)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrGeneralRecordNotFound
		default:
			return nil, err
		}
	}
	if len(trackedGoals) == 0 {
		return nil, ErrGeneralRecordNotFound
	}
	// initializa a slice of TrackedGoal
	trackedGoalsSlice := []*TrackedGoal{}
	// Process each tracked goal
	for _, row := range trackedGoals {
		var trackedGoal TrackedGoal
		// fill in the TrackedGoal struct with the data from the database
		trackedGoal.ID = row.ID
		trackedGoal.UserID = row.UserID
		trackedGoal.GoalID = row.GoalID.Int64
		trackedGoal.ContributedAmount = decimal.RequireFromString(row.ContributedAmount)
		// append the tracked goal to the slice
		trackedGoalsSlice = append(trackedGoalsSlice, &trackedGoal)
	}
	// everything went well
	return trackedGoalsSlice, nil
}

// UpdateGoalProgressOnExpiredGoals() is the main function that will be used to update goals that have expired
// Will be used in tandem and work 1 way with the cron job scheduler
// We return nothing and an error if the operation fails.
func (m FinancialManagerModel) UpdateGoalProgressOnExpiredGoals() error {
	ctx, cancel := contextGenerator(context.Background(), DefaultFinManDBContextTimeout)
	defer cancel()
	err := m.DB.UpdateGoalProgressOnExpiredGoals(ctx)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return ErrEditConflict
		default:
			return err
		}
	}
	// everything went well
	return nil
}

// ============================================================================================================
// Saving Goal Plan
// ============================================================================================================

// GetAllGoalSummaryBuBudgetID() retrieves all goal summaries associated with a budget
// We return the goal summaries and additional totals which contains the total goals, total monthly contribution
// total surplus, budget total amount, budget currency and budget strictness
// This is the main function that will be used to get and manage surplus by most of the handlers
func (m FinancialManagerModel) GetAllGoalSummaryBudgetID(budgetID, userID int64) (*Goal_Summary_Totals, error) {
	ctx, cancel := contextGenerator(context.Background(), DefaultFinManDBContextTimeout)
	defer cancel()

	fmt.Println("Received data: budgetID:", budgetID, "userID:", userID)

	// Fetch goals from the database
	goals, err := m.DB.GetAllGoalSummaryByBudgetID(ctx, database.GetAllGoalSummaryByBudgetIDParams{
		ID:     budgetID,
		UserID: userID,
	})
	if err != nil {
		fmt.Println("0Error: ", err)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrGeneralRecordNotFound
		default:
			return nil, err
		}
	}

	// Initialize totals and summaries
	goalTotals := &Goal_Summary_Totals{}

	// Process each goal
	for _, row := range goals {

		// Check if BudgetTotalAmount is empty and set a default value if needed
		if row.TotalAmount != "" {
			goalTotals.TotalBudgetAmount, err = convertToDecimal(row.TotalAmount)
			if err != nil {
				fmt.Println("--Error: ", err)
				return nil, err
			}
		} else {
			goalTotals.TotalBudgetAmount = decimal.NewFromInt(0) // Default to 0 if empty
		}

		// Check if TotalMonthlyContributions is empty and set default if necessary
		// type conversion is necessary as the value is a string
		if row.TotalMonthlyContributions != "" {
			goalTotals.TotalMonthlyContribution, err = convertToDecimal(row.TotalMonthlyContributions)
			if err != nil {
				fmt.Println("--1Error: ", err)
				return nil, err
			}
		} else {
			goalTotals.TotalMonthlyContribution = decimal.NewFromInt(0) // Default to 0 if empty
		}

		// Total expenses, convert to string}
		if row.TotalExpenses != "" {
			goalTotals.TotalExpenses, err = convertToDecimal(row.TotalExpenses)
			if err != nil {
				fmt.Println("--2Error: ", err)
				return nil, err
			}
		} else {
			goalTotals.TotalExpenses = decimal.NewFromInt(0) // Default to 0 if empty
		}
		// projected recurring expenses, convert to string
		if row.ProjectedRecurringExpenses != "" {
			goalTotals.ProjectedRecurringExpenses, err = convertToDecimal(row.ProjectedRecurringExpenses)
			if err != nil {
				fmt.Println("--3Error: ", err)
				return nil, err
			}
		} else {
			goalTotals.ProjectedRecurringExpenses = decimal.NewFromInt(0) // Default to 0 if empty
		}

		// Check if BudgetSurplus is empty and set default if necessary
		if row.BudgetSurplus != "" {
			goalTotals.TotalSurplus, err = decimal.NewFromString(row.BudgetSurplus)
			if err != nil {
				fmt.Println("--4Error: ", err)
				return nil, err
			}
		} else {
			goalTotals.TotalSurplus = decimal.NewFromInt(0) // Default to 0 if empty
		}
	}
	fmt.Println("Goal Totals: ", goalTotals.TotalSurplus)
	// Return the goal summaries and totals
	return goalTotals, nil
}

// ============================================================================================================
// Populators
// ============================================================================================================

func populateEnrichedBudgetSummary(enrichedBugetSummaryRow interface{}) *EnrichedBudgetSummary {
	switch enrichedBudget := enrichedBugetSummaryRow.(type) {
	case database.GetBudgetGoalExpenseSummaryRow:
		var enrichedBudgetSummary EnrichedBudgetSummary

		// Unmarshal the goals (JSON array)
		err := json.Unmarshal([]byte(enrichedBudget.Goals), &enrichedBudgetSummary.Goals)
		if err != nil {
			return nil
		}

		// Unmarshal the recurring expenses (JSON array)
		err = json.Unmarshal([]byte(enrichedBudget.RecurringExpenses), &enrichedBudgetSummary.RecurringExpenses)
		if err != nil {
			return nil
		}

		// Fill in the rest of the enriched budget summary
		enrichedBudgetSummary.BudgetID = enrichedBudget.BudgetID
		enrichedBudgetSummary.BudgetName = enrichedBudget.BudgetName
		enrichedBudgetSummary.BudgetCategory = enrichedBudget.BudgetCategory
		enrichedBudgetSummary.BudgetTotalAmount = decimal.RequireFromString(enrichedBudget.BudgetTotalAmount)
		enrichedBudgetSummary.BudgetIsStrict = enrichedBudget.BudgetIsStrict
		// get int64 totals
		enrichedBudgetSummary.TotalProjectedRecurringExpenses = decimal.RequireFromString(enrichedBudget.TotalProjectedRecurringExpenses)
		enrichedBudgetSummary.TotalExpenses = decimal.RequireFromString(enrichedBudget.TotalExpenses)

		fmt.Println("--- Enriched Budget Summary: ", enrichedBudgetSummary.BudgetName)
		// Return the enriched budget summary
		return &enrichedBudgetSummary

	default:
		return nil
	}
}

func populateGoal(goalRow interface{}) *Goals {
	switch goal := goalRow.(type) {
	case database.GetGoalByIDRow:
		return &Goals{
			Id:                  goal.ID,
			UserID:              goal.UserID,
			BudgetID:            goal.BudgetID.Int64,
			Name:                goal.Name,
			CurrentAmount:       decimal.RequireFromString(goal.CurrentAmount.String),
			TargetAmount:        decimal.RequireFromString(goal.TargetAmount),
			MonthlyContribution: decimal.RequireFromString(goal.MonthlyContribution),
			StartDate:           goal.StartDate,
			EndDate:             goal.EndDate,
			Status:              goal.Status,
			CreatedAt:           goal.CreatedAt,
			UpdatedAt:           goal.UpdatedAt,
		}
	case database.GetAllGoalsWithProgressionByUserIDRow:
		return &Goals{
			Id:                  goal.ID,
			UserID:              goal.UserID,
			BudgetID:            goal.BudgetID.Int64,
			Name:                goal.Name,
			CurrentAmount:       decimal.RequireFromString(goal.CurrentAmount.String),
			TargetAmount:        decimal.RequireFromString(goal.TargetAmount),
			MonthlyContribution: decimal.RequireFromString(goal.MonthlyContribution),
			StartDate:           goal.StartDate,
			EndDate:             goal.EndDate,
			Status:              goal.Status,
			CreatedAt:           goal.CreatedAt,
			UpdatedAt:           goal.UpdatedAt,
		}
	default:
		return nil
	}
}

func populateInvestmentGoalHelper(goalRow interface{}) *InvestmentGoalHelper {
	switch goal := goalRow.(type) {
	case database.GetGoalsForUserInvestmentHelperRow:
		return &InvestmentGoalHelper{
			Name:                goal.Name,
			CurrentAmount:       decimal.RequireFromString(goal.CurrentAmount.String),
			TargetAmount:        decimal.RequireFromString(goal.TargetAmount),
			MonthlyContribution: decimal.RequireFromString(goal.MonthlyContribution),
			StartDate:           goal.StartDate,
			EndDate:             goal.EndDate,
		}
	default:
		return nil
	}
}

func populateGoalPlan(goalPlanRow interface{}) *GoalPlan {
	switch goalPlan := goalPlanRow.(type) {
	case database.GoalPlan:
		return &GoalPlan{
			ID:                  goalPlan.ID,
			UserID:              goalPlan.UserID,
			Name:                goalPlan.Name,
			Description:         goalPlan.Description.String,
			TargetAmount:        decimal.RequireFromString(goalPlan.TargetAmount.String),
			MonthlyContribution: decimal.RequireFromString(goalPlan.MonthlyContribution.String),
			DurationInMonths:    int(goalPlan.DurationInMonths.Int32),
			IsStrict:            goalPlan.IsStrict,
			CreatedAt:           goalPlan.CreatedAt.Time,
			UpdatedAt:           goalPlan.UpdatedAt.Time,
		}
	default:
		return nil
	}
}
