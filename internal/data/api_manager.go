package data

import (
	"errors"
	"time"

	"github.com/Blue-Davinci/OptiVest/internal/database"
	"github.com/shopspring/decimal"
)

type ApiManagerModel struct {
	DB *database.Queries
}

var (
	ErrorEmptyCurrency     = errors.New("source_currency and target_currency cannot be empty")
	ErrFailedToGetRate     = errors.New("failed to get exchange rate")
	ErrFailedToGetCurrency = errors.New("failed to get currency")
)

const (
	APIExchangeCacheTTL = 24 * time.Hour
)

const (
	RedisExchangeRatePrefix = "exchange_rate"
)

// ExchangeRateResponse represents the structure of the JSON response from the API
type ExchangeRateResponse struct {
	Result             string          `json:"result"`
	Documentation      string          `json:"documentation"`
	TermsOfUse         string          `json:"terms_of_use"`
	TimeLastUpdateUnix int64           `json:"time_last_update_unix"`
	TimeLastUpdateUTC  string          `json:"time_last_update_utc"`
	TimeNextUpdateUnix int64           `json:"time_next_update_unix"`
	TimeNextUpdateUTC  string          `json:"time_next_update_utc"`
	BaseCode           string          `json:"base_code"`
	TargetCode         string          `json:"target_code"`
	ConversionRate     decimal.Decimal `json:"conversion_rate"`
}

// CurrencyRates represents the structure of the JSON response from the API on currencies
type CurrencyRates struct {
	Result          string             `json:"result"`
	Documentation   string             `json:"documentation"`
	TermsOfUse      string             `json:"terms_of_use"`
	TimeLastUpdate  int64              `json:"time_last_update_unix"`
	TimeNextUpdate  int64              `json:"time_next_update_unix"`
	BaseCode        string             `json:"base_code"`
	ConversionRates map[string]float64 `json:"conversion_rates"`
}

type ConvertedAmount struct {
	SourceAmount    decimal.Decimal `json:"source_amount"`
	ConvertedAmount decimal.Decimal `json:"converted_amount"`
}

// ConvertAmount converts a given source amount using the conversion rate in the response
func (r *ExchangeRateResponse) ConvertAmount(sourceAmount decimal.Decimal) ConvertedAmount {
	return ConvertCurrency(sourceAmount, r.ConversionRate)
}

// ConvertCurrency converts the source amount to the target amount using the conversion rate
func ConvertCurrency(sourceAmount decimal.Decimal, conversionRate decimal.Decimal) ConvertedAmount {
	convertedAmount := sourceAmount.Mul(conversionRate)
	return ConvertedAmount{
		SourceAmount:    sourceAmount,
		ConvertedAmount: convertedAmount,
	}
}
