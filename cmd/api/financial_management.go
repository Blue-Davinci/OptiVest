package main

import (
	"context"
	"errors"
	"fmt"
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
	budgetTotal, err := app.models.FinancialManager.GetAllGoalSummaryBudgetID(budget.Id, user.ID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	app.logger.Info("Total Surplus", zap.String("Total Surplus", budgetTotal.TotalSurplus.String()))

	// Initialize validator
	v := validator.New()

	// Validation and update logic combined in one function
	totalUtilizedBudgetAmount := budget.TotalAmount.Sub(budgetTotal.TotalSurplus)

	// Validate strictness change
	if input.IsStrict != nil && *input.IsStrict != budget.IsStrict {
		if *input.IsStrict {
			// Moving from non-strict to strict
			if input.TotalAmount == nil {
				input.TotalAmount = &budget.TotalAmount
			}
			if input.TotalAmount.Cmp(totalUtilizedBudgetAmount) < 0 {
				v.AddError("total_amount", "total amount is less than the total goal contribution")
				v.AddError("is_strict", "strictness prevents the total amount from being less than the total goals")
				app.failedValidationResponse(w, r, v.Errors)
				return
			} else {
				message.Message = append(message.Message, "budget strictness changed from non-strict to strict")
			}
		} else {
			if input.TotalAmount.LessThan(totalUtilizedBudgetAmount) {
				// Moving from strict to non-strict
				message.Message = append(message.Message, "though allowed, your new budget amount is less than the total current expenses and goals")
			}
		}
	}

	// Validate total amount change
	if input.TotalAmount != nil && input.TotalAmount.Cmp(budget.TotalAmount) != 0 {
		if budget.IsStrict && input.TotalAmount.Cmp(totalUtilizedBudgetAmount) < 0 {
			v.AddError("total_amount", "total amount is less than the total goal contribution")
			v.AddError("is_strict", "strictness prevents the total amount from being less than the total goals")
			app.failedValidationResponse(w, r, v.Errors)
			return
		} else if input.TotalAmount.Cmp(totalUtilizedBudgetAmount) < 0 && !budget.IsStrict {
			message.Message = append(message.Message, "though allowed, your new budget total amount is less than the total goal and expense amounts")
		} else {
			message.Message = append(message.Message, "budget total amount updated")
		}
	}

	// Apply updates if provided
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

	// Update the budgetTotals before returning. Surplus is the same,
	// Just updating budget total amount and new surplus which is gonna be
	// budget total amount - totalUtilizedBudgetAmount
	budgetTotal.TotalBudgetAmount = budget.TotalAmount
	budgetTotal.TotalSurplus = budget.TotalAmount.Sub(totalUtilizedBudgetAmount)

	// Return the updated budget with a 200 OK response
	err = app.writeJSON(w, http.StatusOK, envelope{"budget": budget, "message": message, "totals": budgetTotal}, nil)
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
	input.Filters.PageSize = app.readInt(qs, "page_size", 10, v)
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
	// check if budget exists
	budget, err := app.models.FinancialManager.GetBudgetByID(input.BudgetID)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGeneralRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	mappedStatus, err := app.models.FinancialManager.MapStatusToOCFConstant(input.Status)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrInvalidOCFStatus):
			app.badRequestResponse(w, r, err)
		default:
			app.serverErrorResponse(w, r, err)
		}
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
	goalSummaryTotals, err := app.models.FinancialManager.GetAllGoalSummaryBudgetID(newGoal.BudgetID, newGoal.UserID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	// Check if the new goal's monthly contribution exceeds the available surplus
	if newGoal.MonthlyContribution.Cmp(goalSummaryTotals.TotalSurplus) > 0 {
		if budget.IsStrict {
			// Prevent the creation of the goal if the budget is strict
			v.AddError("monthly_contribution", "monthly contribution is greater than the available surplus for this budget")
			app.failedValidationResponse(w, r, v.Errors)
			return
		} else {
			// Add a warning message but allow the creation of the goal if the budget is not strict
			message.Message = append(message.Message, "monthly contribution exceeds the available surplus. Budget needs to be updated.")
		}
	}

	// just directly write to the database
	err = app.models.FinancialManager.CreateNewGoal(newGoal)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrDuplicateGoal):
			app.badRequestResponse(w, r, err)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	newSurplus := goalSummaryTotals.TotalSurplus.Sub(newGoal.MonthlyContribution)
	if newSurplus.Cmp(decimal.Zero) < 0 {
		newSurplus = decimal.Zero
	}
	// Update new data
	goalSummaryTotals.TotalSurplus = newSurplus
	goalSummaryTotals.TotalMonthlyContribution = goalSummaryTotals.TotalMonthlyContribution.Add(newGoal.MonthlyContribution)
	// Write the response
	err = app.writeJSON(w, http.StatusCreated, envelope{"goal": newGoal, "Totals": goalSummaryTotals}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}

}

// updatedGoalHandler() is a handler function that handles the updating of a Goal.
// When updating a Goal, we need to perform similar validation to when we are creating a goal.
// We check if the new monthly contribution is less than the available surplus for the budget and
// if the budget is strict, we prevent the update otherwise add a message
// We check if the Goal can still be achieved within the dates provided. If not, REJECT.
// We check if the current amount is less than the target amount if changed/altered, If not REJECT.
// If any of these checks fail, we return an error.
// Otherwise we update the goal and return the updated goal in the response body alongside available
// goal summaries and Update the new surplus in REDIS
// format: /goals/{goalID}
func (app *application) updatedGoalHandler(w http.ResponseWriter, r *http.Request) {
	var message = data.Warning_Messages
	var input struct {
		BudgetID            *int64           `json:"budget_id"`
		Name                *string          `json:"name"`
		CurrentAmount       *decimal.Decimal `json:"current_amount"`
		TargetAmount        *decimal.Decimal `json:"target_amount"`
		MonthlyContribution *decimal.Decimal `json:"monthly_contribution"`
		StartDate           *time.Time       `json:"start_date"`
		EndDate             *time.Time       `json:"end_date"`
		Status              *string          `json:"status"`
	}
	// Read goalID parameter from the URL
	goalID, err := app.readIDParam(r, "goalID")
	if err != nil || goalID < 1 {
		app.notFoundResponse(w, r)
		return
	}
	// get user
	user := app.contextGetUser(r)
	// Get the goal details from the database
	goal, err := app.models.FinancialManager.GetGoalByID(user.ID, goalID)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGeneralRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// check if budget exists
	budget, err := app.models.FinancialManager.GetBudgetByID(goal.BudgetID)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGeneralRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// We found the Goal with the user ID, Decode request body into the input struct
	err = app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	// Get the budget summary and total monthly contribution
	budgetTotal, err := app.models.FinancialManager.GetAllGoalSummaryBudgetID(goal.BudgetID, user.ID)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGeneralRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// Initialize validator
	v := validator.New()
	// Step 1: Subtract the current goal's monthly contribution to get the correct starting surplus
	//totalContributionExcludingCurrentGoal := budgetTotal.TotalMonthlyContribution.Sub(goal.MonthlyContribution)
	app.logger.Info("Total redcieved surplus", zap.String("Tota Surplus", budgetTotal.TotalSurplus.String()), zap.String("Monthly Contribution", goal.MonthlyContribution.String()))
	availableSurplus := budgetTotal.TotalSurplus.Add(goal.MonthlyContribution)
	app.logger.Info("Available Surplus", zap.String("Available Surplus", availableSurplus.String()))
	// Step 2: Initialize newTotalSurplus with the available surplus
	newTotalSurplus := availableSurplus

	// Step 3: Check if the monthly contribution is being updated
	if input.MonthlyContribution != nil && input.MonthlyContribution.Cmp(goal.MonthlyContribution) != 0 {

		// Step 4: Check if the new contribution exceeds the available surplus
		if budget.IsStrict && input.MonthlyContribution.Cmp(availableSurplus) > 0 {
			v.AddError("monthly_contribution", "monthly contribution is greater than the total surplus provisioned for this budget")
			app.failedValidationResponse(w, r, v.Errors)
			return
		} else if input.MonthlyContribution.Cmp(availableSurplus) > 0 && !budget.IsStrict {
			// Allow the update but add a warning message
			message.Message = append(message.Message, "monthly contribution is greater than the total surplus provisioned. budget needs to be updated")
		}
		newTotalSurplus = availableSurplus.Sub(*input.MonthlyContribution)
	} else {
		// If the contribution is NOT updated, keep the current surplus
		newTotalSurplus = budgetTotal.TotalSurplus
	}
	// Check for changes in other fields and update accordingly
	if input.Name != nil {
		goal.Name = *input.Name
	}
	if input.CurrentAmount != nil {
		goal.CurrentAmount = *input.CurrentAmount
	}
	if input.TargetAmount != nil {
		goal.TargetAmount = *input.TargetAmount
	}
	if input.MonthlyContribution != nil {
		goal.MonthlyContribution = *input.MonthlyContribution
	}
	if input.StartDate != nil {
		goal.StartDate = *input.StartDate
	}
	if input.EndDate != nil {
		goal.EndDate = *input.EndDate
	}
	if input.Status != nil {
		mappedStatus, err := app.models.FinancialManager.MapStatusToOCFConstant(*input.Status)
		if err != nil {
			app.badRequestResponse(w, r, data.ErrInvalidOCFStatus)
			return
		}
		goal.Status = mappedStatus
	}
	// Validate the updated goal
	if data.ValidateGoal(v, goal); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// Proceed with the goal update in the database
	err = app.models.FinancialManager.UpdateGoalByID(user.ID, goal)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrEditConflict):
			app.editConflictResponse(w, r)
		case errors.Is(err, data.ErrDuplicateGoal):
			app.badRequestResponse(w, r, err)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// Set totals to new surplus
	budgetTotal.TotalSurplus = newTotalSurplus

	app.logger.Info("New Surplus Amount from REDIS", zap.String("Surplus", budgetTotal.TotalSurplus.String()))
	// Return the updated budget with a 200 OK response
	err = app.writeJSON(w, http.StatusOK, envelope{"goal": goal, "message": message, "totals": budgetTotal}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// getGoalTrackingHistoryHandler() is a handler function that handles the retrieval of the tracking history of a goal.
// This route supports pagination, a goal tracking search name
// and returns a list of goal tracking history for a specific user ID
func (app *application) getGoalTrackingHistoryHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name string
		data.Filters
	}
	// Validate the query parameters
	v := validator.New()
	qs := r.URL.Query()
	// use our helpers to convert the queries
	input.Name = app.readString(qs, "tracking_type", "monthly")
	//get the page & pagesizes as ints and set to the embedded struct
	input.Filters.Page = app.readInt(qs, "page", 1, v)
	input.Filters.PageSize = app.readInt(qs, "page_size", 10, v)
	// get the sort values falling back to "id" if it is not provided
	input.Filters.Sort = app.readString(qs, "sort", "id")
	// Add the supported sort values for this endpoint to the sort safelist.
	input.Filters.SortSafelist = []string{"id", "-id"}
	// Perform validation
	if data.ValidateFilters(v, input.Filters); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// Get the user from the context
	user := app.contextGetUser(r)
	// convert the tracking type to the OCF constant
	mappedTrackingType, err := app.models.FinancialManager.MapTrackingTypeToConstant(input.Name)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrInvalidOCFStatus):
			app.badRequestResponse(w, r, err)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// Get the goal tracking history for the user
	goalTrackingHistory, metadata, err := app.models.FinancialManager.GetGoalTrackingHistory(user.ID, mappedTrackingType, input.Filters)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	// Return the goal tracking history with a 200 OK response
	err = app.writeJSON(w, http.StatusOK, envelope{"goal_tracking_history": goalTrackingHistory, "metadata": metadata}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}

}

// createNewGoalPlanHandler() is a handler function that handles the creation of a Goal Plan.
// This essentially works as a plan "template" for a goal.
// We validate minimally and just save the plan to the database.
func (app *application) createNewGoalPlanHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name                string          `json:"name"`
		Description         string          `json:"description"`
		TargetAmount        decimal.Decimal `json:"target_amount"`
		MonthlyContribution decimal.Decimal `json:"monthly_contribution"`
		DurationInMonths    int             `json:"duration_in_months"`
		IsStrict            bool            `json:"is_strict"`
	}
	// Decode the request body into the input struct
	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	// Create a new Goal Plan struct and fill it with the data from the input struct
	newGoalPlan := &data.GoalPlan{
		Name:                input.Name,
		Description:         input.Description,
		TargetAmount:        input.TargetAmount,
		MonthlyContribution: input.MonthlyContribution,
		DurationInMonths:    input.DurationInMonths,
		IsStrict:            input.IsStrict,
	}
	// Perform validation
	v := validator.New()
	if data.ValidateGoalPlan(v, newGoalPlan); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// Save the goal plan to the database
	err = app.models.FinancialManager.CreateNewGoalPlan(app.contextGetUser(r).ID, newGoalPlan)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrDuplicateGoalPlan):
			app.badRequestResponse(w, r, err)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// Return a 201 Created status code along with the goal plan in the response body
	err = app.writeJSON(w, http.StatusCreated, envelope{"goal_plan": newGoalPlan}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// updatedGoalPlanHandler() is a handler function that handles the updating of a Goal Plan.
// We validate the input and update the goal plan in the database.
func (app *application) updatedGoalPlanHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name                *string          `json:"name"`
		Description         *string          `json:"description"`
		TargetAmount        *decimal.Decimal `json:"target_amount"`
		MonthlyContribution *decimal.Decimal `json:"monthly_contribution"`
		DurationInMonths    *int             `json:"duration_in_months"`
		IsStrict            *bool            `json:"is_strict"`
	}
	// Read goalPlanID parameter from the URL
	goalPlanID, err := app.readIDParam(r, "goalPlanID")
	if err != nil || goalPlanID < 1 {
		app.notFoundResponse(w, r)
		return
	}
	// Get the goal plan details from the database
	goalPlan, err := app.models.FinancialManager.GetGoalPlanByID(app.contextGetUser(r).ID, goalPlanID)
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
	// Check for changes in other fields and update accordingly
	if input.Name != nil {
		goalPlan.Name = *input.Name
	}
	if input.Description != nil {
		goalPlan.Description = *input.Description
	}
	if input.TargetAmount != nil {
		goalPlan.TargetAmount = *input.TargetAmount
	}
	if input.MonthlyContribution != nil {
		goalPlan.MonthlyContribution = *input.MonthlyContribution
	}
	if input.DurationInMonths != nil {
		goalPlan.DurationInMonths = *input.DurationInMonths
	}
	if input.IsStrict != nil {
		goalPlan.IsStrict = *input.IsStrict
	}
	// Validate the updated goal plan
	v := validator.New()
	if data.ValidateGoalPlan(v, goalPlan); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// Proceed with the goal plan update in the database
	err = app.models.FinancialManager.UpdateGoalPlanByID(app.contextGetUser(r).ID, goalPlan)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrEditConflict):
			app.editConflictResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// Return the updated goal plan with a 200 OK response
	err = app.writeJSON(w, http.StatusOK, envelope{"goal_plan": goalPlan}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// getGoalPlansForUserHandler() is a handler function that handles the retrieval of all goal plans for a user.
// We first check if the goalplans are cached in REDIS using getSerializedCachedData(), if they are we return them.
// Otherwise we get the goal plans from the database and cache them in REDIS using cacheSerializedData().
func (app *application) getGoalPlansForUserHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		data.Filters
	}
	// Validate if queries are provided
	v := validator.New()

	// Call r.URL.Query() to get the url.Values map containing the query string data.
	qs := r.URL.Query()

	// Get the pagesizes as ints and set to the embedded struct
	input.Filters.Page = app.readInt(qs, "page", 1, v)
	input.Filters.PageSize = app.readInt(qs, "page_size", 5, v)

	// Get the sort values falling back to "id" if it is not provided
	input.Filters.Sort = app.readString(qs, "sort", "id")

	// Add the supported sort values for this endpoint to the sort safelist.
	input.Filters.SortSafelist = []string{"id", "-id"}

	// Perform validation
	if data.ValidateFilters(v, input.Filters); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	// Get the user
	user := app.contextGetUser(r)

	// Check if the goal plans are cached in REDIS
	redisKey := fmt.Sprintf("%s%d:%d", data.RedisFinManGoalPlanPrefix, user.ID, input.Filters.Page)

	// Initialize unifiedGoalPlans to avoid nil pointer issues
	unifiedGoalPlans := &data.UnifiedGoalPlanMetadata{}

	// Attempt to retrieve cached data
	cached, err := getFromCache[*data.UnifiedGoalPlanMetadata](context.Background(), app.RedisDB, redisKey)
	if err != nil {
		switch {
		case errors.Is(err, ErrNoDataFoundInRedis):
			// Do nothing
		default:
			app.serverErrorResponse(w, r, err)
			return
		}
	}
	if cached != nil {
		// Return cached data if available
		err = app.writeJSON(w, http.StatusOK, envelope{"goal_plans": unifiedGoalPlans}, nil)
		if err != nil {
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// Get the goal plans for the user
	goalPlans, metadata, err := app.models.FinancialManager.GetGoalPlansForUser(user.ID, input.Filters)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// Create a unified goal plan metadata
	unifiedGoalPlans = &data.UnifiedGoalPlanMetadata{
		GoalPlan: goalPlans,
		Metadata: metadata,
	}

	// Cache the goal plans in REDIS
	err = setToCache(context.Background(), app.RedisDB, redisKey, unifiedGoalPlans, 12*time.Hour)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// Return the goal plans with a 200 OK response
	err = app.writeJSON(w, http.StatusOK, envelope{"goal_plans": unifiedGoalPlans}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// gtBudgetGoalExpenseSummaryHandler() is a handler function that handles the retrieval of all goals and expenses
// for all budgets for a user.
// We get the user from the context and get all the goals and expenses associated with the user.
func (app *application) getBudgetGoalExpenseSummaryHandler(w http.ResponseWriter, r *http.Request) {
	// Get the user from the context
	user := app.contextGetUser(r)

	// Get the goals and expenses for the user
	enrichedBudgets, err := app.models.FinancialManager.GetBudgetGoalExpenseSummary(user.ID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// Return the goals and expenses with a 200 OK response
	err = app.writeJSON(w, http.StatusOK, envelope{"enriched_budgets": enrichedBudgets}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// getAllGoalsWithProgressionByUserIDHandler() is a handler function that handles the retrieval of all goals with progression
// for a user.
// We get the user from the context and get all the goals with progression associated with the user.
// This route supports pagination, filtering and search query via the name parameter.
func (app *application) getAllGoalsWithProgressionByUserIDHandler(w http.ResponseWriter, r *http.Request) {
	// input var
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
	input.Filters.PageSize = app.readInt(qs, "page_size", 10, v)
	// get the sort values falling back to "id" if it is not provided
	input.Filters.Sort = app.readString(qs, "sort", "name")
	// Add the supported sort values for this endpoint to the sort safelist.
	input.Filters.SortSafelist = []string{"name", "-url"}
	// Perform validation
	if data.ValidateFilters(v, input.Filters); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// Get the user from the context
	user := app.contextGetUser(r)

	// Get the goals with progression for the user
	goals, metadata, err := app.models.FinancialManager.GetAllGoalsWithProgressionByUserID(user.ID, input.Name, input.Filters)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGeneralRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// Return the goals with a 200 OK response
	err = app.writeJSON(w, http.StatusOK, envelope{"goals": goals, "metadata": metadata}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

//
