package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/Blue-Davinci/OptiVest/internal/data"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// updateBondAnalysis updates the BondAnalysis data with performance metrics.
// ctx flows from the originating HTTP request through performAndLogBondCalculations
// into the FRED API call and Redis cache reads.
func (app *application) updateBondAnalysis(ctx context.Context, userID int64, bond *data.BondAnalysis, riskFreeRate decimal.Decimal) error {
	defaultFaceValue := decimal.NewFromFloat(1000.0)
	yearsToMaturity := app.calculateYearsToMaturity(bond.MaturityDate)
	bondAnalysisStatistics, err := app.performAndLogBondCalculations(
		ctx,
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
	err = app.models.InvestmentPortfolioManager.CreateBondAnalysis(ctx, userID, bond.BondSymbol, bond)
	if err != nil {
		return err
	}

	return nil
}

// updateStockAnalysis updates the StockAnalysis data with performance metrics.
// ctx flows from the originating HTTP request through getSectorPerformance,
// getStockInvestmentDataHandler, and downstream Alpha Vantage / Redis ops.
func (app *application) updateStockAnalysis(ctx context.Context, userID int64, stock *data.StockAnalysis, riskFreeRate decimal.Decimal) error {
	sectorPerformance, err := app.getSectorPerformance(ctx, stock.Sector)
	if err != nil {
		return err
	}

	stockAnalysisStatistics, err := app.getStockInvestmentDataHandler(ctx, stock.StockSymbol, riskFreeRate)
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
	err = app.models.InvestmentPortfolioManager.CreateStockAnalysis(ctx, userID, riskFreeRate, stock.StockSymbol, stock)
	if err != nil {
		return err
	}

	return nil
}

// =======================================================================================================

// ==========================================================================================================
// Bond Investment Calculations
// ==========================================================================================================
func (app *application) performAndLogBondCalculations(ctx context.Context, symbol, startDatestring string, faceValue, couponRate decimal.Decimal, yearsToMaturity int, riskFreeRate decimal.Decimal) (*data.BondAnalysisStatistics, error) {
	bondData, err := app.getBondInvestmentDataHandler(ctx, symbol, startDatestring)
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
	app.loggerFromContext(ctx).Info("Bond Returns Calculated", zap.Int("num_returns", len(bondReturns)))

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

// getBondInvestmentDataHandler fetches historical data for a given bond
// symbol via the FRED API, caching the result in Redis for 24h. The caller's
// ctx flows into both the Redis read/write and the upstream HTTP call so a
// disconnected request stops blocking server resources promptly.
func (app *application) getBondInvestmentDataHandler(ctx context.Context, symbol, startDatestring string) (*data.BondResponse, error) {
	redisKey := fmt.Sprintf("%s:%s", data.RedisBondTimeSeriesPrefix, symbol)
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

	app.loggerFromContext(ctx).Info("Fetching FRED bond time series", zap.String("symbol", symbol))
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
		app.loggerFromContext(ctx).Info("Bond Data found in cache", zap.String("symbol", symbol))
		return cachedResponse, nil
	}
	// Cache miss: fetch upstream behind a per-symbol singleflight so two
	// concurrent bond workers (e.g. user holds two positions in the same
	// bond series) collapse to one FRED call. The leader populates Redis
	// inside the closure for downstream cache reads.
	bondTimeSeriesResponse, err := singleflightDoTyped(&app.sf, "fred:bond:"+symbol, func() (data.BondResponse, error) {
		resp, fetchErr := GETRequest[data.BondResponse](app.http_client, timeSeriesUrl, nil)
		if fetchErr != nil {
			return data.BondResponse{}, fetchErr
		}
		if len(resp.Observations) == 0 {
			return data.BondResponse{}, fmt.Errorf("no time series data found for symbol: %s", symbol)
		}
		if cacheErr := setToCache(ctx, app.RedisDB, redisKey, &resp, ttl); cacheErr != nil {
			app.loggerFromContext(ctx).Error("Failed to cache time series data in Redis", zap.Error(cacheErr))
		}
		return resp, nil
	})
	if err != nil {
		return nil, err
	}
	app.loggerFromContext(ctx).Info("Bond File Type", zap.String("filetype", bondTimeSeriesResponse.FileType))
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

// getStockInvestmentDataHandler fetches historical data for a given stock
// symbol via the Alpha Vantage API, caching the result in Redis for 24h.
// The caller's ctx flows into the Redis ops, the upstream Alpha Vantage call,
// and the sentiment-analysis sub-fetch.
func (app *application) getStockInvestmentDataHandler(ctx context.Context, symbol string, riskFreeRate decimal.Decimal) (*data.StockAnalysisStatistics, error) {
	redisKey := fmt.Sprintf("%s:%s", data.RedisStockTimeSeriesPrefix, symbol)
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
		// Data found in cache, perform the calculations once. The previous
		// version called performAndLogCalculations twice on this branch and
		// discarded the first return value, doubling the CPU cost on every
		// cache hit and producing duplicate "Average Daily Return" log lines.
		returns, sharpe_ratio, sortino_ratio := app.performAndLogCalculations(ctx, cachedResponse, riskFreeRate)
		newStockAnalysisStatistics := data.StockAnalysisStatistics{
			Returns:      returns,
			SharpeRatio:  sharpe_ratio,
			SortinoRatio: sortino_ratio,
		}
		// call fillSentimentDataHelper to fill in the sentiment data
		err = app.fillSentimentDataHelper(ctx, &newStockAnalysisStatistics, symbol)
		if err != nil {
			// just print the error
			app.loggerFromContext(ctx).Error("Error filling sentiment data", zap.String("symbol", symbol))
		}
		app.loggerFromContext(ctx).Info("Current simble, average sentiment and most frequent label: ",
			zap.String("symbol", symbol),
			zap.String("average_sentiment", newStockAnalysisStatistics.AverageSentiment.String()),
			zap.String("most_frequent_label", newStockAnalysisStatistics.MostFrequentLabel))

		return &newStockAnalysisStatistics, nil
	}

	// Cache miss: fetch upstream behind a singleflight gate keyed per-symbol
	// so a user holding two lots of the same stock (or two concurrent
	// portfolio analyses for users that share a symbol) collapse into one
	// Alpha Vantage call. The leader populates Redis from inside the closure
	// so subsequent misses elsewhere read from cache.
	// Build the URL using the configured Alpha Vantage key from app.config;
	// never embed API keys in source. See SECURITY.md for the rotation runbook.
	timeSeriesURL := fmt.Sprintf("%s%s%s%s",
		data.ALPHA_VANTAGE_TIME_SERIES_URL,
		symbol,
		data.ALPHA_VANTAGE_API_KEY,
		app.config.api.apikeys.alphavantage.key,
	)
	app.loggerFromContext(ctx).Info("Time Series URL", zap.String("symbol", symbol))

	timeSeriesResponse, err := singleflightDoTyped(&app.sf, "av:timeseries:"+symbol, func() (data.TimeSeriesDailyResponse, error) {
		resp, fetchErr := GETRequest[data.TimeSeriesDailyResponse](app.http_client, timeSeriesURL, nil)
		if fetchErr != nil {
			return data.TimeSeriesDailyResponse{}, fetchErr
		}
		if len(resp.DailyTimeSeries) == 0 {
			return data.TimeSeriesDailyResponse{}, fmt.Errorf("no time series data found for symbol: %s", symbol)
		}
		if cacheErr := setToCache(ctx, app.RedisDB, redisKey, &resp, ttl); cacheErr != nil {
			app.loggerFromContext(ctx).Error("Failed to cache time series data in Redis", zap.Error(cacheErr))
		}
		return resp, nil
	})
	if err != nil {
		return nil, err
	}
	app.loggerFromContext(ctx).Info("Current risk free rate: ", zap.String("risk_free_rate", riskFreeRate.String()))

	// Perform and log the calculations
	returns, sharpe_ratio, sortino_ratio := app.performAndLogCalculations(ctx, &timeSeriesResponse, riskFreeRate)
	newStockAnalysisStatistics := data.StockAnalysisStatistics{
		Returns:      returns,
		SharpeRatio:  sharpe_ratio,
		SortinoRatio: sortino_ratio,
	}
	err = app.fillSentimentDataHelper(ctx, &newStockAnalysisStatistics, symbol)
	if err != nil {
		// just print the error
		app.loggerFromContext(ctx).Error("Error filling sentiment data", zap.Error(err))
	}

	return &newStockAnalysisStatistics, nil
}

// fillSentimentDataHelper fills a StockAnalysisStatistics struct with
// sentiment data (average sentiment, most frequent label, weighted relevance,
// ticker sentiment score, most relevant topic). ctx flows into the upstream
// Alpha Vantage sentiment fetch and Redis cache.
func (app *application) fillSentimentDataHelper(ctx context.Context, stockAnalysisStatistics *data.StockAnalysisStatistics, symbol string) error {
	sentimentData, err := app.getSentimentAnalysis(ctx, symbol)
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

// Perform and log calculations like returns, Sharpe ratio, and Sortino ratio.
// ctx is accepted purely so the "Average Daily Return" log line emitted deep
// inside getAverageDailyReturn can carry req_id/conn_id/user_id when this
// function runs under an HTTP request. The actual numerical work is pure.
func (app *application) performAndLogCalculations(ctx context.Context, timeSeriesResponse *data.TimeSeriesDailyResponse, riskFreeRate decimal.Decimal) (
	[]decimal.Decimal, // returns []
	decimal.Decimal, // sharpe ratio
	decimal.Decimal, // sortino ratio
) {
	returns := app.getAverageDailyReturn(ctx, timeSeriesResponse, time.Now().Year()-4)
	sharpeRatio := sharpeRatio(returns, riskFreeRate)
	sortinoRatio := sortinoRatio(returns, riskFreeRate)
	return returns, sharpeRatio, sortinoRatio
}

// getAverageDailyReturn is a helper function that calculates the average daily return for a given stock symbol
// We recieve a filtered map of TimeSeriesData and calculate the average daily return
func (app *application) getAverageDailyReturn(ctx context.Context, timeseriesData *data.TimeSeriesDailyResponse, lastYear int) []decimal.Decimal {
	filteredData := filterTimeSeriesBetweenYears(timeseriesData, lastYear)
	dailyReturns := calculateDailyReturns(filteredData)
	app.loggerFromContext(ctx).Info("Average Daily Return", zap.String("average_daily_return", calculateAverage(dailyReturns).String()))
	return dailyReturns
}

// filterTimeSeriesBetweenYears returns the daily entries from response whose
// date falls in the inclusive [lastYear, currentYear] window, sorted by date
// in ascending order.
//
// Sorting is essential: calculateDailyReturns walks the slice computing
// (price[i] - price[i-1]) / price[i-1], which only produces meaningful
// returns when consecutive entries are chronological neighbours. Iterating
// response.DailyTimeSeries directly would inherit Go's randomized map order
// and produce different Sharpe / Sortino ratios on every call — a real
// financial-accuracy bug, not just a determinism nuisance.
func filterTimeSeriesBetweenYears(response *data.TimeSeriesDailyResponse, lastYear int) []data.TimeSeriesDailyData {
	type dated struct {
		date  time.Time
		entry data.TimeSeriesDailyData
	}

	currentYear := time.Now().Year()
	pairs := make([]dated, 0, len(response.DailyTimeSeries))

	for dateStr, tsData := range response.DailyTimeSeries {
		date, err := time.Parse("2006-01-02", dateStr)
		if err == nil && date.Year() <= currentYear && date.Year() >= lastYear {
			pairs = append(pairs, dated{date: date, entry: tsData})
		}
	}

	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].date.Before(pairs[j].date)
	})

	filteredData := make([]data.TimeSeriesDailyData, len(pairs))
	for i, p := range pairs {
		filteredData[i] = p.entry
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
// sharpeRatio computes the Sharpe ratio for a series of returns. When the
// returns have zero volatility (e.g. a brand-new symbol whose history is
// effectively flat in the analysis window) the divisor is zero and the
// shopspring/decimal Div panics; we return decimal.Zero in that case rather
// than crashing the entire stock-analysis pipeline. Callers that need to
// distinguish "undefined" from "exactly zero" can re-derive the volatility
// themselves; we deliberately avoid surfacing an error here because the
// surrounding code path is best-effort.
func sharpeRatio(returns []decimal.Decimal, riskFreeRate decimal.Decimal) decimal.Decimal {
	avgReturn := calculateAverage(returns)
	volatility := calculateStandardDeviation(returns)
	if volatility.IsZero() {
		return decimal.Zero
	}
	return avgReturn.Sub(riskFreeRate).Div(volatility)
}

// sortinoRatio computes the Sortino ratio. Same divide-by-zero treatment as
// sharpeRatio: a stock with no negative returns in the window has zero
// downside deviation, which would otherwise panic.
func sortinoRatio(returns []decimal.Decimal, riskFreeRate decimal.Decimal) decimal.Decimal {
	avgReturn := calculateAverage(returns)
	downsideVolatility := downsideDeviation(returns)
	if downsideVolatility.IsZero() {
		return decimal.Zero
	}
	return avgReturn.Sub(riskFreeRate).Div(downsideVolatility)
}

// ==========================================================================================================
// Sentiment Analysis Calculations
// ==========================================================================================================

// getSentimentAnalysis fetches sentiment analysis data for a given stock
// symbol via the Alpha Vantage NEWS_SENTIMENT API, caching for 24h. ctx
// flows from the originating HTTP request.
func (app *application) getSentimentAnalysis(ctx context.Context, symbol string) (*data.SentimentData, error) {
	redisKey := fmt.Sprintf("%s:%s", data.RedisSentimentPrefix, symbol)
	ttl := 24 * time.Hour

	sentimentURL := fmt.Sprintf("%s%s%s%s%s%s",
		data.ALPHA_VANTAGE_BASE_URL,
		data.ALPHA_VANTAGE_SENTIMENT_FUNCTION,
		data.ALPHA_VANTAGE_TICKER,
		symbol,
		data.ALPHA_VANTAGE_API_KEY,
		app.config.api.apikeys.alphavantage.key,
	)
	app.loggerFromContext(ctx).Info("Fetching Alpha Vantage sentiment", zap.String("symbol", symbol))

	// check if it was cached
	cachedResponse, err := getFromCache[data.SentimentData](ctx, app.RedisDB, redisKey)
	if err != nil {
		switch {
		case errors.Is(err, ErrNoDataFoundInRedis):
			//return nil, ErrNoDataFoundInRedis
		default:
			app.loggerFromContext(ctx).Error("Error retrieving data from Redis", zap.Error(err))
		}
	}
	if cachedResponse != nil {
		// Data found in cache, perform and log the calculations
		app.loggerFromContext(ctx).Info("Sentiment Data found in cache", zap.String("symbol", symbol))
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
		app.loggerFromContext(ctx).Error("Failed to cache sentiment data in Redis", zap.Error(err))
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
// getRiskMetrics computes the risk-free rate for the given time horizon by
// pulling treasury yield data from Alpha Vantage (cached in Redis for 24h).
// ctx flows from the originating HTTP request.
func (app *application) getRiskMetrics(ctx context.Context, timeHorizon string) (decimal.Decimal, error) {
	redisKey := data.RedisTreasuryYieldRiskRatePrefix
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
	app.loggerFromContext(ctx).Info("Fetching Alpha Vantage treasury yield", zap.String("time_horizon", timeHorizon))
	// check if cached
	cachedResponse, err := getFromCache[data.TreasuryYieldData](ctx, app.RedisDB, redisKey)
	if err != nil {
		switch {
		case errors.Is(err, ErrNoDataFoundInRedis):
			//return nil, ErrNoDataFoundInRedis
		default:
			app.loggerFromContext(ctx).Error("Error retrieving data from Redis", zap.Error(err))
			return decimal.NewFromInt(0), err
		}
	}
	if cachedResponse != nil {
		// Data found in cache, perform and log the calculations
		app.loggerFromContext(ctx).Info("Treasury Yield Data found in cache")
		riskFactor := app.getRiskFactor(ctx, cachedResponse, timeHorizon)
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
		app.loggerFromContext(ctx).Error("Failed to cache treasury yield data in Redis", zap.Error(err))
	}
	// calculate the latest yield
	riskFactor := app.getRiskFactor(ctx, &treasuryYieldResponse, timeHorizon)
	if err != nil {
		return decimal.NewFromInt(0), err
	}
	// print out the name
	//app.logger.Info("Treasury Yield Name", zap.String("name", treasuryYieldResponse.Name))
	//app.getRiskFactor(&treasuryYieldResponse, timeHorizon)
	return riskFactor, nil
}

// getRiskFactor accepts ctx so its error log lines can correlate with the
// originating HTTP request. The treasury-yield computation itself is pure.
func (app *application) getRiskFactor(ctx context.Context, data *data.TreasuryYieldData, timeHorizone string) decimal.Decimal {
	// check time horizon
	// if time horizon includes "short" then get latest yield otherwise get average yield
	if strings.Contains(timeHorizone, "short") {
		latestRisk, err := data.GetLatestYield()
		if err != nil {
			app.loggerFromContext(ctx).Error("Failed to get latest risk rate", zap.Error(err))
			return decimal.NewFromInt(0)
		}
		return latestRisk
	}
	averageRisk, err := data.CalculateAverageYield(180)
	if err != nil {
		app.loggerFromContext(ctx).Error("Failed to calculate average risk rate", zap.Error(err))
		return decimal.NewFromInt(0)
	}
	return averageRisk
}

// ==========================================================================================================
// Sector Analysis
// ==========================================================================================================

// getSectorPerformance fetches sector performance data via the FMP API,
// cached in Redis for 5 minutes. ctx flows from the originating HTTP request.
func (app *application) getSectorPerformance(ctx context.Context, sector string) (decimal.Decimal, error) {
	redisKey := data.RedisSectorPerformancePrefix
	ttl := 5 * time.Minute

	sectorPerformanceURL := fmt.Sprintf("%s%s%s",
		data.FMP_BASE_URL,
		data.FMP_API_KEY,
		app.config.api.apikeys.fmp.key,
	)
	app.loggerFromContext(ctx).Info("Fetching FMP sector performance", zap.String("sector", sector))
	// check if cached
	cachedResponse, err := getFromCache[data.SectorAnalysisData](ctx, app.RedisDB, redisKey)
	if err != nil {
		switch {
		case errors.Is(err, ErrNoDataFoundInRedis):
			//return nil, ErrNoDataFoundInRedis
		default:
			app.loggerFromContext(ctx).Error("Error retrieving data from Redis", zap.Error(err))
			return decimal.NewFromInt(0), err
		}
	}
	if cachedResponse != nil {
		// Data found in cache, perform and log the calculations
		app.loggerFromContext(ctx).Info("Sector Performance Data found in cache")
		sectorScore, err := cachedResponse.GetSectorChange(sector)
		if err != nil {
			return decimal.NewFromInt(0), err
		}
		//app.getSectorPerformanceFactor(cachedResponse, sector)
		return sectorScore, nil
	}
	// Cache miss: fetch upstream behind a singleflight gate so concurrent
	// callers (e.g. 6 in-flight portfolio workers all looking at different
	// sectors of the same global FMP snapshot) collapse into one HTTP call.
	// Followers wait for the leader's response and reuse it. We also write
	// to Redis from inside the leader closure so the next miss elsewhere
	// reads from cache instead of re-firing the upstream call.
	sectorPerformanceResponse, err := singleflightDoTyped(&app.sf, "fmp:sector-performance", func() (data.SectorAnalysisData, error) {
		resp, fetchErr := GETRequest[data.SectorAnalysisData](app.http_client, sectorPerformanceURL, nil)
		if fetchErr != nil {
			return nil, fetchErr
		}
		if len(resp) == 0 {
			return nil, fmt.Errorf("no sector performance data found")
		}
		if cacheErr := setToCache(ctx, app.RedisDB, redisKey, &resp, ttl); cacheErr != nil {
			app.loggerFromContext(ctx).Error("Failed to cache sector performance data in Redis", zap.Error(cacheErr))
		}
		return resp, nil
	})
	if err != nil {
		return decimal.NewFromInt(0), err
	}

	sectorScore, err := sectorPerformanceResponse.GetSectorChange(sector)
	if err != nil {
		return decimal.NewFromInt(0), err
	}
	app.loggerFromContext(ctx).Info("Sector Obtained and Sector Performance", zap.String("Sector recieved", sector), zap.String("Sector Value", sectorScore.String()))
	// return sectorPerformanceResponse.GetSectorChange()
	return sectorScore, nil
}
