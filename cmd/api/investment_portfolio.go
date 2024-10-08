package main

import (
	"errors"
	"net/http"
	"time"

	"github.com/Blue-Davinci/OptiVest/internal/data"
	"github.com/Blue-Davinci/OptiVest/internal/validator"
	"github.com/shopspring/decimal"
)

/*
	// map the status
	mappedStatus, err := app.models.InvestmentPortfolioManager.MapTransactioTypeToConstant(input.TransactionType)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrInvalidStatusType):
			v.AddError("status", "invalid status type")
			app.failedValidationResponse(w, r, v.Errors)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
*/
// createNewStockInvestmentHandler() is a handler responsible for the creation of a new stock investment
// straight forward, we verify the data the user provides, if everything is okay, we proceed with
// a save using CreateNewStockInvestment()
func (app *application) createNewStockInvestmentHandler(w http.ResponseWriter, r *http.Request) {
	// input for user input
	var input struct {
		StockSymbol   string          `json:"stock_symbol"`
		Quantity      decimal.Decimal `json:"quantity"`
		PurchasePrice decimal.Decimal `json:"purchase_price"`
		CurrentValue  decimal.Decimal `json:"current_value"`
		Sector        string          `json:"sector"`
		PurchaseDate  time.Time       `json:"purchase_date"`
		DividendYield decimal.Decimal `json:"dividend_yield"`
	}
	// decode to inout
	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	// get user
	user := app.contextGetUser(r)
	// make a validator
	v := validator.New()
	// create new stock
	stock := &data.StockInvestment{
		UserID:        user.ID,
		StockSymbol:   input.StockSymbol,
		Quantity:      input.Quantity,
		PurchasePrice: input.PurchasePrice,
		CurrentValue:  input.CurrentValue,
		Sector:        input.Sector,
		PurchaseDate:  input.PurchaseDate,
		DividendYield: input.DividendYield,
	}
	// validations
	if data.ValidateStockCreation(v, stock); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// create a new stock
	err = app.models.InvestmentPortfolioManager.CreateNewStockInvestment(user.ID, stock)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	// send response
	err = app.writeJSON(w, http.StatusCreated, envelope{"stock": stock}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// updateStockInvestmentHandler() is a handler responsible for updating a stock investment
// we verify the data the user provides, if everything is okay, we proceed with
// a save using UpdateStockInvestment()
func (app *application) updateStockInvestmentHandler(w http.ResponseWriter, r *http.Request) {
	// input for user input
	var input struct {
		Quantity      *decimal.Decimal `json:"quantity"`
		PurchasePrice *decimal.Decimal `json:"purchase_price"`
		PurchaseDate  *time.Time       `json:"purchase_date"`
		DividendYield *decimal.Decimal `json:"dividend_yield"`
		Sector        *string          `json:"sector"`
	}
	// get the stock ID
	stockID, err := app.readIDParam(r, "stockID")
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}
	// check if stock
	stock, err := app.models.InvestmentPortfolioManager.GetStockByStockID(stockID)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGeneralRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// decode to input
	err = app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	// check which fields are being updated
	if input.Quantity != nil {
		stock.Quantity = *input.Quantity
	}
	if input.PurchasePrice != nil {
		stock.PurchasePrice = *input.PurchasePrice
	}
	if input.PurchaseDate != nil {
		stock.PurchaseDate = *input.PurchaseDate
	}
	if input.DividendYield != nil {
		stock.DividendYield = *input.DividendYield
	}
	if input.Sector != nil {
		stock.Sector = *input.Sector
	}
	// make a validator and validate
	v := validator.New()
	if data.ValidateStockCreation(v, stock); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// update the stock
	err = app.models.InvestmentPortfolioManager.UpdateStockInvestment(app.contextGetUser(r).ID, stock)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGeneralEditConflict):
			app.editConflictResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// send response
	err = app.writeJSON(w, http.StatusOK, envelope{"stock": stock}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}

}
