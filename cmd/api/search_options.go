package main

import (
	"net/http"

	"github.com/Blue-Davinci/OptiVest/internal/data"
)

// getDistinctBudgetCategoryHandler() is a handler function that returns a list of distinct budget categories for a user.
// Users the models.SearchOptions.GetDistinctBudgetCategory() method to get the data.
// We pass in the user id
// If there is an error we return a 500 status code and the error message
func (app *application) getDistinctBudgetCategoryHandler(w http.ResponseWriter, r *http.Request) {
	// extract user
	userID := app.contextGetUser(r).ID
	// get the data
	budgetCategories, err := app.models.SearchOptions.GetDistinctBudgetCategory(userID)
	if err != nil {
		switch {
		case err == data.ErrGeneralRecordNotFound:
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// write the data
	err = app.writeJSON(w, http.StatusOK, envelope{"budget_categories": budgetCategories}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
