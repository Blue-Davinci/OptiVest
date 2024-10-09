package main

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/Blue-Davinci/OptiVest/internal/data"
	"github.com/Blue-Davinci/OptiVest/internal/validator"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
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

// deleteStockInvestmentByIDHandler() is a handler responsible for deleting a stock investment
// We get the stock ID from the URL, we then proceed to delete the stock investment
func (app *application) deleteStockInvestmentByIDHandler(w http.ResponseWriter, r *http.Request) {
	// get the stock ID
	stockID, err := app.readIDParam(r, "stockID")
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}
	// verify it is not
	v := validator.New()
	if data.ValidateURLID(v, stockID, "id"); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// delete the stock
	deletedStockID, err := app.models.InvestmentPortfolioManager.DeleteStockInvestmentByID(app.contextGetUser(r).ID, stockID)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGeneralRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// send response
	message := fmt.Sprintf("stock investment with ID %d deleted successfully", deletedStockID)
	err = app.writeJSON(w, http.StatusOK, envelope{"message": message}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// createNewBondInvestmentHandler() is a handler responsible for the creation of a new bond investment
// straight forward, we verify the data the user provides, if everything is okay, we proceed with
// a save using CreateNewBondInvestment()
func (app *application) createNewBondInvestmentHandler(w http.ResponseWriter, r *http.Request) {
	// create input var
	var input struct {
		BondSymbol    string          `json:"bond_symbol"`
		Quantity      decimal.Decimal `json:"quantity"`
		PurchasePrice decimal.Decimal `json:"purchase_price"`
		CurrentValue  decimal.Decimal `json:"current_value"`
		CouponRate    decimal.Decimal `json:"coupon_rate"`
		MaturityDate  time.Time       `json:"maturity_date"`
		PurchaseDate  time.Time       `json:"purchase_date"`
	}
	// decode to input
	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	// get user
	user := app.contextGetUser(r)
	// create new bond
	bond := &data.BondInvestment{
		BondSymbol:    input.BondSymbol,
		Quantity:      input.Quantity,
		PurchasePrice: input.PurchasePrice,
		CurrentValue:  input.CurrentValue,
		CouponRate:    input.CouponRate,
		MaturityDate:  input.MaturityDate,
		PurchaseDate:  input.PurchaseDate,
	}
	// make a validator
	v := validator.New()
	// validations
	if data.ValidateBondCreation(v, bond); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// create a new bond
	err = app.models.InvestmentPortfolioManager.CreateNewBondInvestment(user.ID, bond)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	// send response
	err = app.writeJSON(w, http.StatusCreated, envelope{"bond": bond}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}

}

// updateBondInvestmentHandler() is a handler responsible for updating a bond investment
// we verify the data the user provides, if everything is okay, we proceed with
// a save using UpdateBondInvestment()
func (app *application) updateBondInvestmentHandler(w http.ResponseWriter, r *http.Request) {
	// updateable input from user
	var input struct {
		Quantity      *decimal.Decimal `json:"quantity"`
		PurchasePrice *decimal.Decimal `json:"purchase_price"`
		CouponRate    *decimal.Decimal `json:"coupon_rate"`
		MaturityDate  *time.Time       `json:"maturity_date"`
	}
	// get the bond ID
	bondID, err := app.readIDParam(r, "bondID")
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}
	// get the bond
	bond, err := app.models.InvestmentPortfolioManager.GetBondByBondID(bondID)
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
		bond.Quantity = *input.Quantity
	}
	if input.PurchasePrice != nil {
		bond.PurchasePrice = *input.PurchasePrice
	}
	if input.CouponRate != nil {
		bond.CouponRate = *input.CouponRate
	}
	if input.MaturityDate != nil {
		bond.MaturityDate = *input.MaturityDate
	}
	// make a validator and validate
	v := validator.New()
	if data.ValidateBondCreation(v, bond); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// update the bond
	err = app.models.InvestmentPortfolioManager.UpdateBondInvestment(app.contextGetUser(r).ID, bond)
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
	err = app.writeJSON(w, http.StatusOK, envelope{"bond": bond}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// deleteBondInvestmentByIDHandler() is a handler responsible for deleting a bond investment
// We get the bond ID from the URL, we then proceed to delete the bond investment
func (app *application) deleteBondInvestmentByIDHandler(w http.ResponseWriter, r *http.Request) {
	// get the bond ID
	bondID, err := app.readIDParam(r, "bondID")
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}
	// verify it is not
	v := validator.New()
	if data.ValidateURLID(v, bondID, "id"); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// delete the bond
	deletedBondID, err := app.models.InvestmentPortfolioManager.DeleteBondInvestmentByID(app.contextGetUser(r).ID, bondID)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGeneralRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// send response
	message := fmt.Sprintf("bond investment with ID %d deleted successfully", deletedBondID)
	err = app.writeJSON(w, http.StatusOK, envelope{"message": message}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// createNewAlternativeInvestmentHandler() is a handler responsible for the creation of a new alternative investment
// we cater for both business and non business investments
// For business investments, we must validate annual revenue, profit margin, valuation and location
// For none-businesses we can allow annual revenue and profit margins to be left out of the input
// After validation, we save using the data using CreateNewAlternativeInvestment()
func (app *application) createNewAlternativeInvestmentHandler(w http.ResponseWriter, r *http.Request) {
	// create input var
	var input struct {
		InvestmentType string          `json:"investment_type"`
		InvesmentName  string          `json:"investment_name"`
		IsBusiness     bool            `json:"is_business"`
		Quantity       decimal.Decimal `json:"quantity"`
		AnnualRevenue  decimal.Decimal `json:"annual_revenue"` // must only for business
		AcquiredAt     time.Time       `json:"acquired_at"`
		ProfitMargin   decimal.Decimal `json:"profit_margin"` // must only for business
		Valuation      decimal.Decimal `json:"valuation"`
		Location       string          `json:"location"`
	}
	// decode to input
	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	// get user
	user := app.contextGetUser(r)
	// create new alternative
	alternative := &data.AlternativeInvestment{
		InvestmentType: input.InvestmentType,
		InvestmentName: input.InvesmentName,
		IsBusiness:     input.IsBusiness,
		Quantity:       input.Quantity,
		AnnualRevenue:  input.AnnualRevenue,
		AcquiredAt:     input.AcquiredAt,
		ProfitMargin:   input.ProfitMargin,
		Valuation:      input.Valuation,
		Location:       input.Location,
	}
	// validate, if business validate business fields using ValidateAlternativeInvestmentBusinessCreation()
	// if not business, validate only the non-business fields using ValidateAlternativeInvestmentNonBusinessCreation()
	v := validator.New()
	if alternative.IsBusiness {
		if data.ValidateAlternativeInvestmentBusinessCreation(v, alternative); !v.Valid() {
			app.failedValidationResponse(w, r, v.Errors)
			return
		}
	} else {
		if data.ValidateAlternativeInvestmentNonBusinessCreation(v, alternative); !v.Valid() {
			app.failedValidationResponse(w, r, v.Errors)
			return
		}
	}
	// create a new alternative
	err = app.models.InvestmentPortfolioManager.CreateNewAlternativeInvestment(user.ID, alternative)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	// send response
	err = app.writeJSON(w, http.StatusCreated, envelope{"alternative": alternative}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// updateAlternativeInvestmentHandler() is a handler responsible for updating an alternative investment
// we verify the data the user provides, if everything is okay, we proceed with
// a save using UpdateAlternativeInvestment()
// We use the same validation as the creation of the alternative investment
// we fill in an Alternative Investment, then we update the fields that are being updated
// we then validate the data and proceed with the update
func (app *application) updateAlternativeInvestmentHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		InvestmentType *string          `json:"investment_type"`
		InvesmentName  *string          `json:"investment_name"`
		IsBusiness     *bool            `json:"is_business"`
		Quantity       *decimal.Decimal `json:"quantity"`
		AnnualRevenue  *decimal.Decimal `json:"annual_revenue"` // must only for business
		AcquiredAt     *time.Time       `json:"acquired_at"`
		ProfitMargin   *decimal.Decimal `json:"profit_margin"` // must only for business
		Valuation      *decimal.Decimal `json:"valuation"`
		Location       *string          `json:"location"`
	}
	// get the alternative ID
	alternativeID, err := app.readIDParam(r, "alternativeID")
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}
	// get the alternative
	alternative, err := app.models.InvestmentPortfolioManager.GetAlternativeInvestmentByAlternativeID(alternativeID)
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
	if input.InvestmentType != nil {
		alternative.InvestmentType = *input.InvestmentType
	}
	if input.InvesmentName != nil {
		alternative.InvestmentName = *input.InvesmentName
	}
	if input.IsBusiness != nil {
		alternative.IsBusiness = *input.IsBusiness
	}
	if input.Quantity != nil {
		alternative.Quantity = *input.Quantity
	}
	if input.AnnualRevenue != nil {
		alternative.AnnualRevenue = *input.AnnualRevenue
	}
	if input.AcquiredAt != nil {
		alternative.AcquiredAt = *input.AcquiredAt
	}
	if input.ProfitMargin != nil {
		alternative.ProfitMargin = *input.ProfitMargin
	}
	if input.Valuation != nil {
		alternative.Valuation = *input.Valuation
	}
	if input.Location != nil {
		alternative.Location = *input.Location
	}
	// make a validator and validate
	v := validator.New()
	if alternative.IsBusiness {
		if data.ValidateAlternativeInvestmentBusinessCreation(v, alternative); !v.Valid() {
			app.failedValidationResponse(w, r, v.Errors)
			return
		}
	} else {
		if data.ValidateAlternativeInvestmentNonBusinessCreation(v, alternative); !v.Valid() {
			app.failedValidationResponse(w, r, v.Errors)
			return
		}
	}

	// update the alternative
	err = app.models.InvestmentPortfolioManager.UpdateAlternativeInvestment(app.contextGetUser(r).ID, alternative)
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
	err = app.writeJSON(w, http.StatusOK, envelope{"alternative": alternative}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// DeleteAlternativeInvestmentByIDHandler() is a handler responsible for deleting an alternative investment
// We get the alternative ID from the URL, we then proceed to delete the alternative investment
func (app *application) deleteAlternativeInvestmentByIDHandler(w http.ResponseWriter, r *http.Request) {
	// get the alternative ID
	alternativeID, err := app.readIDParam(r, "alternativeID")
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}
	// verify it is not
	v := validator.New()
	if data.ValidateURLID(v, alternativeID, "id"); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// delete the alternative
	deletedAlternativeID, err := app.models.InvestmentPortfolioManager.DeleteAlternativeInvestmentByID(app.contextGetUser(r).ID, alternativeID)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGeneralRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// send response
	message := fmt.Sprintf("alternative investment with ID %d deleted successfully", deletedAlternativeID)
	err = app.writeJSON(w, http.StatusOK, envelope{"message": message}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// createNewInvestmentTransactionHandler() is a handler responsible for the creation of a new investment transaction
// we verify the data the user provides, if everything is okay, we proceed with
// a save using CreateNewInvestmentTransaction()
func (app *application) createNewInvestmentTransactionHandler(w http.ResponseWriter, r *http.Request) {
	// input for user input
	var input struct {
		InvestmentType    string          `json:"investment_type"`
		InvestmentID      int64           `json:"investment_id"`
		TransactionType   string          `json:"transaction_type"`
		TransactionAmount decimal.Decimal `json:"transaction_amount"`
		TransactionDate   time.Time       `json:"transaction_date"`
		Quantity          decimal.Decimal `json:"quantity"`
	}
	// decode to input
	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	// make validator for the input
	v := validator.New()
	// try to map the investment type and transaction type
	investmentType, err := app.models.InvestmentPortfolioManager.MapInvestmentTypeToConstant(input.InvestmentType)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrInvalidInvestmentType):
			v.AddError("investment_type", "invalid investment type")
			app.failedValidationResponse(w, r, v.Errors)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	transactionType, err := app.models.InvestmentPortfolioManager.MapTransactionTypeToConstant(input.TransactionType)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrInvalidInvestmentType):
			v.AddError("investment_type", "invalid investment type")
			app.failedValidationResponse(w, r, v.Errors)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// get user
	user := app.contextGetUser(r)
	// create new transaction
	transaction := &data.InvestmentTransaction{
		UserID:            user.ID,
		InvestmentType:    investmentType,
		InvestmentID:      input.InvestmentID,
		TransactionType:   transactionType,
		TransactionAmount: input.TransactionAmount,
		TransactionDate:   input.TransactionDate,
		Quantity:          input.Quantity,
	}
	// validate the transaction
	if data.ValidateInvestmentTransaction(v, transaction); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// validate the investment
	resultValue := app.investmentTransactionValidatorHelper(v, transaction)
	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// create a new transaction
	err = app.models.InvestmentPortfolioManager.CreateNewInvestmentTransaction(user.ID, transaction)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	// if transaction was successful, let us update the investment
	err = app.updateInvestmentTransactionHelper(user.ID, input.TransactionType, input.Quantity, resultValue)
	if err != nil {
		app.logger.Info("error updating investment", zap.Error(err))
	}
	// send response
	err = app.writeJSON(w, http.StatusCreated, envelope{"investment_transaction": transaction, "updated_investment": resultValue}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// deleteInvestmentTransactionByIDHandler() is a handler responsible for deleting an investment transaction
// We get the transaction ID from the URL, we then proceed to delete the investment transaction
func (app *application) deleteInvestmentTransactionByIDHandler(w http.ResponseWriter, r *http.Request) {
	// get the transaction ID
	transactionID, err := app.readIDParam(r, "transactionID")
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}
	// verify it is not
	v := validator.New()
	if data.ValidateURLID(v, transactionID, "id"); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// delete the transaction
	deletedTransactionID, err := app.models.InvestmentPortfolioManager.DeleteInvestmentTransactionByID(app.contextGetUser(r).ID, transactionID)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGeneralRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// send response
	message := fmt.Sprintf("investment transaction with ID %d deleted successfully", deletedTransactionID)
	err = app.writeJSON(w, http.StatusOK, envelope{"message": message}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
