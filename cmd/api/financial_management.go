package main

import (
	"net/http"

	"github.com/Blue-Davinci/OptiVest/internal/data"
	"github.com/Blue-Davinci/OptiVest/internal/validator"
	"github.com/shopspring/decimal"
)

// createNewBudgetdHandler() is a handler function that handles the creation of a Budget.
// We validate a the recieved inputs in our input struct.
// If everything is okay, we perform a check to see if the currency code of the budget is
// the same as the user's currency code. If it is not the same, we use our convertor function
// to convert the amount to the user's currency code. We then save the budget to the database
// including the convertion rate.
func (app *application) createNewBudgetdHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name           string          `json:"name"`
		IsStrict       bool            `json:"is_strict"`
		Category       string          `json:"category"`
		TotalAmount    decimal.Decimal `json:"total_amount"`
		CurrencyCode   string          `json:"currency_code"`
		ConversionRate decimal.Decimal `json:"conversion_rate"`
		Description    string          `json:"description"`
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
		UserID:         user.ID,
		Name:           input.Name,
		IsStrict:       input.IsStrict,
		Category:       input.Category,
		TotalAmount:    input.TotalAmount,
		CurrencyCode:   input.CurrencyCode,
		ConversionRate: input.ConversionRate,
		Description:    input.Description,
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
