package main

import (
	"fmt"
	"math"
	"time"

	"github.com/Blue-Davinci/OptiVest/internal/data"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// getTimeSeriesDataForSymbol is a helper function that fetches historical data for a given stock symbol
// We will get a symbol and use the client to fetch the historical data for that symbol via ALPHA VANTAGE API
// We use app.http_client as our main client that expects an *Optivet_Client, url, headers if any
// We expect back a TimeSeriesMonthlyResponse struct and an error
func (app *application) getTimeSeriesDataForSymbol(symbol string) (*data.TimeSeriesDailyResponse, error) {
	// Create a new TimeSeriesMonthlyRequest with the symbol
	timeSeriesURL := fmt.Sprintf("%s%s&apikey=4X2SW379QZJPKZZC", data.ALPHA_VANTAGE_TIME_SERIES_URL, symbol)
	app.logger.Info("Time Series URL", zap.String("url", timeSeriesURL))

	// send request via GETRequest: func GETRequest[T any](c *Optivet_Client, url string, headers map[string]string) (T, error) {}
	timeSeriesResponse, err := GETRequest[data.TimeSeriesDailyResponse](app.http_client, timeSeriesURL, nil)
	if err != nil {
		return nil, err
	}
	// check if the response is not empty
	if len(timeSeriesResponse.DailyTimeSeries) == 0 {
		return nil, fmt.Errorf("no time series data found for symbol: %s", symbol)
	}
	returns := app.getAverageDailyReturn(&timeSeriesResponse, time.Now().Year()-4)
	app.logger.Info("===================================================================================")
	app.logger.Info("Returns", zap.String("returns", returns[0].String()))
	app.logger.Info("Sharpe", zap.String("sharpe_ratio", sharpeRatio(returns, decimal.NewFromFloat(0.02)).String()))
	app.logger.Info("Sortino", zap.String("sortino_ratio", sortinoRatio(returns, decimal.NewFromFloat(0.02)).String()))
	app.logger.Info("===================================================================================")
	return &timeSeriesResponse, nil
}

// getAverageDailyReturn is a helper function that calculates the average daily return for a given stock symbol
// We recieve a filtered map of TimeSeriesData and calculate the average daily return
func (app *application) getAverageDailyReturn(timeseriesData *data.TimeSeriesDailyResponse, lastYear int) []decimal.Decimal {
	filteredData := filterTimeSeriesBetweenYears(timeseriesData, lastYear)
	dailyReturns := calculateDailyReturns(filteredData)
	app.logger.Info("Average Daily Return", zap.String("average_daily_return", calculateAverage(dailyReturns).String()))
	return dailyReturns
}

func filterTimeSeriesBetweenYears(response *data.TimeSeriesDailyResponse, lastYear int) []data.TimeSeriesDailyData {
	var filteredData []data.TimeSeriesDailyData
	currentYear := time.Now().Year()

	for dateStr, tsData := range response.DailyTimeSeries {
		date, err := time.Parse("2006-01-02", dateStr)
		if err == nil && date.Year() <= currentYear && date.Year() >= lastYear {
			filteredData = append(filteredData, tsData)
		}
	}

	return filteredData
}

// Function to calculate average daily returns
func calculateDailyReturns(filteredData []data.TimeSeriesDailyData) []decimal.Decimal {
	var returns []decimal.Decimal
	var prices []decimal.Decimal

	// Convert Close prices to decimal
	for _, tsData := range filteredData {
		closePrice, err := decimal.NewFromString(tsData.Close.String())
		if err == nil {
			prices = append(prices, closePrice)
		}
	}

	// Calculate returns
	for i := 1; i < len(prices); i++ {
		// Calculate the difference between prices[i] and prices[i-1]
		diff := prices[i].Sub(prices[i-1])

		// Divide the difference by prices[i-1]
		returnValue := diff.Div(prices[i-1])

		// Append the result to the returns slice
		returns = append(returns, returnValue)
	}

	return returns
}

// calculateAverageReturn() calculates the average return from a slice of decimal.Decimal values
func calculateAverage(returns []decimal.Decimal) decimal.Decimal {
	var total decimal.Decimal
	for _, r := range returns {
		total = total.Add(r)
	}
	return total.Div(decimal.NewFromInt(int64(len(returns))))
}

// calculateStandardDeviation() calculates the standard deviation 9volatility) from a slice of decimal.Decimal values
func calculateStandardDeviation(returns []decimal.Decimal) decimal.Decimal {
	average := calculateAverage(returns)
	var sumOfSquaredDifferences decimal.Decimal
	for _, r := range returns {
		diff := r.Sub(average)
		squaredDiff := diff.Mul(diff)
		sumOfSquaredDifferences = sumOfSquaredDifferences.Add(squaredDiff)
	}

	// Calculate the variance (average of the squared differences)
	variance := sumOfSquaredDifferences.Div(decimal.NewFromInt(int64(len(returns))))

	// Use the conversion-based square root function
	return sqrtDecimalUsingFloat(variance)
}

// sqrtDecimalUsingFloat() calculates the square root of a decimal.Decimal value using float64
func sqrtDecimalUsingFloat(d decimal.Decimal) decimal.Decimal {
	floatVal, _ := d.Float64()             // Convert decimal.Decimal to float64
	sqrtFloat := math.Sqrt(floatVal)       // Perform square root on float64
	return decimal.NewFromFloat(sqrtFloat) // Convert back to decimal.Decimal
}

// downsideDeviation() calculates the downside deviation from a slice of decimal.Decimal values
func downsideDeviation(returns []decimal.Decimal) decimal.Decimal {
	var sumSquares decimal.Decimal
	negativeCount := 0

	for _, r := range returns {
		if r.LessThan(decimal.NewFromInt(0)) {
			squared := r.Mul(r) // Square the negative returns
			sumSquares = sumSquares.Add(squared)
			negativeCount++
		}
	}

	if negativeCount == 0 {
		return decimal.NewFromInt(0) // Return 0 if there are no negative returns
	}

	// Calculate average of squared negative returns
	avgNegativeSquares := sumSquares.Div(decimal.NewFromInt(int64(negativeCount)))

	// Return the square root of the average
	return sqrtDecimalUsingFloat(avgNegativeSquares)
}

// sharpeRatio() calculates the Sharpe ratio from a slice of decimal.Decimal values and a risk-free rate
func sharpeRatio(returns []decimal.Decimal, riskFreeRate decimal.Decimal) decimal.Decimal {
	avgReturn := calculateAverage(returns)
	volatility := calculateStandardDeviation(returns) // Assuming you have this function

	// (avgReturn - riskFreeRate) / volatility
	return avgReturn.Sub(riskFreeRate).Div(volatility)
}

// sortinoRatio() calculates the Sortino ratio from a slice of decimal.Decimal values and a risk-free rate
func sortinoRatio(returns []decimal.Decimal, riskFreeRate decimal.Decimal) decimal.Decimal {
	avgReturn := calculateAverage(returns)
	downsideVolatility := downsideDeviation(returns) // Call downside deviation function

	// (avgReturn - riskFreeRate) / downsideVolatility
	return avgReturn.Sub(riskFreeRate).Div(downsideVolatility)
}
