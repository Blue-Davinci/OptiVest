package data

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Blue-Davinci/OptiVest/internal/database"
)

var (
	DefaultSearchOptionDBContextTimeout = 5 * time.Second
)

type SearchOptionsModel struct {
	DB *database.Queries
}

type BudgetCategorySearchOption struct {
	ID       int    `json:"id"`
	Category string `json:"category"`
}

type BudgetIDNameSearchOption struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

func (s SearchOptionsModel) GetDistinctBudgetCategory(userID int64) ([]*BudgetCategorySearchOption, error) {
	ctx, cancel := contextGenerator(context.Background(), DefaultSearchOptionDBContextTimeout)
	defer cancel()
	// get data for the search options
	budgetCategories, err := s.DB.GetDistinctBudgetCategory(ctx, userID)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrGeneralRecordNotFound
		default:
			return nil, err
		}
	}
	// check length of the data
	if len(budgetCategories) == 0 {
		return nil, ErrGeneralRecordNotFound
	}
	// fill in our struct
	var searchOptions []*BudgetCategorySearchOption
	for i, category := range budgetCategories {
		searchOptions = append(searchOptions, &BudgetCategorySearchOption{
			ID:       i + 1,
			Category: category,
		})
	}
	// we are good now return
	return searchOptions, nil
}

// GetDistincBudgetIdBudgetName returns the distinct budget id and budget name
func (s SearchOptionsModel) GetDistinctBudgetIdBudgetName(userID int64) ([]*BudgetIDNameSearchOption, error) {
	ctx, cancel := contextGenerator(context.Background(), DefaultSearchOptionDBContextTimeout)
	defer cancel()
	// get data for the search options
	budgets, err := s.DB.GetDistinctBudgetIdBudgetName(ctx, userID)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrGeneralRecordNotFound
		default:
			return nil, err
		}
	}
	// check length of the data
	if len(budgets) == 0 {
		return nil, ErrGeneralRecordNotFound
	}
	// fill in our struct
	var searchOptions []*BudgetIDNameSearchOption
	for _, budget := range budgets {
		searchOptions = append(searchOptions, &BudgetIDNameSearchOption{
			ID:   budget.ID,
			Name: budget.Name,
		})
	}
	// we are good now return
	return searchOptions, nil
}
