package data

import (
	"context"
	"math"
	"math/rand"
	"time"

	"github.com/Blue-Davinci/OptiVest/internal/database"
	"github.com/shopspring/decimal"
)

type AlgoManager struct {
	DB *database.Queries
}

const (
	DefaultAlgoManDBContextTimeout = 5 * time.Second
	// Ramdomization factor
	DefaultRandomizationFactor = 0.05
)

// Weight factors for different metrics
const (
	weightProfileCompletion = 0.3
	weightBudgets           = 0.15
	weightGoals             = 0.25
	weightExpenses          = 0.1
	weightIncome            = 0.1
	weightGroups            = 0.1
	weightAccountAge        = 0.1
	weightAwards            = 0.2
)

// EnrichedAccountStats holds all the stats of the account along with the account rating
type EnrichedAccountStats struct {
	AccountStats  *AccountStats
	AccountRating decimal.Decimal
}

// AccountStats holds all the statistics needed for the account rating calculation
type AccountStats struct {
	ProfileCompletion   decimal.Decimal
	TotalBudgets        int
	TotalBudgetAmount   decimal.Decimal
	AvgBudgetAmount     decimal.Decimal
	StdDevBudgetAmount  decimal.Decimal
	TotalGoals          int
	AvgGoalCompletion   decimal.Decimal
	AvgGoalAmount       decimal.Decimal
	StdDevGoalAmount    decimal.Decimal
	TotalExpenses       int
	TotalExpenseAmount  decimal.Decimal
	TotalIncomeSources  int
	TotalIncomeAmount   decimal.Decimal
	GroupsJoined        int
	GroupsCreated       int
	AvgGoalProgress     decimal.Decimal
	StdDevGoalProgress  decimal.Decimal
	AccountCreatedAt    time.Time
	RandomizationFactor decimal.Decimal
}

// ClampDecimal ensures a value is within the given min and max range
func (m AlgoManager) ClampDecimal(value, min, max decimal.Decimal) decimal.Decimal {
	if value.LessThan(min) {
		return min
	}
	if value.GreaterThan(max) {
		return max
	}
	return value
}

// CalculateAccountAgeFactor computes a factor based on the account's age
func (m AlgoManager) CalculateAccountAgeFactor(accountCreatedAt time.Time) decimal.Decimal {
	now := time.Now()
	ageInMonths := decimal.NewFromFloat(now.Sub(accountCreatedAt).Hours() / (24 * 30)) // Age in months

	if ageInMonths.LessThan(decimal.NewFromInt(6)) {
		// New account: reward engagement, but keep lower baseline
		return decimal.NewFromFloat(0.6)
	} else if ageInMonths.LessThanOrEqual(decimal.NewFromInt(24)) {
		// Medium account: moderate reward
		return decimal.NewFromFloat(0.85)
	} else {
		// Old account: highest reward for sustained engagement
		return decimal.NewFromFloat(1.0)
	}
}

// CalculateAccountRating computes the final account rating
func (m AlgoManager) CalculateAccountRating(stats AccountStats, awards []*Award) decimal.Decimal {
	// Calculate total award points
	totalAwardPoints := decimal.Zero
	for _, award := range awards {
		totalAwardPoints = totalAwardPoints.Add(decimal.NewFromInt32(award.Points))
	}

	// Normalize award points (assuming max possible points is 1000 for scaling)
	normalizedAwards := totalAwardPoints.Div(decimal.NewFromInt(1000))
	if normalizedAwards.GreaterThan(decimal.NewFromInt(1)) {
		normalizedAwards = decimal.NewFromInt(1) // Clamp to max value of 1
	}
	// Normalize budgets (comparison to peer stats)
	normalizedBudgetAmount := decimal.Zero
	if !stats.StdDevBudgetAmount.IsZero() {
		normalizedBudgetAmount = stats.TotalBudgetAmount.Sub(stats.AvgBudgetAmount).Div(stats.StdDevBudgetAmount)
	}
	// Normalize goals (comparison to peer stats)
	normalizedGoalCompletion := stats.AvgGoalCompletion.Div(decimal.NewFromInt(100)) // Normalize to a range [0, 1]
	normalizedGoalProgress := decimal.Zero
	if !stats.StdDevGoalProgress.IsZero() {
		normalizedGoalProgress = stats.AvgGoalProgress.Sub(stats.AvgGoalAmount).Div(stats.StdDevGoalProgress)
	}

	// Normalize expenses relative to income
	expenseToIncomeRatio := decimal.Zero
	if stats.TotalIncomeAmount.GreaterThan(decimal.Zero) {
		expenseToIncomeRatio = stats.TotalExpenseAmount.Div(stats.TotalIncomeAmount)
	}
	expensePenalty := m.ClampDecimal(decimal.NewFromInt(1).Sub(expenseToIncomeRatio), decimal.Zero, decimal.NewFromInt(1))

	// Apply decay functions to large values (diminishing returns)
	scaledBudgets := decimal.NewFromFloat(math.Log1p(float64(stats.TotalBudgets))) // Logarithmic scaling
	scaledGoals := decimal.NewFromFloat(math.Log1p(float64(stats.TotalGoals)))     // Logarithmic scaling
	scaledGroups := decimal.NewFromFloat(math.Log1p(float64(stats.GroupsJoined)))  // Logarithmic scaling
	scaledGroupsCreated := decimal.NewFromFloat(math.Log1p(float64(stats.GroupsCreated)))

	// Account age factor
	accountAgeFactor := m.CalculateAccountAgeFactor(stats.AccountCreatedAt)

	// Randomization factor for slight variation
	rand.Seed(time.Now().UnixNano())
	randomFactor := stats.RandomizationFactor.Mul(decimal.NewFromFloat(rand.Float64()))

	// Compute weighted score
	finalScore := decimal.Zero
	finalScore = finalScore.Add(stats.ProfileCompletion.Div(decimal.NewFromInt(100)).Mul(decimal.NewFromFloat(weightProfileCompletion)))
	finalScore = finalScore.Add(normalizedBudgetAmount.Add(scaledBudgets).Mul(decimal.NewFromFloat(weightBudgets)))
	finalScore = finalScore.Add(normalizedGoalCompletion.Add(normalizedGoalProgress).Add(scaledGoals).Mul(decimal.NewFromFloat(weightGoals)))
	finalScore = finalScore.Add(expensePenalty.Mul(decimal.NewFromFloat(weightExpenses)))
	finalScore = finalScore.Add(decimal.NewFromFloat(math.Log1p(float64(stats.TotalIncomeSources))).Mul(decimal.NewFromFloat(weightIncome)))
	finalScore = finalScore.Add(scaledGroups.Add(scaledGroupsCreated).Mul(decimal.NewFromFloat(weightGroups)))
	finalScore = finalScore.Add(accountAgeFactor.Mul(decimal.NewFromFloat(weightAccountAge)))
	finalScore = finalScore.Add(randomFactor)

	// Scale to [0, 100]
	finalScore = m.ClampDecimal(finalScore.Mul(decimal.NewFromInt(10)), decimal.Zero, decimal.NewFromInt(100))

	return finalScore
}

// GetAccountStatisticsByUserId() retrieves all the statistics needed for the account rating calculation
// We take in the user ID
func (m AlgoManager) GetAccountStatisticsByUserId(userID int64, accountCreationDate time.Time, awards []*Award) (*EnrichedAccountStats, error) {
	ctx, cancel := contextGenerator(context.Background(), DefaultAlgoManDBContextTimeout)
	defer cancel()
	// get the statistics
	stats, err := m.DB.GetAccountStatisticsByUserId(ctx, userID)
	if err != nil {
		return nil, err
	}
	// make the account stats struct
	accountStats := &AccountStats{
		ProfileCompletion:   decimal.RequireFromString(stats.ProfileCompletion.String),
		TotalBudgets:        int(stats.TotalBudgets),
		TotalBudgetAmount:   decimal.NewFromInt32(stats.TotalBudgetAmount),
		AvgBudgetAmount:     decimal.NewFromFloat(stats.AvgBudgetAmount),
		StdDevBudgetAmount:  decimal.NewFromFloat(stats.StddevBudgetAmount),
		TotalGoals:          int(stats.TotalGoals),
		AvgGoalCompletion:   decimal.NewFromFloat(stats.AvgGoalAmount),
		AvgGoalAmount:       decimal.NewFromFloat(stats.AvgGoalAmount),
		StdDevGoalAmount:    decimal.NewFromFloat(stats.StddevGoalAmount),
		TotalExpenses:       int(stats.TotalExpenses),
		TotalExpenseAmount:  decimal.NewFromFloat(float64(stats.TotalExpenseAmount)),
		TotalIncomeSources:  int(stats.TotalIncomeSources),
		TotalIncomeAmount:   decimal.NewFromFloat(float64(stats.TotalIncomeAmount)),
		GroupsJoined:        int(stats.GroupsJoined),
		GroupsCreated:       int(stats.GroupsCreated),
		AvgGoalProgress:     decimal.NewFromFloat(stats.AvgGoalProgress),
		StdDevGoalProgress:  decimal.NewFromFloat(stats.StddevGoalProgress),
		AccountCreatedAt:    accountCreationDate,
		RandomizationFactor: decimal.NewFromFloat(DefaultRandomizationFactor),
	}
	// calculate the account rating
	accountRating := m.CalculateAccountRating(*accountStats, awards)
	// return the account stats and account rating
	return &EnrichedAccountStats{
		AccountStats:  accountStats,
		AccountRating: accountRating,
	}, nil
}
