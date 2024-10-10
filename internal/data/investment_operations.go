package data

import (
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

type CustomTime struct {
	time.Time
}

// Custom time layout: "20241010T110000"
const customTimeLayout = "20060102T150405"

// UnmarshalJSON implements the unmarshalling for CustomTime
func (ct *CustomTime) UnmarshalJSON(b []byte) error {
	// Remove quotes around the string
	s := strings.Trim(string(b), "\"")

	// Parse the string using the custom layout
	parsedTime, err := time.Parse(customTimeLayout, s)
	if err != nil {
		return err
	}
	ct.Time = parsedTime
	return nil
}

// MarshalJSON implements the marshalling for CustomTime
func (ct CustomTime) MarshalJSON() ([]byte, error) {
	// Format the time using the custom layout
	formattedTime := fmt.Sprintf("\"%s\"", ct.Time.Format(customTimeLayout))
	return []byte(formattedTime), nil
}

const (
	ALPHA_VANTAGE_BASE_URL                = "https://www.alphavantage.co/query?"
	ALPHA_VANTAGE_TIME_SERIES_URL         = ALPHA_VANTAGE_BASE_URL + "function=TIME_SERIES_DAILY&symbol="
	ALPHA_VANTAGE_SENTIMENT_FUNCTION      = "function=NEWS_SENTIMENT"
	ALPHA_VANTAGE_TREASURY_YIELD_FUNCTION = "function=TREASURY_YIELD"
	ALPHA_VANTAGE_DAILY_INTERVAL          = "&interval=daily"
	ALPHA_VANTAGE_MATURITY                = "&maturity=10year"
	ALPHA_VANTAGE_TICKER                  = "&tickers="
	ALPHA_VANTAGE_API_KEY                 = "&apikey="
	// FRED
	FRED_BASE_URL       = "https://api.stlouisfed.org/fred/series/observations?"
	FRED_SERIES_ID      = "series_id="
	FRED_REALTIME_START = "&realtime_start="
	FRED_API_KEY        = "&api_key="
	FRED_FILE_TYPE_JSON = "&file_type=json"
	// FMP
	FMP_BASE_URL = "https://financialmodelingprep.com/api/v3/sectors-performance?"
	FMP_API_KEY  = "apikey="
)
const (
	RedisStockTimeSeriesPrefix       = "stock_time_series:"
	RedisBondTimeSeriesPrefix        = "bond_time_series:"
	RedisSentimentPrefix             = "sentiment:"
	RedisTreasuryYieldRiskRatePrefix = "treasury_yield_risk_rate:"
	RedisSectorPerformancePrefix     = "sector_performance:"
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

// Response represents the overall response structure.
type BondResponse struct {
	RealTimeStart    string        `json:"realtime_start"`
	RealTimeEnd      string        `json:"realtime_end"`
	ObservationStart string        `json:"observation_start"`
	ObservationEnd   string        `json:"observation_end"`
	Units            string        `json:"units"`
	OutputType       int           `json:"output_type"`
	FileType         string        `json:"file_type"`
	OrderBy          string        `json:"order_by"`
	SortOrder        string        `json:"sort_order"`
	Count            int           `json:"count"`
	Offset           int           `json:"offset"`
	Limit            int           `json:"limit"`
	Observations     []Observation `json:"observations"`
}

// Observation represents an individual observation entry.
type Observation struct {
	RealTimeStart string `json:"realtime_start"`
	RealTimeEnd   string `json:"realtime_end"`
	Date          string `json:"date"`
	Value         string `json:"value"` // Keep as string to match JSON representation
}

// Filtering time series for bond returns calculation
func (br *BondResponse) FilterTimeSeriesBetweenYears(lastYear int) []Observation {
	var filteredData []Observation
	currentYear := time.Now().Year()

	for _, tsData := range br.Observations {
		date, err := time.Parse("2006-01-02", tsData.Date) // Adjust to match the date format from FRED API
		if err == nil && date.Year() <= currentYear && date.Year() >= lastYear {
			filteredData = append(filteredData, tsData)
		}
	}

	return filteredData
}

// Function to calculate bond returns
func (br *BondResponse) CalculateBondReturns() []decimal.Decimal {
	var returns []decimal.Decimal
	var values []decimal.Decimal
	// Convert bond yield values to decimal and filter out invalid ones
	for _, tsData := range br.Observations {
		value, err := decimal.NewFromString(tsData.Value)
		if err == nil && value.IsPositive() {
			values = append(values, value)
		}
	}
	// Calculate returns based on yield values
	for i := 1; i < len(values); i++ {
		// Calculate the percentage change between values[i] and values[i-1]
		diff := values[i].Sub(values[i-1])
		returnValue := diff.Div(values[i-1]) // Return percentage difference
		// Append the return value to the slice
		returns = append(returns, returnValue)
	}

	return returns
}

type Bond struct {
	FaceValue       decimal.Decimal
	CouponRate      decimal.Decimal
	CurrentPrice    decimal.Decimal
	YearsToMaturity int
}

func (b *Bond) CalculateAnnualReturn() decimal.Decimal {
	return b.CouponRate.Mul(b.FaceValue).Div(b.CurrentPrice)
}

func (b *Bond) CalculateMacaulayDuration(ytm decimal.Decimal) decimal.Decimal {
	var duration decimal.Decimal
	for t := 1; t <= b.YearsToMaturity; t++ {
		cashFlow := b.CouponRate.Mul(b.FaceValue)
		discountedCashFlow := cashFlow.Div(decimal.NewFromFloat(1).Add(ytm).Pow(decimal.NewFromInt(int64(t))))
		duration = duration.Add(discountedCashFlow.Mul(decimal.NewFromInt(int64(t))))
	}
	finalPayment := b.FaceValue.Div(decimal.NewFromFloat(1).Add(ytm).Pow(decimal.NewFromInt(int64(b.YearsToMaturity))))
	duration = duration.Add(finalPayment.Mul(decimal.NewFromInt(int64(b.YearsToMaturity))))
	return duration.Div(b.CurrentPrice)
}

func (b *Bond) CalculateConvexity(ytm decimal.Decimal) decimal.Decimal {
	var convexity decimal.Decimal
	for t := 1; t <= b.YearsToMaturity; t++ {
		cashFlow := b.CouponRate.Mul(b.FaceValue)
		discountedCashFlow := cashFlow.Div(decimal.NewFromFloat(1).Add(ytm).Pow(decimal.NewFromInt(int64(t) + 2)))
		convexity = convexity.Add(discountedCashFlow.Mul(decimal.NewFromInt(int64(t * (t + 1)))))
	}
	finalPayment := b.FaceValue.Div(decimal.NewFromFloat(1).Add(ytm).Pow(decimal.NewFromInt(int64(b.YearsToMaturity + 2))))
	convexity = convexity.Add(finalPayment.Mul(decimal.NewFromInt(int64(b.YearsToMaturity * (b.YearsToMaturity + 1)))))
	return convexity.Div(b.CurrentPrice.Mul(decimal.NewFromFloat(1).Add(ytm).Pow(decimal.NewFromInt(2))))
}

// =================================================================================================
// Structs for Sentiment Analysis
// =================================================================================================
// Struct for the main sentiment feed data
type SentimentData struct {
	Items                    string     `json:"items"`
	SentimentScoreDefinition string     `json:"sentiment_score_definition"`
	RelevanceScoreDefinition string     `json:"relevance_score_definition"`
	Feed                     []FeedItem `json:"feed"`
}

// Struct for each feed item (news article)
type FeedItem struct {
	Title                 string            `json:"title"`
	URL                   string            `json:"url"`
	TimePublished         CustomTime        `json:"time_published"`
	Authors               []string          `json:"authors"`
	Summary               string            `json:"summary"`
	BannerImage           string            `json:"banner_image"`
	Source                string            `json:"source"`
	CategoryWithinSource  string            `json:"category_within_source"`
	SourceDomain          string            `json:"source_domain"`
	Topics                []Topic           `json:"topics"`
	OverallSentimentScore decimal.Decimal   `json:"overall_sentiment_score"`
	OverallSentimentLabel string            `json:"overall_sentiment_label"`
	TickerSentiments      []TickerSentiment `json:"ticker_sentiment"`
}

// Struct for each topic in a feed item
type Topic struct {
	Topic          string          `json:"topic"`
	RelevanceScore decimal.Decimal `json:"relevance_score"`
}

// Struct for sentiment related to specific tickers
type TickerSentiment struct {
	Ticker               string          `json:"ticker"`
	RelevanceScore       decimal.Decimal `json:"relevance_score"`
	TickerSentimentScore decimal.Decimal `json:"ticker_sentiment_score"`
	TickerSentimentLabel string          `json:"ticker_sentiment_label"`
}

// Calculate the average overall sentiment score across all feed items
func (s *SentimentData) CalculateAverageSentiment() decimal.Decimal {
	var totalSentiment decimal.Decimal
	count := decimal.NewFromInt(int64(len(s.Feed)))

	for _, item := range s.Feed {
		totalSentiment = totalSentiment.Add(item.OverallSentimentScore)
	}

	if !count.IsZero() {
		return totalSentiment.Div(count)
	}

	return decimal.Zero
}

// Find the most frequent sentiment label across all feed items
func (s *SentimentData) FindMostFrequentSentimentLabel() string {
	sentimentCount := make(map[string]int)

	for _, item := range s.Feed {
		sentimentCount[item.OverallSentimentLabel]++
	}

	mostFrequentLabel := ""
	maxCount := 0

	for label, count := range sentimentCount {
		if count > maxCount {
			maxCount = count
			mostFrequentLabel = label
		}
	}

	return mostFrequentLabel
}

// Calculate the weighted average relevance score for the overall feed
func (s *SentimentData) CalculateWeightedRelevance() decimal.Decimal {
	var totalRelevance decimal.Decimal
	var totalWeight decimal.Decimal

	for _, item := range s.Feed {
		for _, topic := range item.Topics {
			totalRelevance = totalRelevance.Add(topic.RelevanceScore)
			totalWeight = totalWeight.Add(decimal.NewFromInt(1))
		}
	}

	if !totalWeight.IsZero() {
		return totalRelevance.Div(totalWeight)
	}

	return decimal.Zero
}

// Extract the sentiment score for a particular ticker
func (s *SentimentData) GetTickerSentiment(ticker string) decimal.Decimal {
	var totalSentiment decimal.Decimal
	count := decimal.NewFromInt(0)

	for _, item := range s.Feed {
		for _, tickerSentiment := range item.TickerSentiments {
			if tickerSentiment.Ticker == ticker {
				totalSentiment = totalSentiment.Add(tickerSentiment.TickerSentimentScore)
				count = count.Add(decimal.NewFromInt(1))
			}
		}
	}

	if !count.IsZero() {
		return totalSentiment.Div(count)
	}

	return decimal.Zero
}

// Find the most relevant topic across all feed items
func (s *SentimentData) FindMostRelevantTopic() string {
	topicRelevance := make(map[string]decimal.Decimal)

	for _, item := range s.Feed {
		for _, topic := range item.Topics {
			topicRelevance[topic.Topic] = topicRelevance[topic.Topic].Add(topic.RelevanceScore)
		}
	}

	mostRelevantTopic := ""
	maxRelevance := decimal.Zero

	for topic, relevance := range topicRelevance {
		if relevance.GreaterThan(maxRelevance) {
			maxRelevance = relevance
			mostRelevantTopic = topic
		}
	}

	return mostRelevantTopic
}

// =================================================================================================
// Structs for Treasury Yield Data
// =================================================================================================
type TreasuryYieldData struct {
	Name     string             `json:"name"`
	Interval string             `json:"interval"`
	Unit     string             `json:"unit"`
	Data     []TreasuryYieldDay `json:"data"`
}

type TreasuryYieldDay struct {
	Date  string `json:"date"`
	Value string `json:"value"`
}

func (t *TreasuryYieldData) GetLatestYield() (decimal.Decimal, error) {
	if len(t.Data) == 0 {
		return decimal.Zero, fmt.Errorf("no treasury yield data available")
	}

	// Sanitize the value string
	rawValue := strings.TrimSpace(t.Data[0].Value)

	// Convert the sanitized value to decimal
	latestYield, err := decimal.NewFromString(rawValue)
	if err != nil {
		return decimal.Zero, fmt.Errorf("invalid yield value format: %v", err)
	}
	// divide by 100 to convert percentage to decimal
	latestYield = latestYield.Div(decimal.NewFromInt(100))

	return latestYield, nil
}

func (t *TreasuryYieldData) CalculateAverageYield(days int) (decimal.Decimal, error) {
	if len(t.Data) == 0 {
		return decimal.Zero, fmt.Errorf("no treasury yield data available")
	}

	// Limit the range to the number of available data points
	if days > len(t.Data) {
		days = len(t.Data)
	}

	totalYield := decimal.Zero
	validDays := 0 // Count valid data points

	for i := 0; i < days; i++ {
		// Sanitize the value string
		rawValue := strings.TrimSpace(t.Data[i].Value)

		// Try to convert the yield value to decimal
		yield, err := decimal.NewFromString(rawValue)
		if err != nil || rawValue == "." || rawValue == "" {
			// Log or skip invalid yield values
			//fmt.Printf("Skipping invalid yield value on day %d: %s\n", i, t.Data[i].Value)
			continue
		}

		totalYield = totalYield.Add(yield)
		validDays++
	}

	// If no valid data points are available, return an error
	if validDays == 0 {
		return decimal.Zero, fmt.Errorf("no valid treasury yield data points")
	}

	// Calculate the average from valid data points
	averageYield := totalYield.Div(decimal.NewFromInt(int64(validDays)))
	// divide by 100 to convert percentage to decimal
	averageYield = averageYield.Div(decimal.NewFromInt(100))
	return averageYield, nil
}

// =================================================================================================
// Sector Performance Data
// =================================================================================================
// Struct to represent each sector and its changes percentage
type SectorAnalysis struct {
	Sector            string `json:"sector"`
	ChangesPercentage string `json:"changesPercentage"`
}

// No need for a wrapper struct with "sectors" field, just a slice of SectorAnalysis
type SectorAnalysisData []SectorAnalysis

// Method to get the percentage change of a given sector using decimal.Decimal
func (s SectorAnalysisData) GetSectorChange(sectorName string) (decimal.Decimal, error) {
	fmt.Println("Sector Name: ", sectorName)
	// Loop through the sectors to find the matching sector
	for _, sector := range s {
		if strings.EqualFold(sector.Sector, sectorName) {
			// Remove the "%" symbol and parse the value into decimal
			changes := strings.TrimSuffix(sector.ChangesPercentage, "%")
			changeValue, err := decimal.NewFromString(changes)
			if err != nil {
				return decimal.Zero, fmt.Errorf("invalid percentage format for sector %s", sectorName)
			}
			return changeValue, nil
		}
	}
	// Return an error if the sector is not found
	return decimal.Zero, fmt.Errorf("sector %s not found", sectorName)
}
