package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/Blue-Davinci/OptiVest/internal/data"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// updateBondAnalysis updates the BondAnalysis data with performance metrics.
func (app *application) updateBondAnalysis(userID int64, bond *data.BondAnalysis, riskFreeRate decimal.Decimal) error {
	defaultFaceValue := decimal.NewFromFloat(1000.0)
	// calculate years to maturity by subtracting the current date from the maturity date to int
	yearsToMaturity := app.calculateYearsToMaturity(bond.MaturityDate)
	// Get bond investment data
	bondAnalysisStatistics, err := app.performAndLogBondCalculations(
		bond.BondSymbol,
		data.BondDefaultStartDate,
		defaultFaceValue,
		bond.CouponRate,
		yearsToMaturity,
		riskFreeRate,
	)
	if err != nil {
		return err
	}

	// Fill in the bond analysis data
	bond.YTM = bondAnalysisStatistics.YTM
	bond.CurrentYield = bondAnalysisStatistics.CurrentYield
	bond.MacaulayDuration = bondAnalysisStatistics.MacaulayDuration
	bond.Convexity = bondAnalysisStatistics.Convexity
	bond.BondReturns = bondAnalysisStatistics.BondReturns[:5]
	bond.AnnualReturn = bondAnalysisStatistics.AnnualReturn
	bond.BondVolatility = bondAnalysisStatistics.BondVolatility
	bond.SharpeRatio = bondAnalysisStatistics.SharpeRatio
	bond.SortinoRatio = bondAnalysisStatistics.SortinoRatio

	// save the bond analysis
	err = app.models.InvestmentPortfolioManager.CreateBondAnalysis(userID, bond.BondSymbol, bond)
	if err != nil {
		return err
	}

	return nil
}

// updateStockAnalysis updates the StockAnalysis data with performance metrics.
func (app *application) updateStockAnalysis(userID int64, stock *data.StockAnalysis, riskFreeRate decimal.Decimal) error {
	// Get sector performance
	sectorPerformance, err := app.getSectorPerformance(stock.Sector)
	if err != nil {
		return err
	}

	// Get stock investment data
	stockAnalysisStatistics, err := app.getStockInvestmentDataHandler(stock.StockSymbol, riskFreeRate)
	if err != nil {
		return err
	}

	// Fill in the stock analysis data
	stock.Returns = stockAnalysisStatistics.Returns[:5]
	stock.SharpeRatio = stockAnalysisStatistics.SharpeRatio
	stock.SortinoRatio = stockAnalysisStatistics.SortinoRatio
	stock.SectorPerformance = sectorPerformance
	stock.SentimentLabel = stockAnalysisStatistics.MostFrequentLabel
	// save the stock analysis using CreateStockAnalysis passing userID, riskFreeRate, stockSymbol, stockAnalysis
	err = app.models.InvestmentPortfolioManager.CreateStockAnalysis(userID, riskFreeRate, stock.StockSymbol, stock)
	if err != nil {
		return err
	}

	return nil
}

// =======================================================================================================

// ==========================================================================================================
// Bond Investment Calculations
// ==========================================================================================================
func (app *application) performAndLogBondCalculations(symbol, startDatestring string, faceValue, couponRate decimal.Decimal, yearsToMaturity int, riskFreeRate decimal.Decimal) (*data.BondAnalysisStatistics, error) {
	// Fetch bond data using the getBondInvestmentDataHandler
	bondData, err := app.getBondInvestmentDataHandler(symbol, startDatestring)
	if err != nil {
		return nil, fmt.Errorf("failed to get bond data: %v", err)
	}

	// Filter bond time series data for the last N years
	filteredData := bondData.FilterTimeSeriesBetweenYears(time.Now().Year() - yearsToMaturity)

	// If no data, return an error
	if len(filteredData) == 0 {
		return nil, fmt.Errorf("no bond data available for calculations")
	}

	// Use the latest bond price (the last observation value in filtered data)
	latestPriceStr := filteredData[len(filteredData)-1].Value
	currentPrice, err := decimal.NewFromString(latestPriceStr)
	if err != nil {
		return nil, fmt.Errorf("invalid bond price in data: %v", err)
	}
	// make a bond
	bond := data.Bond{
		FaceValue:       faceValue,
		CouponRate:      couponRate,
		CurrentPrice:    currentPrice,
		YearsToMaturity: yearsToMaturity,
	}
	//app.logger.Info("=============================================================================================")
	// Perform Yield to Maturity (YTM) Calculation
	ytm := calculateYTM(bond.FaceValue, bond.CurrentPrice, bond.CouponRate, bond.YearsToMaturity)
	//app.logger.Info("Yield to Maturity (YTM)", zap.String("symbol", symbol), zap.String("ytm", ytm.String()))

	// Perform Current Yield Calculation
	currentYield := calculateCurrentYield(bond.CouponRate, bond.FaceValue, bond.CurrentPrice)
	//app.logger.Info("Current Yield", zap.String("symbol", symbol), zap.String("current_yield", currentYield.String()))

	// Calculate Macaulay Duration
	macaulayDuration := bond.CalculateMacaulayDuration(ytm)
	//app.logger.Info("Macaulay Duration", zap.String("symbol", symbol), zap.String("duration", macaulayDuration.String()))

	// Calculate Convexity
	convexity := bond.CalculateConvexity(ytm)
	//app.logger.Info("Convexity", zap.String("symbol", symbol), zap.String("convexity", convexity.String()))

	// Calculate Bond Returns
	bondReturns := bondData.CalculateBondReturns()
	if len(bondReturns) == 0 {
		return nil, fmt.Errorf("no valid bond returns to calculate")
	}
	app.logger.Info("Bond Returns Calculated", zap.Int("num_returns", len(bondReturns)))

	// Calculate Anual Bond Returns
	annualReturn := calculateAnnualReturn(bond.CouponRate, bond.FaceValue, bond.CurrentPrice)
	//app.logger.Info("Annual Return", zap.String("symbol", symbol), zap.String("annual_return", annualReturn.String()))

	// Calculate Volatility
	bondVolatility := calculateBondVolatility(bondReturns)
	//app.logger.Info("Bond Volatility", zap.String("symbol", symbol), zap.String("volatility", bondVolatility.String()))

	// log the Sharpe and Sortino ratios :
	sharpe := sharpeRatio(bondReturns, riskFreeRate)
	sortino := sortinoRatio(bondReturns, riskFreeRate)
	//app.logger.Info("Sharpe Ratio", zap.String("symbol", symbol), zap.String("sharpe_ratio", sharpe.String()))
	//app.logger.Info("Sortino Ratio", zap.String("symbol", symbol), zap.String("sortino_ratio", sortino.String()))
	//app.logger.Info("=============================================================================================")
	// fill in our bond analysis
	newBondAnalysisStatistics := &data.BondAnalysisStatistics{
		YTM:              ytm,
		CurrentYield:     currentYield,
		MacaulayDuration: macaulayDuration,
		Convexity:        convexity,
		BondReturns:      bondReturns,
		AnnualReturn:     annualReturn,
		BondVolatility:   bondVolatility,
		SharpeRatio:      sharpe,
		SortinoRatio:     sortino,
	}
	return newBondAnalysisStatistics, nil
}

// getBondInvestmentDataHandler() is a helper function that fetches historical data for a given bond symbol
// We will get a symbol and use the client to fetch the historical data for that symbol via ALPHA VANTAGE API
// We use app.http_client as our main client that expects an *Optivet_Client, url, headers if any
// We expect back a TimeSeriesMonthlyResponse struct and an error
func (app *application) getBondInvestmentDataHandler(symbol, startDatestring string) (*data.BondResponse, error) {
	redisKey := fmt.Sprintf("%s:%s", data.RedisBondTimeSeriesPrefix, symbol)
	ctx := context.Background()
	ttl := 24 * time.Hour
	timeSeriesUrl := fmt.Sprintf("%s%s%s%s%s%s%s%s",
		data.FRED_BASE_URL,
		data.FRED_SERIES_ID,
		symbol,
		data.FRED_REALTIME_START,
		startDatestring,
		data.FRED_API_KEY,
		app.config.api.apikeys.fred.key,
		data.FRED_FILE_TYPE_JSON)

	app.logger.Info("Fred Compiled URL", zap.String("url", timeSeriesUrl))
	// check if it was cached
	cachedResponse, err := getFromCache[data.BondResponse](ctx, app.RedisDB, redisKey)
	if err != nil {
		switch {
		case errors.Is(err, ErrNoDataFoundInRedis):
			//return nil, ErrNoDataFoundInRedis
		default:
			return nil, fmt.Errorf("error retrieving data from Redis: %v", err)
		}
	}
	if cachedResponse != nil {
		// Data found in cache, perform and log the calculations
		app.logger.Info("Bond Data found in cache", zap.String("symbol", symbol))
		return cachedResponse, nil
	}
	// if no cache was found, get the data
	bondTimeSeriesResponse, err := GETRequest[data.BondResponse](app.http_client, timeSeriesUrl, nil)
	if err != nil {
		return nil, err
	}
	// check if we got data
	if len(bondTimeSeriesResponse.Observations) == 0 {
		return nil, fmt.Errorf("no time series data found for symbol: %s", symbol)
	}
	// Cache the data using the updated setToCache method
	err = setToCache(ctx, app.RedisDB, redisKey, &bondTimeSeriesResponse, ttl)
	if err != nil {
		app.logger.Error("Failed to cache time series data in Redis", zap.Error(err))
	}
	// print out the filetype
	app.logger.Info("Bond File Type", zap.String("filetype", bondTimeSeriesResponse.FileType))
	// just return
	return &bondTimeSeriesResponse, nil
}

// calculateYieldToMaturity() calculates the yield to maturity for a given bond
func calculateYTM(faceValue, currentPrice, couponRate decimal.Decimal, yearsToMaturity int) decimal.Decimal {
	guess := decimal.NewFromFloat(0.05) // initial guess for YTM
	precision := decimal.NewFromFloat(0.0001)
	maxIterations := 100
	for i := 0; i < maxIterations; i++ {
		bondPrice := calculateBondPrice(faceValue, couponRate, guess, yearsToMaturity)
		error := bondPrice.Sub(currentPrice)
		if error.Abs().LessThan(precision) {
			break
		}
		// Adjust the guess using Newton's method
		guess = guess.Sub(error.Div(calculateBondPriceDerivative(faceValue, couponRate, guess, yearsToMaturity)))
	}
	return guess
}

// Function to calculate bond price based on a guess for YTM
func calculateBondPrice(faceValue, couponRate, ytm decimal.Decimal, yearsToMaturity int) decimal.Decimal {
	couponPayment := couponRate.Mul(faceValue)
	bondPrice := decimal.NewFromFloat(0.0)

	for t := 1; t <= yearsToMaturity; t++ {
		discountFactor := decimal.NewFromFloat(1.0).Div((decimal.NewFromFloat(1.0).Add(ytm)).Pow(decimal.NewFromInt(int64(t))))
		bondPrice = bondPrice.Add(couponPayment.Mul(discountFactor))
	}

	finalDiscountFactor := decimal.NewFromFloat(1.0).Div((decimal.NewFromFloat(1.0).Add(ytm)).Pow(decimal.NewFromInt(int64(yearsToMaturity))))
	bondPrice = bondPrice.Add(faceValue.Mul(finalDiscountFactor))

	return bondPrice
}

// Function to calculate the derivative of bond price with respect to YTM
func calculateBondPriceDerivative(faceValue, couponRate, ytm decimal.Decimal, yearsToMaturity int) decimal.Decimal {
	couponPayment := couponRate.Mul(faceValue)
	derivative := decimal.NewFromFloat(0.0)

	for t := 1; t <= yearsToMaturity; t++ {
		discountFactor := decimal.NewFromFloat(1.0).Div((decimal.NewFromFloat(1.0).Add(ytm)).Pow(decimal.NewFromInt(int64(t + 1))))
		derivative = derivative.Sub(couponPayment.Mul(decimal.NewFromInt(int64(t)).Mul(discountFactor)))
	}

	finalDiscountFactor := decimal.NewFromFloat(1.0).Div((decimal.NewFromFloat(1.0).Add(ytm)).Pow(decimal.NewFromInt(int64(yearsToMaturity + 1))))
	derivative = derivative.Sub(faceValue.Mul(decimal.NewFromInt(int64(yearsToMaturity)).Mul(finalDiscountFactor)))

	return derivative
}

// calculateCurrentYield() calculates the current yield for a given bond
func calculateCurrentYield(couponRate, faceValue, currentPrice decimal.Decimal) decimal.Decimal {
	couponPayment := couponRate.Mul(faceValue)
	currentYield := couponPayment.Div(currentPrice)
	return currentYield
}

// Function to calculate the annual return for a bond
func calculateAnnualReturn(couponRate, faceValue, currentPrice decimal.Decimal) decimal.Decimal {
	return couponRate.Mul(faceValue).Div(currentPrice) // Coupon return
}

// Function to calculate the annual return for a bond
func calculateBondVolatility(bondReturns []decimal.Decimal) decimal.Decimal {
	return calculateStandardDeviation(bondReturns) // Reuse from stock calculations
}

// ==========================================================================================================
//
//	Stock Investment Calculations
//
// ==========================================================================================================

// getStockInvestmentDataHandler() is a helper function that fetches historical data for a given stock symbol
// We will get a symbol and use the client to fetch the historical data for that symbol via ALPHA VANTAGE API
// We use app.http_client as our main client that expects an *Optivet_Client, url, headers if any
// We expect back a TimeSeriesMonthlyResponse struct and an error
func (app *application) getStockInvestmentDataHandler(symbol string, riskFreeRate decimal.Decimal) (*data.StockAnalysisStatistics, error) {
	redisKey := fmt.Sprintf("%s:%s", data.RedisStockTimeSeriesPrefix, symbol)
	ctx := context.Background()
	ttl := 24 * time.Hour

	// Try to get the cached data from Redis
	cachedResponse, err := getFromCache[data.TimeSeriesDailyResponse](ctx, app.RedisDB, redisKey)
	if err != nil {
		switch {
		case errors.Is(err, ErrNoDataFoundInRedis):
			//return nil, ErrNoDataFoundInRedis
		default:
			return nil, fmt.Errorf("error retrieving data from Redis: %v", err)
		}
	}

	if cachedResponse != nil {
		// Data found in cache, perform and log the calculations
		app.performAndLogCalculations(cachedResponse, riskFreeRate)
		returns, sharpe_ratio, sortino_ratio := app.performAndLogCalculations(cachedResponse, riskFreeRate)
		newStockAnalysisStatistics := data.StockAnalysisStatistics{
			Returns:      returns,
			SharpeRatio:  sharpe_ratio,
			SortinoRatio: sortino_ratio,
		}
		// call fillSentimentDataHelper to fill in the sentiment data
		err = app.fillSentimentDataHelper(&newStockAnalysisStatistics, symbol)
		if err != nil {
			// just print the error
			app.logger.Error("Error filling sentiment data", zap.String("symbol", symbol))
		}
		app.logger.Info("Current simble, average sentiment and most frequent label: ",
			zap.String("symbol", symbol),
			zap.String("average_sentiment", newStockAnalysisStatistics.AverageSentiment.String()),
			zap.String("most_frequent_label", newStockAnalysisStatistics.MostFrequentLabel))

		return &newStockAnalysisStatistics, nil
	}

	// If no cached data is found, make the API call
	timeSeriesURL := fmt.Sprintf("%s%s&apikey=4X2SW379QZJPKZZC", data.ALPHA_VANTAGE_TIME_SERIES_URL, symbol)
	app.logger.Info("Time Series URL", zap.String("url", timeSeriesURL))

	timeSeriesResponse, err := GETRequest[data.TimeSeriesDailyResponse](app.http_client, timeSeriesURL, nil)
	if err != nil {
		return nil, err
	}
	// Check if the response is not empty
	if len(timeSeriesResponse.DailyTimeSeries) == 0 {
		return nil, fmt.Errorf("no time series data found for symbol: %s", symbol)
	}
	// Cache the data using the updated setToCache method
	err = setToCache(ctx, app.RedisDB, redisKey, &timeSeriesResponse, ttl)
	if err != nil {
		app.logger.Error("Failed to cache time series data in Redis", zap.Error(err))
	}
	app.logger.Info("Current risk free rate: ", zap.String("risk_free_rate", riskFreeRate.String()))

	// Perform and log the calculations
	returns, sharpe_ratio, sortino_ratio := app.performAndLogCalculations(&timeSeriesResponse, riskFreeRate)
	newStockAnalysisStatistics := data.StockAnalysisStatistics{
		Returns:      returns,
		SharpeRatio:  sharpe_ratio,
		SortinoRatio: sortino_ratio,
	}
	err = app.fillSentimentDataHelper(&newStockAnalysisStatistics, symbol)
	if err != nil {
		// just print the error
		app.logger.Error("Error filling sentiment data", zap.Error(err))
	}

	return &newStockAnalysisStatistics, nil
}

// fillSentimentDataHelper() is a helper function that will fill a StockAnalysisStatistics struct with sentiment data
// sentiment data include average sentiment, most frequent label, weighted relevance, ticker sentiment score, and most relevant topic
// we return an error if the call to getSentimentAnalysis fails
func (app *application) fillSentimentDataHelper(stockAnalysisStatistics *data.StockAnalysisStatistics, symbol string) error {
	// get sentiment data
	sentimentData, err := app.getSentimentAnalysis(symbol)
	if err != nil {
		// fill in the items with empty
		stockAnalysisStatistics.AverageSentiment = decimal.NewFromInt(0)
		stockAnalysisStatistics.MostFrequentLabel = "N/A"
		stockAnalysisStatistics.WeightedRelevance = decimal.NewFromInt(0)
		stockAnalysisStatistics.TickerSentimentScore = decimal.NewFromInt(0)
		stockAnalysisStatistics.MostRelevantTopic = "N/A"
		return err
	}
	// Calculate Average Sentiment
	stockAnalysisStatistics.AverageSentiment = sentimentData.CalculateAverageSentiment()

	// Find Most Frequent Sentiment Label
	stockAnalysisStatistics.MostFrequentLabel = sentimentData.FindMostFrequentSentimentLabel()

	// Calculate Weighted Relevance
	stockAnalysisStatistics.WeightedRelevance = sentimentData.CalculateWeightedRelevance()

	// Ticker Sentiment Score
	stockAnalysisStatistics.TickerSentimentScore = sentimentData.GetTickerSentiment(symbol)

	// Most relevant topc
	stockAnalysisStatistics.MostRelevantTopic = sentimentData.FindMostRelevantTopic()

	return nil
}

// Perform and log calculations like returns, Sharpe ratio, and Sortino ratio
func (app *application) performAndLogCalculations(timeSeriesResponse *data.TimeSeriesDailyResponse, riskFreeRate decimal.Decimal) (
	[]decimal.Decimal, // returns []
	decimal.Decimal, // sharpe ratio
	decimal.Decimal, // sortino ratio
) {
	returns := app.getAverageDailyReturn(timeSeriesResponse, time.Now().Year()-4)
	sharpeRatio := sharpeRatio(returns, riskFreeRate)
	sortinoRatio := sortinoRatio(returns, riskFreeRate)
	return returns, sharpeRatio, sortinoRatio
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

// ==========================================================================================================
// Sentiment Analysis Calculations
// ==========================================================================================================

// getSentimentAnalysis() is a helper function that fetches sentiment analysis data for a given stock symbol
func (app *application) getSentimentAnalysis(symbol string) (*data.SentimentData, error) {
	redisKey := fmt.Sprintf("%s:%s", data.RedisSentimentPrefix, symbol)
	ctx := context.Background()
	ttl := 24 * time.Hour

	sentimentURL := fmt.Sprintf("%s%s%s%s%s%s",
		data.ALPHA_VANTAGE_BASE_URL,
		data.ALPHA_VANTAGE_SENTIMENT_FUNCTION,
		data.ALPHA_VANTAGE_TICKER,
		symbol,
		data.ALPHA_VANTAGE_API_KEY,
		app.config.api.apikeys.alphavantage.key,
	)
	//app.logger.Info("=============================================================================================")
	app.logger.Info("Sentiment URL", zap.String("url", sentimentURL))
	app.logger.Info("Sentiment Symbol", zap.String("symbol", symbol))

	// check if it was cached
	cachedResponse, err := getFromCache[data.SentimentData](ctx, app.RedisDB, redisKey)
	if err != nil {
		switch {
		case errors.Is(err, ErrNoDataFoundInRedis):
			//return nil, ErrNoDataFoundInRedis
		default:
			app.logger.Error("Error retrieving data from Redis", zap.Error(err))
		}
	}
	if cachedResponse != nil {
		// Data found in cache, perform and log the calculations
		app.logger.Info("Sentiment Data found in cache", zap.String("symbol", symbol))
		return cachedResponse, nil
	}

	// if no cache was found, get the data
	sentimentResponse, err := GETRequest[data.SentimentData](app.http_client, sentimentURL, nil)
	if err != nil {
		return nil, err
	}
	// check if we got data
	if len(sentimentResponse.Feed) == 0 {
		return nil, fmt.Errorf("no sentiment data found for symbol: %s", symbol)
	}

	// Cache the data using the updated setToCache method
	err = setToCache(ctx, app.RedisDB, redisKey, &sentimentResponse, ttl)
	if err != nil {
		app.logger.Error("Failed to cache sentiment data in Redis", zap.Error(err))
	}

	// print out the filetype
	//app.logger.Info("Sentiment Amount", zap.Any("filetype", sentimentResponse.Items))
	//app.logger.Info("=============================================================================================")
	// just return
	return &sentimentResponse, nil
}

// ==========================================================================================================
// RISK
// ==========================================================================================================
// calculateRiskMetrics() is a helper function that calculates risk metrics for a given stock symbol
func (app *application) getRiskMetrics(timeHorizon string) (decimal.Decimal, error) {
	//
	redisKey := data.RedisTreasuryYieldRiskRatePrefix
	ctx := context.Background()
	ttl := 24 * time.Hour
	//https://www.alphavantage.co/query?function=TREASURY_YIELD&interval=daily&maturity=10year&apikey=NYRXRLGLWY29115K
	treasuryYieldURL := fmt.Sprintf("%s%s%s%s%s%s",
		data.ALPHA_VANTAGE_BASE_URL,
		data.ALPHA_VANTAGE_TREASURY_YIELD_FUNCTION,
		data.ALPHA_VANTAGE_DAILY_INTERVAL,
		data.ALPHA_VANTAGE_MATURITY,
		data.ALPHA_VANTAGE_API_KEY,
		app.config.api.apikeys.alphavantage.key,
	)
	app.logger.Info("Treasury Yield URL", zap.String("url", treasuryYieldURL))
	// check if cached
	cachedResponse, err := getFromCache[data.TreasuryYieldData](ctx, app.RedisDB, redisKey)
	if err != nil {
		switch {
		case errors.Is(err, ErrNoDataFoundInRedis):
			//return nil, ErrNoDataFoundInRedis
		default:
			app.logger.Error("Error retrieving data from Redis", zap.Error(err))
			return decimal.NewFromInt(0), err
		}
	}
	if cachedResponse != nil {
		// Data found in cache, perform and log the calculations
		app.logger.Info("Treasury Yield Data found in cache")
		riskFactor := app.getRiskFactor(cachedResponse, timeHorizon)
		return riskFactor, nil
	}
	// if no cache was found, get the data
	treasuryYieldResponse, err := GETRequest[data.TreasuryYieldData](app.http_client, treasuryYieldURL, nil)
	if err != nil {
		return decimal.NewFromInt(0), err
	}
	// check if we got data
	if len(treasuryYieldResponse.Data) == 0 {
		return decimal.NewFromInt(0), fmt.Errorf("no treasury yield data found")
	}
	// Cache the data using the updated setToCache method
	err = setToCache(ctx, app.RedisDB, redisKey, &treasuryYieldResponse, ttl)
	if err != nil {
		app.logger.Error("Failed to cache treasury yield data in Redis", zap.Error(err))
	}
	// calculate the latest yield
	riskFactor := app.getRiskFactor(&treasuryYieldResponse, timeHorizon)
	if err != nil {
		return decimal.NewFromInt(0), err
	}
	// print out the name
	//app.logger.Info("Treasury Yield Name", zap.String("name", treasuryYieldResponse.Name))
	//app.getRiskFactor(&treasuryYieldResponse, timeHorizon)
	return riskFactor, nil
}

func (app *application) getRiskFactor(data *data.TreasuryYieldData, timeHorizone string) decimal.Decimal {
	// check time horizon
	// if time horizon includes "short" then get latest yield otherwise get average yield
	if strings.Contains(timeHorizone, "short") {
		latestRisk, err := data.GetLatestYield()
		if err != nil {
			app.logger.Error("Failed to get latest risk rate", zap.Error(err))
			return decimal.NewFromInt(0)
		}
		return latestRisk
	}
	averageRisk, err := data.CalculateAverageYield(180)
	if err != nil {
		app.logger.Error("Failed to calculate average risk rate", zap.Error(err))
		return decimal.NewFromInt(0)
	}
	return averageRisk
}

// ==========================================================================================================
// Sector Analysis
// ==========================================================================================================

// getSectorPerformance() is a helper function that fetches sector performance data
// We only require the sector as the input and return a decimal.Decimal and an error
// of the sector performance
func (app *application) getSectorPerformance(sector string) (decimal.Decimal, error) {
	redisKey := data.RedisSectorPerformancePrefix
	ctx := context.Background()
	ttl := 5 * time.Minute

	sectorPerformanceURL := fmt.Sprintf("%s%s%s",
		data.FMP_BASE_URL,
		data.FMP_API_KEY,
		app.config.api.apikeys.fmp.key,
	)
	app.logger.Info("Sector Performance URL", zap.String("url", sectorPerformanceURL))
	// check if cached
	cachedResponse, err := getFromCache[data.SectorAnalysisData](ctx, app.RedisDB, redisKey)
	if err != nil {
		switch {
		case errors.Is(err, ErrNoDataFoundInRedis):
			//return nil, ErrNoDataFoundInRedis
		default:
			app.logger.Error("Error retrieving data from Redis", zap.Error(err))
			return decimal.NewFromInt(0), err
		}
	}
	if cachedResponse != nil {
		// Data found in cache, perform and log the calculations
		app.logger.Info("Sector Performance Data found in cache")
		sectorScore, err := cachedResponse.GetSectorChange(sector)
		if err != nil {
			return decimal.NewFromInt(0), err
		}
		//app.getSectorPerformanceFactor(cachedResponse, sector)
		return sectorScore, nil
	}
	// if no cache was found, get the data
	sectorPerformanceResponse, err := GETRequest[data.SectorAnalysisData](app.http_client, sectorPerformanceURL, nil)
	if err != nil {
		return decimal.NewFromInt(0), err
	}
	// check if we got data
	if len(sectorPerformanceResponse) == 0 {
		return decimal.NewFromInt(0), fmt.Errorf("no sector performance data found")
	}
	// Cache the data using the updated setToCache method
	err = setToCache(ctx, app.RedisDB, redisKey, &sectorPerformanceResponse, ttl)
	if err != nil {
		app.logger.Error("Failed to cache sector performance data in Redis", zap.Error(err))
	}

	sectorScore, err := sectorPerformanceResponse.GetSectorChange(sector)
	if err != nil {
		return decimal.NewFromInt(0), err
	}
	app.logger.Info("Sector Obtained and Sector Performance", zap.String("Sector recieved", sector), zap.String("Sector Value", sectorScore.String()))
	// return sectorPerformanceResponse.GetSectorChange()
	return sectorScore, nil
}
