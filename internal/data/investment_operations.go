package data

import "github.com/shopspring/decimal"

const (
	ALPHA_VANTAGE_BASE_URL        = "https://www.alphavantage.co/query?"
	ALPHA_VANTAGE_TIME_SERIES_URL = ALPHA_VANTAGE_BASE_URL + "function=TIME_SERIES_DAILY&symbol="
)

// MetaData represents the metadata portion of the API response.
type MetaData struct {
	Information   string `json:"1. Information"`
	Symbol        string `json:"2. Symbol"`
	LastRefreshed string `json:"3. Last Refreshed"`
	OutputSize    string `json:"4. Output Size"`
	TimeZone      string `json:"5. Time Zone"`
}

// TimeSeriesData represents the monthly price and volume data for a single date.
type TimeSeriesDailyData struct {
	Open   decimal.Decimal `json:"1. open"`
	High   decimal.Decimal `json:"2. high"`
	Low    decimal.Decimal `json:"3. low"`
	Close  decimal.Decimal `json:"4. close"`
	Volume decimal.Decimal `json:"5. volume"`
}

// TimeSeriesMonthlyResponse represents the complete API response including metadata and the monthly time series.
type TimeSeriesDailyResponse struct {
	MetaData        MetaData                       `json:"Meta Data"`
	DailyTimeSeries map[string]TimeSeriesDailyData `json:"Time Series (Daily)"`
}
