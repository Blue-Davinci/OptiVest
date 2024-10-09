package data

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"time"

	"github.com/Blue-Davinci/OptiVest/internal/database"
	"github.com/Blue-Davinci/OptiVest/internal/validator"
	"github.com/shopspring/decimal"
)

const (
	DefaultInvPortContextTimeout = 5 * time.Second
)

var (
	ErrInvalidInvestmentType = errors.New("invalid transaction type")
)

const (
	InvPortTransactionTypeStatusSell  = database.TransactionTypeEnumSell
	InvPortTransactionTypeStatusBuy   = database.TransactionTypeEnumBuy
	InvPortTransactionTypeStatusOther = database.TransactionTypeEnumOther
	// investment type enums
	InvPortInvestmentTypeStock       = database.InvestmentTypeEnumStock
	InvPortInvestmentTypeBond        = database.InvestmentTypeEnumBond
	InvPortInvestmentTypeAlternative = database.InvestmentTypeEnumAlternative
)

//TransactionType           database.TransactionTypeEnum `json:"transaction_type"`

type InvestmentPortfolioModel struct {
	DB *database.Queries
}

// StockInvestment represents a stock investment made by a user.
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

// BondInvestment represents a bond investment made by a user.
type BondInvestment struct {
	ID            int64           `json:"id"`             // Unique bond investment ID
	UserID        int64           `json:"user_id"`        // Reference to the user, must match Users(id)
	BondSymbol    string          `json:"bond_symbol"`    // Bond symbol, cannot be null
	Quantity      decimal.Decimal `json:"quantity"`       // Quantity of bonds, must be positive
	PurchasePrice decimal.Decimal `json:"purchase_price"` // Purchase price, cannot be negative
	CurrentValue  decimal.Decimal `json:"current_value"`  // Current value, cannot be negative
	CouponRate    decimal.Decimal `json:"coupon_rate"`    // Coupon rate, non-negative percentage
	MaturityDate  time.Time       `json:"maturity_date"`  // Maturity date for the bond
	PurchaseDate  time.Time       `json:"purchase_date"`  // Purchase date for the bond
	CreatedAt     time.Time       `json:"created_at"`     // Record creation timestamp
	UpdatedAt     time.Time       `json:"updated_at"`     // Record update timestamp
}

// AlternativeInvestment represents an alternative investment made by a user.
type AlternativeInvestment struct {
	ID                 int64           `json:"id"` // Unique alternative investment ID
	UserID             int64           `json:"user_id"`
	InvestmentType     string          `json:"investment_type"`
	InvestmentName     string          `json:"investment_name"` // Optional, could be NULL
	IsBusiness         bool            `json:"is_business"`
	Quantity           decimal.Decimal `json:"quantity"`       // Optional, could be NULL
	AnnualRevenue      decimal.Decimal `json:"annual_revenue"` // Optional, could be NULL
	AcquiredAt         time.Time       `json:"acquired_at"`    // Optional, could be NULL
	ProfitMargin       decimal.Decimal `json:"profit_margin"`  // Optional, could be NULL
	Valuation          decimal.Decimal `json:"valuation"`
	ValuationUpdatedAt time.Time       `json:"valuation_updated_at"`
	Location           string          `json:"location"`   // Optional, could be NULL
	CreatedAt          time.Time       `json:"created_at"` // Record creation timestamp
	UpdatedAt          time.Time       `json:"updated_at"` // Record update timestamp
}

// InvestmentTransaction represents a transaction made by a user in the investment portfolio.
type InvestmentTransaction struct {
	ID                int64                        `json:"id"`                 // Auto-generated ID
	UserID            int64                        `json:"user_id"`            // ID of the user making the transaction
	InvestmentType    database.InvestmentTypeEnum  `json:"investment_type"`    // Type of investment (Stock, Bond, Alternative)
	InvestmentID      int64                        `json:"investment_id"`      // ID of the investment
	TransactionType   database.TransactionTypeEnum `json:"transaction_type"`   // Type of transaction (buy, sell, other)
	TransactionDate   time.Time                    `json:"transaction_date"`   // Date of the transaction
	TransactionAmount decimal.Decimal              `json:"transaction_amount"` // Amount involved in the transaction
	Quantity          decimal.Decimal              `json:"quantity"`           // Number of units bought/sold
	CreatedAt         time.Time                    `json:"created_at"`         // Record creation timestamp
	UpdatedAt         time.Time                    `json:"updated_at"`         // Record update timestamp
}

func ValidateURLID(v *validator.Validator, stockID int64, fieldName string) {
	v.Check(stockID > 0, fieldName, "must be a valid ID")
}
func ValidateBoolean(v *validator.Validator, isBusiness bool, fieldName string) {
	v.Check(reflect.TypeOf(isBusiness).Kind() == reflect.Bool, fieldName, "must be a boolean")
}
func ValidatePurchaseDate(v *validator.Validator, purchaseDate time.Time, fieldName string) {
	// check if no date is provided
	v.Check(!purchaseDate.IsZero(), fieldName, "must be a valid date")
}

func ValidateStockCreation(v *validator.Validator, stock *StockInvestment) {
	ValidateName(v, stock.StockSymbol, "stock_symbol")
	ValidateAmount(v, stock.Quantity, "quantity")
	ValidateAmount(v, stock.PurchasePrice, "purchase_price")
	ValidateAmount(v, stock.CurrentValue, "current_value")
}

func ValidateBondCreation(v *validator.Validator, bond *BondInvestment) {
	ValidateName(v, bond.BondSymbol, "bond_symbol")
	ValidateAmount(v, bond.Quantity, "quantity")
	ValidateAmount(v, bond.PurchasePrice, "purchase_price")
	ValidateAmount(v, bond.CurrentValue, "current_value")
	ValidatePurchaseDate(v, bond.PurchaseDate, "purchase_date")
	// maturity
	ValidatePurchaseDate(v, bond.MaturityDate, "maturity_date")
}

func ValidateAlternativeInvestmentNonBusinessCreation(v *validator.Validator, alternative *AlternativeInvestment) {
	ValidateName(v, alternative.InvestmentType, "investment_type")
	// valuation
	ValidateAmount(v, alternative.Valuation, "valuation")
	// location
	ValidateName(v, alternative.Location, "location")
	// acquired at
	ValidatePurchaseDate(v, alternative.AcquiredAt, "acquired_at")
}

func ValidateAlternativeInvestmentBusinessCreation(v *validator.Validator, alternative *AlternativeInvestment) {
	ValidateName(v, alternative.InvestmentType, "investment_type")
	//quantity
	ValidateAmount(v, alternative.Quantity, "quantity")
	// valuation
	ValidateAmount(v, alternative.Valuation, "valuation")
	// annual revenue
	ValidateAmount(v, alternative.AnnualRevenue, "annual_revenue")
	// profit margin
	ValidateAmount(v, alternative.ProfitMargin, "profit_margin")
	// location
	ValidateName(v, alternative.Location, "location")
	// acquired at
	ValidatePurchaseDate(v, alternative.AcquiredAt, "acquired_at")
}

func ValidateInvestmentTransaction(v *validator.Validator, transaction *InvestmentTransaction) {
	// investment id
	ValidateURLID(v, transaction.InvestmentID, "investment_id")
	// transaction date
	ValidatePurchaseDate(v, transaction.TransactionDate, "transaction_date")
	ValidateAmount(v, transaction.TransactionAmount, "transaction_amount")
	ValidateAmount(v, transaction.Quantity, "quantity")
}

// MapTransactioTypeToConstant() maps a transaction type to a constant in the database.
func (m InvestmentPortfolioModel) MapTransactionTypeToConstant(status string) (database.TransactionTypeEnum, error) {
	switch status {
	case "buy":
		return InvPortTransactionTypeStatusBuy, nil
	case "sell":
		return InvPortTransactionTypeStatusSell, nil
	case "other":
		return InvPortTransactionTypeStatusOther, nil
	default:
		return "", ErrInvalidInvestmentType
	}
}

// MapInvestmentTypeToConstant() maps an investment type to a constant in the database.
func (m InvestmentPortfolioModel) MapInvestmentTypeToConstant(investmentType string) (database.InvestmentTypeEnum, error) {
	switch investmentType {
	case "stock":
		return InvPortInvestmentTypeStock, nil
	case "bond":
		return InvPortInvestmentTypeBond, nil
	case "alternative":
		return InvPortInvestmentTypeAlternative, nil
	default:
		return "", ErrInvalidInvestmentType
	}
}

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

// DeleteStockInvestmentByID() deletes a stock investment.
// We take in a userID and a stock ID.
// We return the stock ID of the deleted stock investment and an error if there was an issue deleting the stock investment.
func (m *InvestmentPortfolioModel) DeleteStockInvestmentByID(userID, stockID int64) (int64, error) {
	ctx, cancel := contextGenerator(context.Background(), DefaultInvPortContextTimeout)
	defer cancel()
	// Delete the stock investment from the database.
	deletedStockID, err := m.DB.DeleteStockInvestmentByID(ctx, database.DeleteStockInvestmentByIDParams{
		ID:     stockID,
		UserID: userID,
	})
	if err != nil {
		switch {
		case err == sql.ErrNoRows:
			return 0, ErrGeneralRecordNotFound
		default:
			return 0, err
		}
	}
	// Return the stock ID of the deleted stock investment and nil if there was no error.
	return deletedStockID, nil
}

// CreateNewBondInvestment() creates a new bond investment in the database.
// We take in a user id, and a pointer to a bond investment.
// We return an error if there was an issue creating the bond investment.
func (m *InvestmentPortfolioModel) CreateNewBondInvestment(userID int64, bondInvestment *BondInvestment) error {
	ctx, cancel := contextGenerator(context.Background(), DefaultInvPortContextTimeout)
	defer cancel()
	// Create a new bond investment in the database.
	newBondInfo, err := m.DB.CreateNewBondInvestment(ctx, database.CreateNewBondInvestmentParams{
		UserID:        userID,
		BondSymbol:    bondInvestment.BondSymbol,
		Quantity:      bondInvestment.Quantity.String(),
		PurchasePrice: bondInvestment.PurchasePrice.String(),
		CurrentValue:  bondInvestment.CurrentValue.String(),
		CouponRate:    sql.NullString{String: bondInvestment.CouponRate.String(), Valid: bondInvestment.CouponRate.String() != ""},
		MaturityDate:  bondInvestment.MaturityDate,
		PurchaseDate:  bondInvestment.PurchaseDate,
	})
	if err != nil {
		return err
	}
	// fill in the bond investment struct with the information from the database.
	bondInvestment.ID = newBondInfo.ID
	bondInvestment.UserID = userID
	bondInvestment.CreatedAt = newBondInfo.CreatedAt.Time
	bondInvestment.UpdatedAt = newBondInfo.UpdatedAt.Time

	// Return nil if there was no error.
	return nil
}

// DeleteInvestmentTransactionByID() deletes an investment transaction.
// We take in a userID and a transaction ID.
// We return the transaction ID of the deleted investment transaction and an error if there was an issue deleting the investment transaction.
func (m *InvestmentPortfolioModel) DeleteInvestmentTransactionByID(userID, transactionID int64) (int64, error) {
	ctx, cancel := contextGenerator(context.Background(), DefaultInvPortContextTimeout)
	defer cancel()
	// Delete the investment transaction from the database.
	deletedTransactionID, err := m.DB.DeleteInvestmentTransactionByID(ctx, database.DeleteInvestmentTransactionByIDParams{
		ID:     transactionID,
		UserID: userID,
	})
	if err != nil {
		switch {
		case err == sql.ErrNoRows:
			return 0, ErrGeneralRecordNotFound
		default:
			return 0, err
		}
	}
	// Return the transaction ID of the deleted investment transaction and nil if there was no error.
	return deletedTransactionID, nil
}

// UpdateBondInvestment() updates a bond investment in the database.
// We take in a pointer to a bond investment and a User ID.
// We return an error if there was an issue updating the bond investment.
func (m *InvestmentPortfolioModel) UpdateBondInvestment(userID int64, bondInvestment *BondInvestment) error {
	ctx, cancel := contextGenerator(context.Background(), DefaultInvPortContextTimeout)
	defer cancel()
	// Update the bond investment in the database.
	updatedAt, err := m.DB.UpdateBondInvestment(ctx, database.UpdateBondInvestmentParams{
		Quantity:      bondInvestment.Quantity.String(),
		PurchasePrice: bondInvestment.PurchasePrice.String(),
		CurrentValue:  bondInvestment.CurrentValue.String(),
		CouponRate:    sql.NullString{String: bondInvestment.CouponRate.String(), Valid: bondInvestment.CouponRate.String() != ""},
		MaturityDate:  bondInvestment.MaturityDate,
		PurchaseDate:  bondInvestment.PurchaseDate,
		ID:            bondInvestment.ID,
		UserID:        userID,
	})
	if err != nil {
		switch {
		case err == sql.ErrNoRows:
			return ErrGeneralEditConflict
		default:
			return err
		}
	}
	// fill updated at
	bondInvestment.UpdatedAt = updatedAt.Time
	// Return nil if there was no error.
	return nil
}

// DeleteBondInvestmentByID() deletes a bond investment.
// We take in a userID and a bond ID.
// We return the bond ID of the deleted bond investment and an error if there was an issue deleting the bond investment.
func (m *InvestmentPortfolioModel) DeleteBondInvestmentByID(userID, bondID int64) (int64, error) {
	ctx, cancel := contextGenerator(context.Background(), DefaultInvPortContextTimeout)
	defer cancel()
	// Delete the bond investment from the database.
	deletedBondID, err := m.DB.DeleteBondInvestmentByID(ctx, database.DeleteBondInvestmentByIDParams{
		ID:     bondID,
		UserID: userID,
	})
	if err != nil {
		switch {
		case err == sql.ErrNoRows:
			return 0, ErrGeneralRecordNotFound
		default:
			return 0, err
		}
	}
	// Return the bond ID of the deleted bond investment and nil if there was no error.
	return deletedBondID, nil
}

// GetBondByBondID() retrieves a bond investment by bond id.
// We take in a bond id.
// We return a pointer to a bond investment and an error if there was an issue retrieving the bond investment.
func (m *InvestmentPortfolioModel) GetBondByBondID(bondID int64) (*BondInvestment, error) {
	ctx, cancel := contextGenerator(context.Background(), DefaultInvPortContextTimeout)
	defer cancel()
	// Retrieve the bond investment from the database.
	bondInfo, err := m.DB.GetBondByBondID(ctx, bondID)
	if err != nil {
		switch {
		case err == sql.ErrNoRows:
			return nil, ErrGeneralRecordNotFound
		default:
			return nil, err
		}
	}
	// Create a new bond investment struct to hold the information.
	bondInvestment := populateBondInvestment(bondInfo)
	// Return the bond investment and nil if there was no error.
	return bondInvestment, nil
}

// CreateNewAlternativeInvestment() creates a new alternative investment in the database.
// We take in a user id, and a pointer to an alternative investment.
// We return an error if there was an issue creating the alternative investment.
func (m *InvestmentPortfolioModel) CreateNewAlternativeInvestment(userID int64, alternativeInvestment *AlternativeInvestment) error {
	ctx, cancel := contextGenerator(context.Background(), DefaultInvPortContextTimeout)
	defer cancel()
	// Create a new alternative investment in the database.
	newAlternativeInfo, err := m.DB.CreateNewAlternativeInvestment(ctx, database.CreateNewAlternativeInvestmentParams{
		UserID:         userID,
		InvestmentType: alternativeInvestment.InvestmentType,
		InvestmentName: sql.NullString{String: alternativeInvestment.InvestmentName, Valid: alternativeInvestment.InvestmentName != ""},
		IsBusiness:     alternativeInvestment.IsBusiness,
		Quantity:       sql.NullString{String: alternativeInvestment.Quantity.String(), Valid: alternativeInvestment.Quantity.String() != ""},
		AnnualRevenue:  sql.NullString{String: alternativeInvestment.AnnualRevenue.String(), Valid: alternativeInvestment.AnnualRevenue.String() != ""},
		AcquiredAt:     alternativeInvestment.AcquiredAt,
		ProfitMargin:   sql.NullString{String: alternativeInvestment.ProfitMargin.String(), Valid: alternativeInvestment.ProfitMargin.String() != ""},
		Valuation:      alternativeInvestment.Valuation.String(),
		Location:       sql.NullString{String: alternativeInvestment.Location, Valid: alternativeInvestment.Location != ""},
	})
	if err != nil {
		return err
	}
	// fill in the alternative investment struct with the information from the database.
	alternativeInvestment.ID = newAlternativeInfo.ID
	alternativeInvestment.UserID = userID
	alternativeInvestment.CreatedAt = newAlternativeInfo.CreatedAt.Time
	alternativeInvestment.UpdatedAt = newAlternativeInfo.UpdatedAt.Time
	// Return nil if there was no error.
	return nil
}

// UpdateAlternativeInvestment() updates an alternative investment in the database.
// We take in a pointer to an alternative investment and a User ID.
// We return an error if there was an issue updating the alternative investment.
func (m *InvestmentPortfolioModel) UpdateAlternativeInvestment(userID int64, alternativeInvestment *AlternativeInvestment) error {
	ctx, cancel := contextGenerator(context.Background(), DefaultInvPortContextTimeout)
	defer cancel()
	// Update the alternative investment in the database.
	updatedAt, err := m.DB.UpdateAlternativeInvestment(ctx, database.UpdateAlternativeInvestmentParams{
		InvestmentType:     alternativeInvestment.InvestmentType,
		InvestmentName:     sql.NullString{String: alternativeInvestment.InvestmentName, Valid: alternativeInvestment.InvestmentName != ""},
		IsBusiness:         alternativeInvestment.IsBusiness,
		Quantity:           sql.NullString{String: alternativeInvestment.Quantity.String(), Valid: alternativeInvestment.Quantity.String() != ""},
		AnnualRevenue:      sql.NullString{String: alternativeInvestment.AnnualRevenue.String(), Valid: alternativeInvestment.AnnualRevenue.String() != ""},
		AcquiredAt:         alternativeInvestment.AcquiredAt,
		ProfitMargin:       sql.NullString{String: alternativeInvestment.ProfitMargin.String(), Valid: alternativeInvestment.ProfitMargin.String() != ""},
		Valuation:          alternativeInvestment.Valuation.String(),
		ValuationUpdatedAt: sql.NullTime{Time: alternativeInvestment.ValuationUpdatedAt, Valid: !alternativeInvestment.ValuationUpdatedAt.IsZero()},
		Location:           sql.NullString{String: alternativeInvestment.Location, Valid: alternativeInvestment.Location != ""},
		ID:                 alternativeInvestment.ID,
		UserID:             userID,
	})
	if err != nil {
		switch {
		case err == sql.ErrNoRows:
			return ErrGeneralEditConflict
		default:
			return err
		}
	}
	// fill updated at
	alternativeInvestment.UpdatedAt = updatedAt.UpdatedAt.Time
	alternativeInvestment.ValuationUpdatedAt = updatedAt.ValuationUpdatedAt.Time
	// Return nil if there was no error.
	return nil
}

// DeleteAlternativeInvestmentByID() deletes an alternative investment.
// We take in a userID and an alternative ID.
// We return the alternative ID of the deleted alternative investment and an error if there was an issue deleting the alternative investment.
func (m *InvestmentPortfolioModel) DeleteAlternativeInvestmentByID(userID, alternativeID int64) (int64, error) {
	ctx, cancel := contextGenerator(context.Background(), DefaultInvPortContextTimeout)
	defer cancel()
	// Delete the alternative investment from the database.
	deletedAlternativeID, err := m.DB.DeleteAlternativeInvestmentByID(ctx, database.DeleteAlternativeInvestmentByIDParams{
		ID:     alternativeID,
		UserID: userID,
	})
	if err != nil {
		switch {
		case err == sql.ErrNoRows:
			return 0, ErrGeneralRecordNotFound
		default:
			return 0, err
		}
	}
	// Return the alternative ID of the deleted alternative investment and nil if there was no error.
	return deletedAlternativeID, nil
}

// GetAlternativeInvestmentByAlternativeID() retrieves an alternative investment by alternative id.
// We take in an alternative id.
// We return a pointer to an alternative investment and an error if there was an issue retrieving the alternative investment.
func (m *InvestmentPortfolioModel) GetAlternativeInvestmentByAlternativeID(alternativeID int64) (*AlternativeInvestment, error) {
	ctx, cancel := contextGenerator(context.Background(), DefaultInvPortContextTimeout)
	defer cancel()
	// Retrieve the alternative investment from the database.
	alternativeInfo, err := m.DB.GetAlternativeInvestmentByAlternativeID(ctx, alternativeID)
	if err != nil {
		switch {
		case err == sql.ErrNoRows:
			return nil, ErrGeneralRecordNotFound
		default:
			return nil, err
		}
	}
	// Create a new alternative investment struct to hold the information.
	alternativeInvestment := populateAlternativeInvestment(alternativeInfo)
	// Return the alternative investment and nil if there was no error.
	return alternativeInvestment, nil
}

// CreateNewInvestmentTransaction() creates a new investment transaction in the database.
// We take in a user id, a transaction type, and a pointer to an investment transaction.
// We return an error if there was an issue creating the investment transaction.
func (m *InvestmentPortfolioModel) CreateNewInvestmentTransaction(userID int64, investmentTransaction *InvestmentTransaction) error {
	ctx, cancel := contextGenerator(context.Background(), DefaultInvPortContextTimeout)
	defer cancel()
	// Create a new investment transaction in the database.
	newTransactionInfo, err := m.DB.CreateNewInvestmentTransaction(ctx, database.CreateNewInvestmentTransactionParams{
		UserID:            userID,
		InvestmentType:    investmentTransaction.InvestmentType,
		InvestmentID:      investmentTransaction.InvestmentID,
		TransactionType:   investmentTransaction.TransactionType,
		TransactionDate:   investmentTransaction.TransactionDate,
		TransactionAmount: investmentTransaction.TransactionAmount.String(),
		Quantity:          investmentTransaction.Quantity.String(),
	})
	if err != nil {
		return err
	}
	// fill in the investment transaction struct with the information from the database.
	investmentTransaction.ID = newTransactionInfo.ID
	investmentTransaction.UserID = userID
	investmentTransaction.CreatedAt = newTransactionInfo.CreatedAt.Time
	investmentTransaction.UpdatedAt = newTransactionInfo.UpdatedAt.Time
	// Return nil if there was no error.
	return nil
}

// populateAlternativeInvestment() populates an alternative investment struct with information from the database.
// We take in a row from the database.
// We return a pointer to an alternative investment.
func populateAlternativeInvestment(alternativeInvestmentRow interface{}) *AlternativeInvestment {
	switch alternativeInvestment := alternativeInvestmentRow.(type) {
	case database.AlternativeInvestment:
		return &AlternativeInvestment{
			ID:                 alternativeInvestment.ID,
			UserID:             alternativeInvestment.UserID,
			InvestmentType:     alternativeInvestment.InvestmentType,
			InvestmentName:     alternativeInvestment.InvestmentName.String,
			IsBusiness:         alternativeInvestment.IsBusiness,
			Quantity:           decimal.RequireFromString(alternativeInvestment.Quantity.String),
			AnnualRevenue:      decimal.RequireFromString(alternativeInvestment.AnnualRevenue.String),
			AcquiredAt:         alternativeInvestment.AcquiredAt,
			ProfitMargin:       decimal.RequireFromString(alternativeInvestment.ProfitMargin.String),
			Valuation:          decimal.RequireFromString(alternativeInvestment.Valuation),
			ValuationUpdatedAt: alternativeInvestment.ValuationUpdatedAt.Time,
			Location:           alternativeInvestment.Location.String,
			CreatedAt:          alternativeInvestment.CreatedAt.Time,
			UpdatedAt:          alternativeInvestment.UpdatedAt.Time,
		}
	default:
		return nil
	}
}

// populateBondInvestment() populates a bond investment struct with information from the database.
// We take in a row from the database.
// We return a pointer to a bond investment.
func populateBondInvestment(bondInvestmentRow interface{}) *BondInvestment {
	switch bondInvestment := bondInvestmentRow.(type) {
	case database.BondInvestment:
		return &BondInvestment{
			ID:            bondInvestment.ID,
			UserID:        bondInvestment.UserID,
			BondSymbol:    bondInvestment.BondSymbol,
			Quantity:      decimal.RequireFromString(bondInvestment.Quantity),
			PurchasePrice: decimal.RequireFromString(bondInvestment.PurchasePrice),
			CurrentValue:  decimal.RequireFromString(bondInvestment.CurrentValue),
			CouponRate:    decimal.RequireFromString(bondInvestment.CouponRate.String),
			MaturityDate:  bondInvestment.MaturityDate,
			PurchaseDate:  bondInvestment.PurchaseDate,
			CreatedAt:     bondInvestment.CreatedAt.Time,
			UpdatedAt:     bondInvestment.UpdatedAt.Time,
		}
	default:
		return nil
	}
}

// populateStockInvestment() populates a stock investment struct with information from the database.
// We take in a row from the database.
// We return a pointer to a stock investment.
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
