package main

import (
	"errors"
	"net/http"
	"time"

	"github.com/Blue-Davinci/OptiVest/internal/data"
	"github.com/Blue-Davinci/OptiVest/internal/validator"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

//==============================================================================================================
// BUDGET HANDLERS
//==============================================================================================================

// createNewBudgetdHandler() is a handler function that handles the creation of a Budget.
// We validate a the recieved inputs in our input struct.
// If everything is okay, we perform a check to see if the currency code of the budget is
// the same as the user's currency code. If it is not the same, we use our convertor function
// to convert the amount to the user's currency code. We then save the budget to the database
// including the convertion rate.
func (app *application) createNewBudgetdHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name         string          `json:"name"`
		IsStrict     bool            `json:"is_strict"`
		Category     string          `json:"category"`
		TotalAmount  decimal.Decimal `json:"total_amount"`
		CurrencyCode string          `json:"currency_code"`
		Description  string          `json:"description"`
	}
	// Decode the request body into the input struct
	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	// get our user
	user := app.contextGetUser(r)
	// Create a new Budget struct and fill it with the data from the input struct
	newBudget := &data.Budget{
		UserID:       user.ID,
		Name:         input.Name,
		IsStrict:     input.IsStrict,
		Category:     input.Category,
		TotalAmount:  input.TotalAmount,
		CurrencyCode: input.CurrencyCode,
		Description:  input.Description,
	}
	// Perform validation
	v := validator.New()
	if data.ValidateBudget(v, newBudget); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// check if provided currency code is supported
	if err := app.verifyCurrencyInRedis(newBudget.CurrencyCode); err != nil {
		v.AddError("currency_code", "currency code is not supported")
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// We check if the currency code is similar to the user's currency code
	// if not we convert the amount to the user's currency code
	// and save.
	if newBudget.CurrencyCode != user.CurrencyCode {
		// Convert the amount to the user's currency code
		convertedAmount, err := app.convertAndGetExchangeRate(newBudget.CurrencyCode, user.CurrencyCode)
		if err != nil {
			v.AddError("currency_code", "could not convert currency")
			app.failedValidationResponse(w, r, v.Errors)
			return
		}
		// set the currency and exchange rate to the user's budget
		newBudget.TotalAmount = convertedAmount.ConvertAmount(newBudget.TotalAmount).ConvertedAmount
	} else {
		// otherwise we set the exchange rate to 1/ users default currency
		// set the exchange rate to 1
		newBudget.ConversionRate = decimal.NewFromInt(1)
	}
	// Save the budget to the database
	err = app.models.FinancialManager.CreateNewBudget(newBudget)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	// Return a 201 Created status code along with the budget in the response body
	err = app.writeJSON(w, http.StatusCreated, envelope{"budget": newBudget}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// updateBudgetHandler() is a handler function that handles the updating of a Budget.
// we perform general validations on the input struct.
// If all is well, we check if the currency was changed, if it ws we throw an error.
// We checki if the currency rate was changed, if so, we will update it but also add a message/notify
// We get all the goals associated with the budget, their amount and surplus
// If the update is from Strict OFF to Strict ON, we check if the  total amount provided is enough
// to cover the goals, if not we throw an error.
// If the strict is OFF, we check if the total amount is enough to cover the goals, if so we allow the
// Update but add a message that the budget need change.
// Finally we update the budget in the database.
func (app *application) updateBudgetHandler(w http.ResponseWriter, r *http.Request) {
	var message = data.Warning_Messages
	var input struct {
		Name         *string          `json:"name"`
		IsStrict     *bool            `json:"is_strict"`
		Category     *string          `json:"category"`
		TotalAmount  *decimal.Decimal `json:"total_amount"`
		CurrencyCode *string          `json:"currency_code"`
		Description  *string          `json:"description"`
	}

	// Read budgetID parameter from the URL
	budgetID, err := app.readIDParam(r, "budgetID")
	if err != nil || budgetID < 1 {
		app.notFoundResponse(w, r)
		return
	}

	// Get budget details from the database
	budget, err := app.models.FinancialManager.GetBudgetByID(budgetID)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGeneralRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// Decode request body into the input struct
	err = app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	// Get the user from the context
	user := app.contextGetUser(r)

	// Retrieve the budget summary and total monthly contribution
	budgetSummary, budgetTotal, err := app.models.FinancialManager.GetAllGoalSummaryBudgetID(budget.Id, user.ID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// Initialize validator
	v := validator.New()

	// Check for changes in strictness
	// if strictness is provided and it's being changed
	if input.IsStrict != nil && *input.IsStrict != budget.IsStrict {
		// if we are moving from non-strict to strict
		if !budget.IsStrict && *input.IsStrict {
			// Ensure total amount is greater than or equal to the total monthly contribution
			if input.TotalAmount == nil {
				input.TotalAmount = &budget.TotalAmount
			}
			// if total amount is less than the total monthly contribution
			if input.TotalAmount.Cmp(budgetTotal.TotalMonthlyContribution) < 0 {
				v.AddError("total_amount", "total amount is less than the total goal contribution")
				v.AddError("is_strict", "strictness prevents the total amount from being less than the total goals")
				app.failedValidationResponse(w, r, v.Errors)
				return
			} else {
				message.Message = append(message.Message, "budget strictness changed from non-strict to strict")
			}
		} else if budget.IsStrict && !*input.IsStrict {
			// If strictness changed from true to false, allow the update but warn if the total amount is lower
			message.Message = append(message.Message, "budget strictness changed from strict to non-strict")
			if input.TotalAmount != nil && input.TotalAmount.Cmp(budgetTotal.TotalMonthlyContribution) < 0 {
				message.Message = append(message.Message, "total amount is lower than total goal contributions")
			}
		}
	}

	// Check for changes in total amount
	if input.TotalAmount != nil && input.TotalAmount.Cmp(budget.TotalAmount) != 0 {
		// If strictness is true, ensure the total amount is not less than the total monthly goal contribution
		if *input.IsStrict && input.TotalAmount.Cmp(budgetTotal.TotalMonthlyContribution) < 0 {
			v.AddError("total_amount", "total amount is less than the total goal contribution")
			app.failedValidationResponse(w, r, v.Errors)
			return
		}
		message.Message = append(message.Message, "budget total amount updated")
	}

	// Check for changes in other fields and update accordingly
	if input.Name != nil {
		budget.Name = *input.Name
	}
	if input.IsStrict != nil {
		budget.IsStrict = *input.IsStrict
	}
	if input.Category != nil {
		budget.Category = *input.Category
	}
	if input.TotalAmount != nil {
		budget.TotalAmount = *input.TotalAmount
	}
	if input.CurrencyCode != nil {
		budget.CurrencyCode = *input.CurrencyCode
	}
	if input.Description != nil {
		budget.Description = *input.Description
	}

	// Validate the updated budget
	if data.ValidateBudgetUpdate(v, budget); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	// Proceed with the budget update in the database
	err = app.models.FinancialManager.UpdateUserBudget(user.ID, budget)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrEditConflict):
			app.editConflictResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// Calculate new surplus after the budget update
	newSurplus := budget.TotalAmount.Sub(budgetTotal.TotalMonthlyContribution)
	if newSurplus.Cmp(decimal.Zero) < 0 {
		newSurplus = decimal.Zero // Ensure surplus doesn't go negative
	}

	// Save the new surplus to Redis
	if err := app.saveAndUpdateSurplus(user.ID, budgetID, newSurplus); err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// Return the updated budget with a 200 OK response
	err = app.writeJSON(w, http.StatusOK, envelope{"budget": budget, "message": message, "summary": budgetSummary}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// deleteBudgetHandler() is a handler function that handles the deletion of a Budget.
// We get the budgetID from the URL parameter and check if it is valid.
// if it is, we perform the deletion.
func (app *application) deleteBudgetByIDHandler(w http.ResponseWriter, r *http.Request) {
	// Read budgetID parameter from the URL
	budgetID, err := app.readIDParam(r, "budgetID")
	if err != nil || budgetID < 1 {
		app.notFoundResponse(w, r)
		return
	}

	// Get the user from the context
	user := app.contextGetUser(r)

	// Delete the budget from the database
	_, err = app.models.FinancialManager.DeleteBudgetByID(user.ID, budgetID)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGeneralRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// Prepare some custom messages
	deletionMessage := "budget deleted. goals that were under this budget will still be available but not tracked!"
	// Return a 200 OK response with the custom message
	err = app.writeJSON(w, http.StatusOK, envelope{"message": deletionMessage}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// getBudgetsForUserHandler() is a handler function that handles the retrieval of all budgets for a user.
// We get the user from the context and get all the budgets associated with the user.
// This route support pagination, filtering and search query via the name parameter.
// We return an enriched budget with summary details of the goals associated with the budget and subsequent totals.
func (app *application) getBudgetsForUserHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name string
		data.Filters
	}
	// Validate the query parameters
	v := validator.New()
	qs := r.URL.Query()
	// use our helpers to convert the queries
	input.Name = app.readString(qs, "name", "")
	//get the page & pagesizes as ints and set to the embedded struct
	input.Filters.Page = app.readInt(qs, "page", 1, v)
	input.Filters.PageSize = app.readInt(qs, "page_size", 20, v)
	// get the sort values falling back to "id" if it is not provided
	input.Filters.Sort = app.readString(qs, "sort", "name")
	// Add the supported sort values for this endpoint to the sort safelist.
	input.Filters.SortSafelist = []string{"name", "-url"}
	// Perform validation
	if data.ValidateFilters(v, input.Filters); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// Get the budgets for the user
	enrichedBudgets, metadata, err := app.models.FinancialManager.GetBudgetsForUser(app.contextGetUser(r).ID, input.Name, input.Filters)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	// Return the enriched budgets with a 200 OK response
	err = app.writeJSON(w, http.StatusOK, envelope{"budgets": enrichedBudgets, "metadata": metadata}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}

}

// ===================================================================================================================
// GOAL HANDLERS
// ===================================================================================================================

// createNewGoalHandler() is a handler function that handles the creation of a Goal.
// The methid validates the Input recieved from the user
// Checks done include whether the Target amount is greater than the current amount
// We also validate whether the goal is achievable within the dates provided
// A check is also done to see if the new goal's contribution is less than the available surplus
// allocated for that specific budget.
// If any of this evaluate to false, we return an error.
// Otherwise we return the created Goal in addition to a summary of existing goals.
func (app *application) createNewGoalHandler(w http.ResponseWriter, r *http.Request) {
	var message = data.Warning_Messages
	var input struct {
		BudgetID            int64           `json:"budget_id"`
		Name                string          `json:"name"`
		CurrentAmount       decimal.Decimal `json:"current_amount"`
		TargetAmount        decimal.Decimal `json:"target_amount"`
		MonthlyContribution decimal.Decimal `json:"monthly_contribution"`
		StartDate           time.Time       `json:"start_date"`
		EndDate             time.Time       `json:"end_date"`
		Status              string          `json:"status"`
	}

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	mappedStatus, err := app.models.FinancialManager.MapStatusToOCFConstant(input.Status)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	// make a goal from the input struct
	newGoal := &data.Goals{
		UserID:              app.contextGetUser(r).ID,
		BudgetID:            input.BudgetID,
		Name:                input.Name,
		CurrentAmount:       input.CurrentAmount,
		TargetAmount:        input.TargetAmount,
		MonthlyContribution: input.MonthlyContribution,
		StartDate:           input.StartDate,
		EndDate:             input.EndDate,
		Status:              mappedStatus,
	}
	// Perform validation
	v := validator.New()
	if data.ValidateGoal(v, newGoal); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// check if the goal is still within the budget
	goalSumary, goalSummaryTotals, err := app.models.FinancialManager.GetAllGoalSummaryBudgetID(newGoal.BudgetID, newGoal.UserID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	// if newGoal.MonthlyContribution is greater than the goalSummaryTotals.TotalMonthlyContribution
	// and the goalSumaryTotals.BudgetStrictness is true, we prevent the creation of the goal
	if goalSummaryTotals.TotalMonthlyContribution.Cmp(decimal.Zero) != 0 && newGoal.MonthlyContribution.Cmp(goalSummaryTotals.TotalMonthlyContribution) > 0 && goalSummaryTotals.BudgetStrictness {
		v.AddError("monthly_contribution", "monthly contribution is greater than the total surplus provisioned for this budget")
		app.failedValidationResponse(w, r, v.Errors)
		return
	} else if newGoal.MonthlyContribution.Cmp(goalSummaryTotals.TotalMonthlyContribution) > 0 && !goalSummaryTotals.BudgetStrictness {
		// if the goal is not strict, we allow the creation of the goal but add a message
		// that the budget needs to be updated
		message.Message = append(message.Message, "monthly contribution is greater than the total surplus provisioned. budget needs to be updated")
	}

	// just directly write to the database
	err = app.models.FinancialManager.CreateNewGoal(newGoal)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	oldSurplus, _ := app.getSurplus(newGoal.UserID, newGoal.BudgetID)
	app.logger.Info("New Surplus Amount from REDIS", zap.String("Surplus", oldSurplus.String()))
	// calculate the new surplus by simly substracting the new goal total monthly contribution from the surplus we had
	newSurplus := goalSummaryTotals.TotalSurplus.Sub(newGoal.MonthlyContribution)
	if newSurplus.Cmp(decimal.Zero) < 0 {
		newSurplus = decimal.Zero
	}
	// if eeverything is successful, we update the surplus in REDIS
	if err := app.saveAndUpdateSurplus(newGoal.UserID, newGoal.BudgetID, newSurplus); err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	// get the new goal summary
	redisSurplus, err := app.getSurplus(newGoal.UserID, newGoal.BudgetID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	// Check if TotalMonthlyContribution is zero and set goalSummary to an empty slice if it is
	// This is to prevent the response from returning a default value if no goals exist for the budget
	if goalSummaryTotals.TotalMonthlyContribution.Cmp(decimal.Zero) == 0 {
		goalSumary = []*data.Goal_Summary{}
	}
	app.logger.Info("New Surplus Amount from REDIS", zap.String("Surplus", redisSurplus.String()))
	err = app.writeJSON(w, http.StatusCreated, envelope{"goal": newGoal, "Existing Goal Summary": goalSumary}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}

}
