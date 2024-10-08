package data

import (
	"context"
	"database/sql"
	"time"

	"github.com/Blue-Davinci/OptiVest/internal/database"
	"github.com/Blue-Davinci/OptiVest/internal/validator"
	"github.com/shopspring/decimal"
)

const (
	DefaultInvPortContextTimeout = 5 * time.Second
)

/*const (
	InvPortTransactionTypeStatusSell = database.TransactionTypeEnumSell
	InvPortTransactionTypeStatusBuy  = database.TransactionTypeEnumBuy
)
		TransactionType           database.TransactionTypeEnum `json:"transaction_type"`
*/

type InvestmentPortfolioModel struct {
	DB *database.Queries
}

type StockInvestment struct {
	ID                     int64           `json:"id"`
	UserID                 int64           `json:"user_id"`
	StockSymbol            string          `json:"stock_symbol"`
	Quantity               decimal.Decimal `json:"quantity"`
	PurchasePrice          decimal.Decimal `json:"purchase_price"`
	CurrentValue           decimal.Decimal `json:"current_value"`
	Sector                 string          `json:"sector"`
	PurchaseDate           time.Time       `json:"purchase_date"`
	DividendYield          decimal.Decimal `json:"dividend_yield"`
	DividendYieldUpdatedAt time.Time       `json:"dividend_yield_updated_at"`
	CreatedAt              time.Time       `json:"created_at"`
	UpdatedAt              time.Time       `json:"updated_at"`
}

func ValidateStockCreation(v *validator.Validator, stock *StockInvestment) {
	ValidateName(v, stock.StockSymbol, "stock_symbol")
	ValidateAmount(v, stock.Quantity, "quantity")
	ValidateAmount(v, stock.PurchasePrice, "purchase_price")
	ValidateAmount(v, stock.CurrentValue, "current_value")
}

/*
func (m InvestmentPortfolioModel) MapTransactioTypeToConstant(status string) (database.TransactionTypeEnum, error) {
	switch status {
	case "buy":
		return InvPortTransactionTypeStatusBuy, nil
	case "sell":
		return InvPortTransactionTypeStatusSell, nil
	default:
		return "", ErrInvalidStatusType
	}
}
*/
// CreateNewStockInvestment() creates a new stock investment in the database.
// We take in a user id, and a pointer to a stock investment.
// We return an error if there was an issue creating the stock investment.
func (m *InvestmentPortfolioModel) CreateNewStockInvestment(userID int64, stockInvestment *StockInvestment) error {
	ctx, cancel := contextGenerator(context.Background(), DefaultInvPortContextTimeout)
	defer cancel()
	// Create a new stock investment in the database.
	newStockInfo, err := m.DB.CreateNewStockInvestment(ctx, database.CreateNewStockInvestmentParams{
		UserID:        userID,
		StockSymbol:   stockInvestment.StockSymbol,
		Quantity:      stockInvestment.Quantity.String(),
		PurchasePrice: stockInvestment.PurchasePrice.String(),
		CurrentValue:  stockInvestment.CurrentValue.String(),
		Sector:        sql.NullString{String: stockInvestment.Sector, Valid: stockInvestment.Sector != ""},
		PurchaseDate:  stockInvestment.PurchaseDate,
		DividendYield: sql.NullString{String: stockInvestment.DividendYield.String(), Valid: stockInvestment.DividendYield.String() != ""},
	})
	if err != nil {
		return err
	}
	// Fill in the stock investment struct with the information from the database.
	stockInvestment.ID = newStockInfo.ID
	stockInvestment.UserID = userID
	stockInvestment.DividendYieldUpdatedAt = newStockInfo.DividendYieldUpdatedAt.Time
	stockInvestment.CreatedAt = newStockInfo.CreatedAt.Time
	stockInvestment.UpdatedAt = newStockInfo.UpdatedAt.Time
	// Return nil if there was no error.
	return nil
}

// UpdateStockInvestment() updates a stock investment in the database.
// We take in a pointer to a stock investment.
// We return an error if there was an issue updating the stock investment.
func (m *InvestmentPortfolioModel) UpdateStockInvestment(userID int64, stockInvestment *StockInvestment) error {
	ctx, cancel := contextGenerator(context.Background(), DefaultInvPortContextTimeout)
	defer cancel()
	// Update the stock investment in the database.
	updatedInfo, err := m.DB.UpdateStockInvestment(ctx, database.UpdateStockInvestmentParams{
		Quantity:               stockInvestment.Quantity.String(),
		PurchasePrice:          stockInvestment.PurchasePrice.String(),
		CurrentValue:           stockInvestment.CurrentValue.String(),
		Sector:                 sql.NullString{String: stockInvestment.Sector, Valid: stockInvestment.Sector != ""},
		PurchaseDate:           stockInvestment.PurchaseDate,
		DividendYield:          sql.NullString{String: stockInvestment.DividendYield.String(), Valid: stockInvestment.DividendYield.String() != ""},
		DividendYieldUpdatedAt: sql.NullTime{Time: stockInvestment.DividendYieldUpdatedAt, Valid: !stockInvestment.DividendYieldUpdatedAt.IsZero()},
		ID:                     stockInvestment.ID,
		UserID:                 userID,
	})
	if err != nil {
		switch {
		case err == sql.ErrNoRows:
			return ErrGeneralEditConflict
		default:
			return err
		}
	}
	// fill updated at and dividend
	stockInvestment.UpdatedAt = updatedInfo.UpdatedAt.Time
	stockInvestment.DividendYieldUpdatedAt = updatedInfo.DividendYieldUpdatedAt.Time
	// Return nil if there was no error.
	return nil
}

// GetStockByStockID() retrieves a stock investment by stock id.
// We take in a stock id.
// We return a pointer to a stock investment and an error if there was an issue retrieving the stock investment.
func (m *InvestmentPortfolioModel) GetStockByStockID(stockID int64) (*StockInvestment, error) {
	ctx, cancel := contextGenerator(context.Background(), DefaultInvPortContextTimeout)
	defer cancel()
	// Retrieve the stock investment from the database.
	stockInfo, err := m.DB.GetStockByStockID(ctx, stockID)
	if err != nil {
		switch {
		case err == sql.ErrNoRows:
			return nil, ErrGeneralRecordNotFound
		default:
			return nil, err
		}
	}
	// Create a new stock investment struct to hold the information.
	stockInvestment := populateStockInvestment(stockInfo)
	// Return the stock investment and nil if there was no error.
	return stockInvestment, nil
}

// GetStockInvestmentByUserIDAndStockSymbol() retrieves a stock investment by user id and stock symbol.
// We take in a user id and a stock symbol.
// We return a pointer to a stock investment and an error if there was an issue retrieving the stock investment.
func (m *InvestmentPortfolioModel) GetStockInvestmentByUserIDAndStockSymbol(userID int64, stockSymbol string) (*StockInvestment, error) {
	ctx, cancel := contextGenerator(context.Background(), DefaultInvPortContextTimeout)
	defer cancel()
	// Retrieve the stock investment from the database.
	stockInfo, err := m.DB.GetStockInvestmentByUserIDAndStockSymbol(ctx, database.GetStockInvestmentByUserIDAndStockSymbolParams{
		UserID:      userID,
		StockSymbol: stockSymbol,
	})
	if err != nil {
		return nil, err
	}
	// Create a new stock investment struct to hold the information.
	stockInvestment := populateStockInvestment(stockInfo)
	// Return the stock investment and nil if there was no error.
	return stockInvestment, nil
}

func populateStockInvestment(stockInvestmentRow interface{}) *StockInvestment {
	switch stockInvestment := stockInvestmentRow.(type) {
	case database.StockInvestment:
		return &StockInvestment{
			ID:                     stockInvestment.ID,
			UserID:                 stockInvestment.UserID,
			StockSymbol:            stockInvestment.StockSymbol,
			Quantity:               decimal.RequireFromString(stockInvestment.Quantity),
			PurchasePrice:          decimal.RequireFromString(stockInvestment.PurchasePrice),
			CurrentValue:           decimal.RequireFromString(stockInvestment.CurrentValue),
			Sector:                 stockInvestment.Sector.String,
			PurchaseDate:           stockInvestment.PurchaseDate,
			DividendYield:          decimal.RequireFromString(stockInvestment.DividendYield.String),
			DividendYieldUpdatedAt: stockInvestment.DividendYieldUpdatedAt.Time,
			CreatedAt:              stockInvestment.CreatedAt.Time,
			UpdatedAt:              stockInvestment.UpdatedAt.Time,
		}
	default:
		return nil
	}
}
