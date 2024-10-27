package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/Blue-Davinci/OptiVest/internal/data"
	"github.com/Blue-Davinci/OptiVest/internal/database"
	"github.com/Blue-Davinci/OptiVest/internal/validator"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// createNewExpenseHandler() creates a new one way/ none recurring expense to the database
// We still verify if the budget exists, if it does not, we return an error
// We then check if the amount of the expense is more than the surplus, if it is, we return an error only if the budget is strict
// If the budget is not strict, we add a message to the response and proceed with the save
func (app *application) createNewExpenseHandler(w http.ResponseWriter, r *http.Request) {
	message := data.Warning_Messages
	var input struct {
		BudgetID    int64           `json:"budget_id"`
		Name        string          `json:"name"`
		Category    string          `json:"category"`
		Amount      decimal.Decimal `json:"amount"`
		Description string          `json:"description"`
		DateOcurred time.Time       `json:"date_occurred"`
	}
	// read the request body into the input struct
	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	// get the user
	user := app.contextGetUser(r)
	// get the budget
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
	// create a new expense
	expense := &data.Expense{
		UserID:       user.ID,
		BudgetID:     input.BudgetID,
		Name:         input.Name,
		Category:     input.Category,
		Amount:       input.Amount,
		IsRecurring:  false,
		Description:  input.Description,
		DateOccurred: input.DateOcurred,
	}
	// create a validator
	v := validator.New()
	// validate the expense
	if data.ValidateExpense(v, expense); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// get the available surplus
	goalTotals, err := app.models.FinancialManager.GetAllGoalSummaryBudgetID(expense.BudgetID, user.ID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	// check if the expense is more than the surplus
	if expense.Amount.Cmp(goalTotals.TotalSurplus) > 0 {
		if budget.IsStrict {
			v.AddError("amount", "expense amount is more than the available surplus")
			app.failedValidationResponse(w, r, v.Errors)
			return
		} else {
			// add a message to the response
			message.Message = append(message.Message, "expense amount is more than the available surplus")
		}
	}
	// save the expense
	err = app.models.FinancialTrackingManager.CreateNewExpense(user.ID, expense)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	// send the response
	err = app.writeJSON(w, http.StatusCreated, envelope{"expense": expense, "message": message, "totals": goalTotals}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}

}

// updateExpenseByIDHandler() is a handler method that will update an expense in the database
// We check if the budget exists, if it does not, we return an error
// We check if the expense exists, if it does not, we return an error
// We check if the amount has changed, if it has, we check if the new amount is more than the total surplus - old amount
// If the amount is more than the surplus and the budget is strict, we return an error
// If the budget is not strict, we add a message to the response and proceed with the save
// We validate the expense and update it in the database
// updateExpenseByIDHandler() is a handler method that will update an expense in the database
func (app *application) updateExpenseByIDHandler(w http.ResponseWriter, r *http.Request) {
	var message = data.Warning_Messages
	var input struct {
		Amount      *decimal.Decimal `json:"amount"`
		Name        *string          `json:"name"`
		Category    *string          `json:"category"`
		Description *string          `json:"description"`
		DateOcurred *time.Time       `json:"date_occurred"`
	}

	// get the expense ID from the url
	expenseID, err := app.readIDParam(r, "expenseID")
	if err != nil || expenseID < 1 {
		app.notFoundResponse(w, r)
		return
	}

	// read the request body into the input struct
	err = app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	// get the user
	user := app.contextGetUser(r)

	// get the expense
	expense, err := app.models.FinancialTrackingManager.GetExpenseByID(user.ID, expenseID)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGeneralRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// get the budget
	budget, err := app.models.FinancialManager.GetBudgetByID(expense.BudgetID)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGeneralRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// get the available surplus (this includes the current expense)
	goalTotals, err := app.models.FinancialManager.GetAllGoalSummaryBudgetID(expense.BudgetID, user.ID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// 1. Subtract the current expense amount from the surplus
	//    (i.e., pretend the current expense doesn't exist for a moment)
	currentSurplus := goalTotals.TotalSurplus.Add(expense.Amount)

	// 2. If the amount has changed, check the new amount against the adjusted surplus
	if input.Amount != nil {
		newAmount := *input.Amount

		// If the new amount is larger than the available surplus
		if newAmount.GreaterThan(currentSurplus) {
			// If the budget is strict, return an error
			if budget.IsStrict {
				app.errorResponse(w, r, http.StatusForbidden, "Budget surplus is insufficient for this expense.")
				return
			} else {
				// Otherwise, proceed but log a warning
				message.Message = append(message.Message, "The expense exceeds the available surplus, but the budget is not strict.")
			}
		}

		// Set the new amount
		expense.Amount = newAmount
	}

	// 3. Update other fields if provided
	if input.Category != nil {
		expense.Category = *input.Category
	}
	if input.Name != nil {
		expense.Name = *input.Name
	}
	if input.Description != nil {
		expense.Description = *input.Description
	}
	if input.DateOcurred != nil {
		expense.DateOccurred = *input.DateOcurred
	}

	// 4. Validate the expense before saving
	v := validator.New()
	if data.ValidateExpense(v, expense); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	// 5. Save the updated expense to the database
	err = app.models.FinancialTrackingManager.UpdateExpenseByID(user.ID, expense)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// 6. Respond with success
	err = app.writeJSON(w, http.StatusOK, envelope{"expense": expense, "warnings": message}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// createNewRecurringExpenseHandler() is an handler method that will add a recurring expense to the database
// Expenses are tied to a badget. A badget is either strict or not. If it is strict, the user cannot spend more than the budget.
// We first check if the budget exists, if it does not, we return an error
// For recurring expenses, we will need to calculate the total amount of the expense per month
// If it is a recurring expenses that is not monthly, recurringExpense.CalculateTotalAmountPerMonth() will be used to calculate the total amount
// If it is a monthly expense, the amount will be the same
// We will then retrieve the available surplus, if the calculated recurrent amount is more than the surplus, we will return an error only if the budget is strict
// If the budget is not strict, we will add the expense to the database but add a message to the response
func (app *application) createNewRecurringExpenseHandler(w http.ResponseWriter, r *http.Request) {
	var message = data.Warning_Messages
	// make the input struct for what we will require in a recurrent expense
	var input struct {
		BudgetID           int64           `json:"budget_id"`
		Amount             decimal.Decimal `json:"amount"`
		Name               string          `json:"name"`
		Description        string          `json:"description"`
		RecurrenceInterval string          `json:"recurrence_interval"`
	}
	// Decode the request body into the input struct
	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	// Map the recurrence interval to the database enum
	recurrenceInterval, err := app.models.FinancialTrackingManager.MapToDatabaseRecurringExpense(input.RecurrenceInterval)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrInvalidRecurringExpenseTime):
			app.badRequestResponse(w, r, err)
		default:
			app.serverErrorResponse(w, r, err)
		}
	}

	// Next Recurrence we will need to calculate
	// Get budget details from the database
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
	// Create a new recurring expense
	recurringExpense := &data.RecurringExpense{
		BudgetID:           input.BudgetID,
		Amount:             input.Amount,
		Name:               input.Name,
		Description:        input.Description,
		RecurrenceInterval: recurrenceInterval,
	}
	// validate the recurring expense
	v := validator.New()
	if data.ValidateRecurringExpense(v, recurringExpense); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// get a user
	user := app.contextGetUser(r)
	// calculate amount of the expense per month based on the recurrence interval
	recurringExpense.ProjectedAmount = recurringExpense.CalculateTotalAmountPerMonth()
	// Get our totals
	goalTotals, err := app.models.FinancialManager.GetAllGoalSummaryBudgetID(recurringExpense.BudgetID, user.ID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	// check if the new expense is more than the surplus
	if recurringExpense.ProjectedAmount.Cmp(goalTotals.TotalSurplus) > 0 {
		// check strictness of the budget
		if budget.IsStrict {
			v.AddError("amount", "recurring expense amount is more than the available surplus")
			app.failedValidationResponse(w, r, v.Errors)
			return
		} else {
			message.Message = append(message.Message, "recurring expense amount is more than the available surplus")
		}
	}
	// safe, calculate the next occurrence
	recurringExpense.CalculateNextOccurrence()
	app.logger.Info("next occurrence", zap.String("next_occurrence", recurringExpense.NextOccurrence.String()))
	// Create the recurring expense
	err = app.models.FinancialTrackingManager.CreateNewRecurringExpense(user.ID, recurringExpense)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrDuplicateRecurringExpense):
			v.AddError("amount", "recurring expense already exists")
			app.failedValidationResponse(w, r, v.Errors)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// determine next occurrence

	// Send the response
	err = app.writeJSON(w, http.StatusCreated, envelope{"expense": recurringExpense, "totals": goalTotals}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}

}

// updateRecurringExpenseByIDHandler() is a handler method that will update a recurring expense in the database
// We check if the budget exists, if it does not, we return an error
// We check if the recurring expense exists, if it does not, we return an error
// We check if the amount or frequency was changed,  IF the frequency was changed, we calculate the new total amount
// and set it to the new amount.
// if the amount was changed, we check if the new amount is more than the (surplus - old amount)
// if it was and the budget is strict, we return an error
// if the budget is not strict, we add a message to the response and proceed with the save
func (app *application) updateRecurringExpenseByIDHandler(w http.ResponseWriter, r *http.Request) {
	var message = data.Warning_Messages
	var input struct {
		Amount             *decimal.Decimal `json:"amount"`
		Name               *string          `json:"name"`
		Description        *string          `json:"description"`
		RecurrenceInterval *string          `json:"recurrence_interval"`
	}

	expenseID, err := app.readIDParam(r, "expenseID")
	if err != nil || expenseID < 1 {
		app.notFoundResponse(w, r)
		return
	}

	err = app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	user := app.contextGetUser(r)
	recurringExpense, err := app.models.FinancialTrackingManager.GetRecurringExpenseByID(user.ID, expenseID)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGeneralRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// print created at
	app.logger.Info("created at", zap.String("created_at", recurringExpense.CreatedAt.String()))

	budget, err := app.models.FinancialManager.GetBudgetByID(recurringExpense.BudgetID)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGeneralRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	var recurrenceInterval database.RecurrenceIntervalEnum
	if input.RecurrenceInterval != nil {
		var err error
		recurrenceInterval, err = app.models.FinancialTrackingManager.MapToDatabaseRecurringExpense(*input.RecurrenceInterval)
		if err != nil {
			switch {
			case errors.Is(err, data.ErrInvalidRecurringExpenseTime):
				app.badRequestResponse(w, r, err)
			default:
				app.serverErrorResponse(w, r, err)
			}
			return
		}
	} else {
		recurrenceInterval = recurringExpense.RecurrenceInterval
	}

	// Create a validator
	v := validator.New()

	// Calculate the old total projected amount
	oldProjectedAmount := recurringExpense.CalculateTotalAmountPerMonth()

	// Update the expense amount if provided
	if input.Amount != nil {
		recurringExpense.Amount = *input.Amount
	}

	// Update recurrence interval if it's provided and changed
	if input.RecurrenceInterval != nil && *input.RecurrenceInterval != string(recurringExpense.RecurrenceInterval) {
		recurringExpense.RecurrenceInterval = recurrenceInterval
	}

	// Calculate the new projected amount
	newProjectedAmount := recurringExpense.CalculateTotalAmountPerMonth()
	// print
	app.logger.Info("new projected amount", zap.String("new_projected_amount", newProjectedAmount.String()))

	// Get available surplus
	goalTotals, err := app.models.FinancialManager.GetAllGoalSummaryBudgetID(recurringExpense.BudgetID, user.ID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	// print out hte surplus
	app.logger.Info("surplus", zap.String("surplus", goalTotals.TotalSurplus.String()))
	// ADD THE OLD projected amount to the surplus effectively nullifying the old expense
	newTotalSurplus := goalTotals.TotalSurplus.Add(oldProjectedAmount)
	// print
	app.logger.Info("new total surplus", zap.String("new_total_surplus", newTotalSurplus.String()))
	if newTotalSurplus.Cmp(newProjectedAmount) < 0 {
		if budget.IsStrict {
			v.AddError("amount", "recurring expense amount is more than the available surplus")
			app.failedValidationResponse(w, r, v.Errors)
			return
		} else {
			message.Message = append(message.Message, "recurring expense amount is more than the available surplus")
		}
	}

	// Update next occurrence
	recurringExpense.CalculateNextOccurrence()
	// print next occurrence
	app.logger.Info("next occurrence", zap.String("next_occurrence", recurringExpense.NextOccurrence.String()))
	// validate recurring expense
	if data.ValidateRecurringExpense(v, recurringExpense); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// Save the updated recurring expense
	err = app.models.FinancialTrackingManager.UpdateRecurringExpenseByID(user.ID, recurringExpense)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// Set the projected amount
	recurringExpense.ProjectedAmount = recurringExpense.CalculateTotalAmountPerMonth()

	// Send the response
	err = app.writeJSON(w, http.StatusOK, envelope{"expense": recurringExpense, "totals": goalTotals}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// getAllExpensesByUserIDHandler() is a handler method that will return all expenses for a user
// This route supports pagination as well as a name search parameter for the expense's name
func (app *application) getAllExpensesByUserIDHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name string
		data.Filters
	}
	//validate if queries are provided
	v := validator.New()
	// Call r.URL.Query() to get the url.Values map containing the query string data.
	qs := r.URL.Query()
	//get the page & pagesizes as ints and set to the embedded struct
	input.Name = app.readString(qs, "name", "")
	input.Filters.Page = app.readInt(qs, "page", 1, v)
	input.Filters.PageSize = app.readInt(qs, "page_size", 20, v)
	// get the sort values falling back to "created_at" if it is not provided
	input.Filters.Sort = app.readString(qs, "sort", "created_at")
	// Add the supported sort values for this endpoint to the sort safelist.
	input.Filters.SortSafelist = []string{"created_at", "-created_at"}
	// Perform validation
	if data.ValidateFilters(v, input.Filters); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// get our expenses
	expenses, metadata, err := app.models.FinancialTrackingManager.GetAllExpensesByUserID(app.contextGetUser(r).ID, input.Name, input.Filters)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGeneralRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// send the response
	err = app.writeJSON(w, http.StatusOK, envelope{"expenses": expenses, "metadata": metadata}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}

}

// getAllRecurringExpensesByUserIDHandler() is a handler method that will return all recurring expenses for a user
// This route supports pagination as well as a name search parameter for the expense's name
func (app *application) getAllRecurringExpensesByUserIDHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name string
		data.Filters
	}
	//validate if queries are provided
	v := validator.New()
	// Call r.URL.Query() to get the url.Values map containing the query string data.
	qs := r.URL.Query()
	//get the page & pagesizes as ints and set to the embedded struct
	input.Name = app.readString(qs, "name", "")
	input.Filters.Page = app.readInt(qs, "page", 1, v)
	input.Filters.PageSize = app.readInt(qs, "page_size", 20, v)
	// get the sort values falling back to "created_at" if it is not provided
	input.Filters.Sort = app.readString(qs, "sort", "created_at")
	// Add the supported sort values for this endpoint to the sort safelist.
	input.Filters.SortSafelist = []string{"created_at", "-created_at"}
	// Perform validation
	if data.ValidateFilters(v, input.Filters); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// get our expenses
	recurringExpenses, metadata, err := app.models.FinancialTrackingManager.GetAllRecurringExpensesByUserID(app.contextGetUser(r).ID, input.Name, input.Filters)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGeneralRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// send the response
	err = app.writeJSON(w, http.StatusOK, envelope{"recurring_expenses": recurringExpenses, "metadata": metadata}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// createNewIncomeHandler() creates a new income for a user
// This route supports multi-currency data.
// We first check if the ORIGINAL currency provided is the same as the user's default currency
// If it is not, then we will check Via REDIS if the provided currency is supported
// If it is, we will convert the amount to the user's default currency, If it is not, we will return an error
// We than validate the income and save it to the database.
func (app *application) createNewIncomeHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Source       string           `json:"source"`
		CurrencyCode string           `json:"currency_code"`
		Amount       decimal.Decimal  `json:"amount_original"`
		Description  string           `json:"description"`
		DateReceived data.CustomTime1 `json:"date_received"`
	}
	// read the request body into the input struct
	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	// get the user
	user := app.contextGetUser(r)
	// make a new income
	income := &data.Income{
		UserID:               user.ID,
		Source:               input.Source,
		OriginalCurrencyCode: input.CurrencyCode,
		AmountOriginal:       input.Amount,
		Description:          input.Description,
		DateReceived:         input.DateReceived.Time,
	}
	// create a validator
	v := validator.New()
	// check if the currency is the users default currency
	if user.CurrencyCode != income.OriginalCurrencyCode {
		// check currrncy code is supported
		if err := app.verifyCurrencyInRedis(income.OriginalCurrencyCode); err != nil {
			v.AddError("currency_code", "currency code not supported")
			app.failedValidationResponse(w, r, v.Errors)
			return
		}
		// convert the amount to the user's default currency
		convertedAmount, err := app.convertAndGetExchangeRate(income.OriginalCurrencyCode, user.CurrencyCode)
		if err != nil {
			v.AddError("currency_code", "could not convert currency")
			app.failedValidationResponse(w, r, v.Errors)
			return
		}
		// set amount and exchange rate
		income.Amount = convertedAmount.ConvertAmount(income.AmountOriginal).ConvertedAmount
		income.ExchangeRate = convertedAmount.ConversionRate
		app.logger.Info("converted amount", zap.String("converted_amount", income.Amount.String()))
		app.logger.Info("exchange rate", zap.String("exchange_rate", income.ExchangeRate.String()))
	} else {
		income.Amount = income.AmountOriginal
		income.ExchangeRate = decimal.NewFromInt(1)
	}
	// validate the income
	if data.ValidateIncome(v, income); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// save the income
	err = app.models.FinancialTrackingManager.CreateNewIncome(user.ID, income)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	// send the response
	err = app.writeJSON(w, http.StatusCreated, envelope{"income": income}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}

}

// getAllIncomesByUserIDHandler() is a handler method that will return all incomes for a user
// This route supports pagination as well as a source search parameter for the income's source
func (app *application) getAllIncomesByUserIDHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name string
		data.Filters
	}
	//validate if queries are provided
	v := validator.New()
	// Call r.URL.Query() to get the url.Values map containing the query string data.
	qs := r.URL.Query()
	//get the page & pagesizes as ints and set to the embedded struct
	input.Name = app.readString(qs, "name", "")
	input.Filters.Page = app.readInt(qs, "page", 1, v)
	input.Filters.PageSize = app.readInt(qs, "page_size", 20, v)
	// get the sort values falling back to "created_at" if it is not provided
	input.Filters.Sort = app.readString(qs, "sort", "created_at")
	// Add the supported sort values for this endpoint to the sort safelist.
	input.Filters.SortSafelist = []string{"created_at", "-created_at"}
	// Perform validation
	if data.ValidateFilters(v, input.Filters); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// get our incomes
	incomes, metadata, err := app.models.FinancialTrackingManager.GetAllIncomesByUserID(app.contextGetUser(r).ID, input.Name, input.Filters)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGeneralRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// send the response
	err = app.writeJSON(w, http.StatusOK, envelope{"incomes": incomes, "metadata": metadata}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// updateIncomeHandler() Updates an existing Income
// User must provide the income ID in the url query
// We first check if the Income exists, if it does not, we return an error
// Since it was saved in the users default currency BUT it was converted from the OriginalCurrencyCode
// We check if that specific currency was changed. If the new currency is different from the original currency
// we need to reconvert it to the users defaultcurrency, saving the new exchange rate as well otherwise we just update the income
// We validate the income and save it to the database
// updateIncomeHandler updates an existing income entry.
// updateIncomeHandler updates an existing income entry.
func (app *application) updateIncomeHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Source       *string          `json:"source"`
		CurrencyCode *string          `json:"currency_code"`
		Amount       *decimal.Decimal `json:"amount_original"`
		Description  *string          `json:"description"`
		DateReceived *time.Time       `json:"date_received"`
	}

	// Get the income ID from the URL.
	incomeID, err := app.readIDParam(r, "incomeID")
	if err != nil || incomeID < 1 {
		app.notFoundResponse(w, r)
		return
	}

	// Read the request body into the input struct.
	err = app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	// Get the user from the request context.
	user := app.contextGetUser(r)

	// Fetch the existing income entry from the database.
	income, err := app.models.FinancialTrackingManager.GetIncomeByID(user.ID, incomeID)
	if err != nil {
		if errors.Is(err, data.ErrGeneralRecordNotFound) {
			app.notFoundResponse(w, r)
		} else {
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// Create a validator instance.
	v := validator.New()

	// Determine if the currency code or amount has been updated.
	if input.CurrencyCode != nil && input.Amount != nil {
		// If both CurrencyCode and Amount are provided, use the new CurrencyCode for conversion.
		newCurrencyCode := *input.CurrencyCode

		// Ensure the new currency is supported.
		if err := app.verifyCurrencyInRedis(newCurrencyCode); err != nil {
			v.AddError("currency_code", "currency code not supported")
			app.failedValidationResponse(w, r, v.Errors)
			return
		}

		// Convert the provided amount in the new currency to the user's default currency.
		convertedAmount, err := app.convertAndGetExchangeRate(newCurrencyCode, user.CurrencyCode)
		if err != nil {
			v.AddError("currency_code", "could not convert currency")
			app.failedValidationResponse(w, r, v.Errors)
			return
		}

		// Update the income amount and exchange rate based on the new conversion.
		income.Amount = convertedAmount.ConvertAmount(*input.Amount).ConvertedAmount
		income.ExchangeRate = convertedAmount.ConversionRate
		income.OriginalCurrencyCode = newCurrencyCode // Update the original currency code as well.
		app.logger.Info("converted amount", zap.String("converted_amount", income.Amount.String()))
		app.logger.Info("exchange rate", zap.String("exchange_rate", income.ExchangeRate.String()))

	} else if input.CurrencyCode != nil {
		// If only the CurrencyCode is provided, ensure the amount is also updated.
		v.AddError("amount", "amount must be provided if the currency code is changed")
		app.failedValidationResponse(w, r, v.Errors)
		return

	} else if input.Amount != nil {
		// If only the Amount is provided, assume it's in the same currency.
		income.Amount = *input.Amount
	}

	// Update optional fields only if they are provided.
	if input.Source != nil {
		income.Source = *input.Source
	}
	if input.Description != nil {
		income.Description = *input.Description
	}
	if input.DateReceived != nil {
		income.DateReceived = *input.DateReceived
	}

	// Validate the updated income.
	if data.ValidateIncome(v, income); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	// Save the updated income to the database.
	err = app.models.FinancialTrackingManager.UpdateIncomeByID(user.ID, income)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// Send a success response (optional, you may want to return the updated income).
	err = app.writeJSON(w, http.StatusOK, envelope{"income": income}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// createNewDebtHandler() creates a new debt user debt for the user
// We calculate initial values including payoff dates if not provided
// Perform additional validation, If everything is okya, we save the ne debt
func (app *application) createNewDebtHandler(w http.ResponseWriter, r *http.Request) {
	// debt input from user
	var input struct {
		Name           string           `json:"name"` // Name of the debt
		Amount         decimal.Decimal  `json:"amount"`
		InterestRate   decimal.Decimal  `json:"interest_rate"`
		Description    string           `json:"description"`
		DueDate        data.CustomTime1 `json:"due_date"` // YYYY-MM-DD
		MinimumPayment decimal.Decimal  `json:"minimum_payment"`
	}

	// read the request body into the input struct
	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	// create a new debt from the input data
	debt := &data.Debt{
		Name:             input.Name,
		Amount:           input.Amount,
		RemainingBalance: input.Amount, // Initially, remaining balance is the total amount
		InterestRate:     input.InterestRate,
		Description:      input.Description,
		DueDate:          input.DueDate.Time,
		MinimumPayment:   input.MinimumPayment,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
		NextPaymentDate:  input.DueDate.Time, // Set next payment date to the first due date initially
	}

	// validate the debt
	v := validator.New()
	if data.ValidateDebt(v, debt); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	// Calculate some additional fields:
	// Accrued interest starts at zero since no payments have been made yet
	debt.AccruedInterest = decimal.NewFromFloat(0.0)

	// The interest calculation date should be the current date, as this is when we start tracking
	debt.InterestLastCalculated = time.Now()

	debt.TotalInterestPaid = decimal.NewFromFloat(0.0) // Initial interest paid is 0

	// Estimated payoff date can be calculated based on minimum payment
	estimatedPayoffDate, err := app.calculateEstimatedPayoffDate(debt)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	debt.EstimatedPayoffDate = estimatedPayoffDate

	// Insert the new debt into the database
	err = app.models.FinancialTrackingManager.CreateNewDebt(app.contextGetUser(r).ID, debt)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrDuplicateDebt):
			v.AddError("description", "debt with this name already exists")
			app.failedValidationResponse(w, r, v.Errors)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// Respond with the created debt
	err = app.writeJSON(w, http.StatusCreated, envelope{"debt": debt}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// updateDebtByIDHandler() updates an existing debt in the database
// We perform additional validation and calculations before saving the updated debt
func (app *application) updateDebtHandler(w http.ResponseWriter, r *http.Request) {
	// Get DebtID
	debtID, err := app.readIDParam(r, "debtID")
	if err != nil || debtID < 1 {
		app.notFoundResponse(w, r)
		return
	}

	// Inline struct for input
	var input struct {
		Name           *string          `json:"name"`
		Amount         *decimal.Decimal `json:"amount"`
		InterestRate   *decimal.Decimal `json:"interest_rate"`
		Description    *string          `json:"description"`
		DueDate        *time.Time       `json:"due_date"`
		MinimumPayment *decimal.Decimal `json:"minimum_payment"`
		PaymentAmount  *decimal.Decimal `json:"payment_amount"`
	}

	// Parse the JSON request
	err = app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	// Fetch the existing debt from the DB
	debt, err := app.models.FinancialTrackingManager.GetDebtByID(app.contextGetUser(r).ID, debtID)
	if err != nil {
		if errors.Is(err, data.ErrGeneralRecordNotFound) {
			app.notFoundResponse(w, r)
		} else {
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// Update fields if provided in the input
	if input.Name != nil {
		debt.Name = *input.Name
	}
	if input.Amount != nil {
		debt.Amount = *input.Amount
		debt.RemainingBalance = *input.Amount // Reset balance to the new amount
	}
	if input.InterestRate != nil {
		debt.InterestRate = *input.InterestRate
	}
	if input.Description != nil {
		debt.Description = *input.Description
	}
	if input.DueDate != nil {
		debt.DueDate = *input.DueDate
		debt.NextPaymentDate = *input.DueDate // Update next payment date
	}
	if input.MinimumPayment != nil {
		debt.MinimumPayment = *input.MinimumPayment
	}

	// Validate the updated debt
	v := validator.New()
	if data.ValidateDebt(v, debt); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	// Recalculate estimated payoff date if the amount, interest rate, or minimum payment changed
	if input.Amount != nil || input.InterestRate != nil || input.MinimumPayment != nil {
		estimatedPayoffDate, err := app.calculateEstimatedPayoffDate(debt)
		if err != nil {
			app.serverErrorResponse(w, r, err)
			return
		}
		debt.EstimatedPayoffDate = estimatedPayoffDate
	}

	// Handle payments, if any (ensure payment amount distribution between principal and interest)
	if input.PaymentAmount != nil {
		paymentAmount := *input.PaymentAmount
		interestPayment, err := app.calculateInterestPayment(debt) // Function to calculate interest payment based on debt
		if err != nil {
			app.serverErrorResponse(w, r, err)
			return
		}

		if paymentAmount.LessThan(interestPayment) {
			app.badRequestResponse(w, r, errors.New("payment amount is less than the interest due"))
			return
		}

		principalPayment := paymentAmount.Sub(interestPayment)

		// Ensure that payment amount matches interest + principal
		if !paymentAmount.Equal(interestPayment.Add(principalPayment)) {
			app.badRequestResponse(w, r, errors.New("payment amount does not match the sum of interest and principal"))
			return
		}

		// Insert payment into debt_payments table
		payment := &data.DebtRepayment{
			DebtID:           debtID,
			UserID:           app.contextGetUser(r).ID,
			PaymentAmount:    paymentAmount,
			PaymentDate:      time.Now(),
			InterestPayment:  interestPayment,
			PrincipalPayment: principalPayment,
		}

		err = app.models.FinancialTrackingManager.CreateNewDebtPayment(app.contextGetUser(r).ID, payment)
		if err != nil {
			app.serverErrorResponse(w, r, err)
			return
		}

		// Update debt's remaining balance after payment
		debt.RemainingBalance = debt.RemainingBalance.Sub(principalPayment)
	}

	// Save the updated debt back to the database
	err = app.models.FinancialTrackingManager.UpdateDebtByID(app.contextGetUser(r).ID, debt)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// Respond with the updated debt
	err = app.writeJSON(w, http.StatusOK, envelope{"debt": debt}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// getAllDebtsByUserIDHandler() is a handler method that will return all debts for a user
// This endpoint supports pagination as well as a name search parameter for the debt's name
func (app *application) getAllDebtsByUserIDHandler(w http.ResponseWriter, r *http.Request) {
	// Inline struct for input
	var input struct {
		Name string
		data.Filters
	}
	// Read and validate the query parameters
	v := validator.New()
	qs := r.URL.Query()
	input.Name = app.readString(qs, "name", "")
	input.Filters.Page = app.readInt(qs, "page", 1, v)
	input.Filters.PageSize = app.readInt(qs, "page_size", 20, v)
	input.Filters.Sort = app.readString(qs, "sort", "created_at")
	input.Filters.SortSafelist = []string{"created_at", "-created_at"}
	// Perform validation
	if data.ValidateFilters(v, input.Filters); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// create a redis key using the data.RedisFinTrackDebtSearchPrefix:userid:input.Name as the key soo that we can cache the results
	redisKey := fmt.Sprintf("%s:%d:%s", data.RedisFinTrackDebtSearchPrefix, app.contextGetUser(r).ID, input.Name)
	// make a struct with []*DebtWithPayments and a metadata
	type debtEnvelope struct {
		Debts    []*data.DebtWithPayments `json:"debts"`
		Metadata *data.Metadata           `json:"metadata"`
	}
	// Get the debts from the cache
	cachedDebts, err := getFromCache[debtEnvelope](context.Background(), app.RedisDB, redisKey)
	if err != nil {
		switch {
		case errors.Is(err, ErrNoDataFoundInRedis):
			// Do nothing
		default:
			app.serverErrorResponse(w, r, err)
			return
		}
	}
	// If the debts are not in the cache, get them from the database
	if cachedDebts != nil {
		// Return cached data if available
		err = app.writeJSON(w, http.StatusOK, envelope{"debts": cachedDebts.Debts, "metadata": cachedDebts.Metadata}, nil)
		if err != nil {
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// Get all debts for the user
	debts, metadata, err := app.models.FinancialTrackingManager.GetAllDebtsByUserID(app.contextGetUser(r).ID, input.Name, input.Filters)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGeneralRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// save the debts to the cache using the debtEnvelope struct
	err = setToCache(context.Background(), app.RedisDB, redisKey, &debtEnvelope{Debts: debts, Metadata: &metadata}, data.DefaultFinTrackRedisDebtTTL)
	if err != nil {
		// just print the error
		app.logger.Info("error setting cache", zap.String("error", err.Error()))
	}

	// Send the response
	err = app.writeJSON(w, http.StatusOK, envelope{"debts": debts, "metadata": metadata}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// getDebtPaymentsByDebtUserIDHandler() is a handler method that will return all payments for a debt
// This endpoint supports pagination
// This route also provides filtering via start and end dates of the payment dates.
// These should default to time.Time{} for the start date and time.Now() for the end date if none are provided
// We return an error if the start date is after the end date and read them from the url parameters
func (app *application) getDebtPaymentsByDebtUserIDHandler(w http.ResponseWriter, r *http.Request) {
	// Inline struct for input
	var input struct {
		StartDate time.Time
		EndDate   time.Time
		data.Filters
	}
	// Read and validate the query parameters
	v := validator.New()
	qs := r.URL.Query()
	input.StartDate = app.readDate(qs, "start_date", time.Time{}, v)
	input.EndDate = app.readDate(qs, "end_date", time.Now(), v)
	input.Filters.Page = app.readInt(qs, "page", 1, v)
	input.Filters.PageSize = app.readInt(qs, "page_size", 20, v)
	input.Filters.Sort = app.readString(qs, "sort", "payment_date")
	input.Filters.SortSafelist = []string{"payment_date", "-payment_date"}
	// Perform validation
	if data.ValidateFilters(v, input.Filters); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// Get DebtID
	debtID, err := app.readIDParam(r, "debtID")
	if err != nil || debtID < 1 {
		app.notFoundResponse(w, r)
		return
	}
	// make redis key by using data.RedisFinTrackDebtPaymentSearchPrefix:userid:debtID:input.StartDate:input.EndDate as the key
	redisKey := fmt.Sprintf("%s:%d:%d:%s:%s", data.RedisFinTrackDebtPaymentSearchPrefix, app.contextGetUser(r).ID, debtID, input.StartDate.Format(time.RFC3339), input.EndDate.Format(time.RFC3339))
	// make a struct with []*DebtRepayment and a metadata
	type paymentEnvelope struct {
		Payments []*data.EnrichedDebtPayment `json:"payments"`
		Metadata *data.Metadata              `json:"metadata"`
	}
	// Get the payments from the cache
	cachedPayments, err := getFromCache[paymentEnvelope](context.Background(), app.RedisDB, redisKey)
	if err != nil {
		switch {
		case errors.Is(err, ErrNoDataFoundInRedis):
			// Do nothing
		default:
			app.serverErrorResponse(w, r, err)
			return
		}
	}
	// If the payments are not in the cache, get them from the database
	if cachedPayments != nil {
		// Return cached data if available
		err = app.writeJSON(w, http.StatusOK, envelope{"payments": cachedPayments.Payments, "metadata": cachedPayments.Metadata}, nil)
		if err != nil {
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// Get the debt payments from the database
	payments, metadata, err := app.models.FinancialTrackingManager.GetDebtPaymentsByDebtUserID(app.contextGetUser(r).ID, debtID, input.StartDate, input.EndDate, input.Filters)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGeneralRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// save the payments to the cache using the paymentEnvelope struct
	err = setToCache(context.Background(), app.RedisDB, redisKey, &paymentEnvelope{Payments: payments, Metadata: &metadata}, data.DefaultFinTrackRedisDebtTTL)
	if err != nil {
		// just print the error
		app.logger.Info("error setting cache", zap.String("error", err.Error()))
	}
	// Send the response
	err = app.writeJSON(w, http.StatusOK, envelope{"payments": payments, "metadata": metadata}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// makeDebtPaymentHandler() is a handler method that will handle debt payments
// We fetch the debt by ID, fetch the user input (payment amount, etc.)
// We check if the payment is late and accrued interest
// If it is late, we calculate overdue interest since the last calculated date
// We update the accrued interest in the debt
// We handle the payment by distributing between interest and principal
// We insert the payment into the debt_payments table
// We update the remaining balance and reset accrued interest
// We update the next payment date
// We save the updated debt
// We respond with the updated debt
func (app *application) makeDebtPaymentHandler(w http.ResponseWriter, r *http.Request) {
	// Step 1: Read and validate the debt ID from the URL parameters
	debtID, err := app.readIDParam(r, "debtID")
	if err != nil || debtID < 1 {
		app.notFoundResponse(w, r)
		return
	}

	// Step 2: Parse the input payment amount from the request body
	var input struct {
		PaymentAmount decimal.Decimal `json:"payment_amount"`
	}

	err = app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	// make validator
	v := validator.New()

	// Step 3: Retrieve the debt record for the user by debt ID
	debt, err := app.models.FinancialTrackingManager.GetDebtByID(app.contextGetUser(r).ID, debtID)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	// Step 3.1: Check if the paymentAmount is more than the remaining balance
	if input.PaymentAmount.GreaterThan(debt.RemainingBalance) {
		// calculate the allowed payment amount
		allowedPaymentAmount := debt.RemainingBalance.Add(debt.AccruedInterest)
		v.AddError("payment_amount", fmt.Sprintf("payment amount is more than the remaining balance. The allowed payment amount is %s", allowedPaymentAmount.String()))
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	// Step 4: Check if the accrued interest needs to be updated (if the cron job hasn't run)
	if debt.AccruedInterest.IsZero() && time.Now().After(debt.NextPaymentDate) {
		// If the interest has not been calculated for the overdue period, calculate it now
		interestAccrued, err := app.calculateInterestPayment(debt)
		if err != nil {
			app.serverErrorResponse(w, r, err)
			return
		}
		// Add the accrued interest to the debt's accrued interest
		debt.AccruedInterest = debt.AccruedInterest.Add(interestAccrued)
	}

	// Step 5: Calculate the interest and principal portions of the payment
	interestPayment := debt.AccruedInterest
	principalPayment := input.PaymentAmount.Sub(interestPayment)

	// Step 6: If the payment does not cover the accrued interest, return an error
	if principalPayment.LessThan(decimal.NewFromFloat(0)) {
		v.AddError("payment_amount", "payment does not cover accrued interest")
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	// Step 7: Create a new debt repayment record
	payment := &data.DebtRepayment{
		DebtID:           debtID,
		UserID:           app.contextGetUser(r).ID,
		PaymentAmount:    input.PaymentAmount,
		PaymentDate:      time.Now(),
		InterestPayment:  interestPayment,
		PrincipalPayment: principalPayment,
	}

	// Step 8: Save the new debt repayment record in the database
	err = app.models.FinancialTrackingManager.CreateNewDebtPayment(debt.UserID, payment)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// Step 9: Update the debt's remaining balance by subtracting the principal payment
	debt.RemainingBalance = debt.RemainingBalance.Sub(principalPayment)

	// Step 10: Reset the accrued interest to 0 after the payment is made
	debt.AccruedInterest = decimal.NewFromFloat(0)

	// Step 11: Set the next payment date (assuming monthly payments)
	debt.NextPaymentDate = debt.NextPaymentDate.AddDate(0, 1, 0) // Add one month to the next payment date

	// Step 12: Recalculate the estimated payoff date based on the updated debt details
	newPayoffDate, err := app.calculateEstimatedPayoffDate(debt)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	debt.EstimatedPayoffDate = newPayoffDate

	// Step 13: Update the last payment date and Total Interest Paid
	debt.LastPaymentDate = time.Now()
	debt.TotalInterestPaid = debt.TotalInterestPaid.Add(interestPayment)

	// Step 14: Update the debt record in the database with the new balance and dates
	err = app.models.FinancialTrackingManager.UpdateDebtByID(debt.UserID, debt)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGeneralRecordNotFound):
			app.notFoundResponse(w, r)
		case errors.Is(err, data.ErrInvalidRemainingBalance):
			// calculate balance required to be entered and create a v.AddError
			v.AddError("payment_amount", "payment amount is more than the remaining balance")
			app.failedValidationResponse(w, r, v.Errors)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// Step 14: Respond with the updated debt record
	err = app.writeJSON(w, http.StatusOK, envelope{"debt": debt}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
