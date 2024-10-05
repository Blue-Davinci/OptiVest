package main

import (
	"errors"
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

// createNewIncomeHandler() creates a new income for a user
// This route supports multi-currency data.
// We first check if the ORIGINAL currency provided is the same as the user's default currency
// If it is not, then we will check Via REDIS if the provided currency is supported
// If it is, we will convert the amount to the user's default currency, If it is not, we will return an error
// We than validate the income and save it to the database.
func (app *application) createNewIncomeHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Source       string          `json:"source"`
		CurrencyCode string          `json:"currency_code"`
		Amount       decimal.Decimal `json:"amount_original"`
		Description  string          `json:"description"`
		DateReceived time.Time       `json:"date_received"`
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
		DateReceived:         input.DateReceived,
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
