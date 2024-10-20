package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/Blue-Davinci/OptiVest/internal/data"
	"github.com/Blue-Davinci/OptiVest/internal/validator"
	"go.uber.org/zap"
)

// getAllFinanceDetailsForAnalysisByUserIDHandler() is a handler that returns all the finance details for analysis by user ID
// we will alse return  the LLM analysis later on
func (app *application) getAllFinanceDetailsForAnalysisByUserIDHandler(w http.ResponseWriter, r *http.Request) {
	// get the user ID
	user := app.contextGetUser(r)
	// get all the finance details for analysis by user ID
	unifiedFinanceAnalysis, err := app.models.PersonalFinancePortfolio.GetAllFinanceDetailsForAnalysisByUserID(user.ID)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGeneralRecordNotFound):
			// should not ignore as this route is full dependant on a user finance data
			// if they do not have any finance data, then we should return a not found response
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
	}
	// call the LLM analysis
	llmPersonalFinanceAnalysis, err := app.buildPersonalFinanceLLMRequest(user, unifiedFinanceAnalysis)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	// send the response
	err = app.writeJSON(w, http.StatusOK, envelope{"personal_finance_analysis": unifiedFinanceAnalysis,
		"llm_analysis": llmPersonalFinanceAnalysis}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// getPersonalFinancePrediction() is a handler that returns the personal finance prediction
// This route is for our micr-service to call and get the personal finance prediction
// We will need to send any combination of the following: atleast 5 sums of incomes per month
// with each month's date, atleast 5 sums of expenses per month with each month's date or
// And aggregate of goals and total progress/ total: current amounts, monthly contributions and target amounts
func (app *application) getPersonalFinancePrediction(w http.ResponseWriter, r *http.Request) {
	// get a date from the url as query parameter
	var input struct {
		StartDate         time.Time
		Timeline          string
		PredictionPeriod  int
		TaxDeductions     bool
		TaxRate           float64
		EnableSeasonality bool
		EnableHolidays    bool
	}
	//validate if queries are provided
	v := validator.New()
	// Call r.URL.Query() to get the url.Values map containing the query string data.
	qs := r.URL.Query()
	// use our helpers to convert the queries
	input.StartDate = app.readDate(qs, "date", time.Now().AddDate(0, -2, 0), v)
	input.Timeline = app.readString(qs, "timeline", "monthly")
	input.PredictionPeriod = app.readInt(qs, "prediction_period", 3, v)
	input.TaxDeductions = app.readBoolean(qs, "tax_deductions", false, v)
	input.TaxRate = app.readFloat64(qs, "tax_rate", 0.1, v)
	input.EnableSeasonality = app.readBoolean(qs, "enable_seasonality", false, v)
	input.EnableHolidays = app.readBoolean(qs, "enable_holidays", false, v)

	// slight validation for the timeline
	if data.ValidatePredictionParameters(v, input.Timeline); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	// check if a user has enough data points to make a prediction
	status, err := app.models.PersonalFinancePortfolio.CheckIfUserHasEnoughPredictionData(app.contextGetUser(r).ID, input.Timeline, input.StartDate)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	// we get either DataUserHasEnoughPredictionDataPerMonth, DataUserHasEnoughPredictionDataPerWeek or DataUserInsufficientPredictionData
	if input.Timeline == "monthly" && status == data.DataUserInsufficientPredictionData {
		app.failedValidationResponse(w, r, map[string]string{"error": "User has insufficient data points to make a prediction"})
		return
	} else if input.Timeline == "weekly" && status == data.DataUserInsufficientPredictionData {
		app.failedValidationResponse(w, r, map[string]string{"error": "User has insufficient data points to make a prediction"})
		return
	}

	// get the user ID
	user := app.contextGetUser(r)
	// get the personal finance prediction based on the chosseb timelin, for monthly or weekly
	// Initialize the personalFinancePrediction variable
	var personalFinancePrediction []*data.PredictionPersonalFinanceData
	if input.Timeline == "weekly" {
		app.logger.Info("Getting personal finance data for weekly")
		personalFinancePrediction, err = app.models.PersonalFinancePortfolio.GetPersonalFinanceDataForWeeklyByUserID(user.ID, input.StartDate)
		if err != nil {
			app.serverErrorResponse(w, r, err)
			return
		}
	} else {
		personalFinancePrediction, err = app.models.PersonalFinancePortfolio.GetPersonalFinanceDataForMonthByUserID(user.ID, input.StartDate)
		if err != nil {
			app.serverErrorResponse(w, r, err)
			return
		}
	}
	// get goals analysis
	goals, err := app.models.FinancialManager.GetGoalsForUserInvestmentHelper(user.ID)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGeneralRecordNotFound):
			// ignore to proceed with other check
		default:
			app.serverErrorResponse(w, r, err)
			return
		}
	}
	// get sum values for goals
	targetAmountSum, currentAmountSum, monthlyContributionSum := goals.GetSumAnalysis()
	// process the personal finance data
	info, err := app.models.PersonalFinancePortfolio.ProcessPersonalFinanceData(
		personalFinancePrediction,
		input.Timeline,
		user.CountryCode,
		input.PredictionPeriod,
		input.TaxDeductions,
		input.TaxRate,
		input.EnableSeasonality,
		input.EnableHolidays,
	)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	// convert items to float
	convertedTargetAmountSum, _ := targetAmountSum.Float64()
	convertedCurrentAmountSum, _ := currentAmountSum.Float64()
	convertedMonthlyContributionSum, _ := monthlyContributionSum.Float64()
	// add the goals analysis to the response
	info.Savings.Goal = convertedTargetAmountSum
	info.Savings.CurrentSavings = convertedCurrentAmountSum
	info.Savings.MonthlyContribution = convertedMonthlyContributionSum
	// Send get Post request using our http client
	// include an "X-API-KEY" using our config
	response, err := POSTRequest[data.PersonalFinancePredictionResponse](
		app.http_client,
		app.config.api.apikeys.optivestmicroservice.url,
		map[string]string{"X-API-KEY": app.config.api.apikeys.optivestmicroservice.key,
			"Content-Type": "application/json"},
		info,
		false,
	)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	// send the response
	err = app.writeJSON(w, http.StatusOK, envelope{"personal_finance_prediction": response}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}

}

// getOCRDRecieptDataAnalysis() is a 2 step endpoint handler that will process reciept information provided
// from the user. The user will only supply the URL of the reciept image and we will process the image.
// We will perform a POST request to the OCR.Space API endpoint to get the text from the image.
// After recieving this text, we will then send the data to our LLM to proceed with the analysis
// And return the analysis to the user.
func (app *application) getOCRDRecieptDataAnalysisHandler(w http.ResponseWriter, r *http.Request) {
	// post request, we receive the URL of the reciept image
	var input struct {
		URL string `json:"url"`
	}
	// decode the request body into the input struct
	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	// validate using validateURL
	v := validator.New()
	err = validateURL(input.URL)
	if err != nil {
		v.AddError("url", "must be a valid URL")
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	redisKey := fmt.Sprintf("%s%s", data.RedisOCRRedeiptPrefix, input.URL)
	ctx := context.Background()
	// check if we already saved this reciept image in REDIS, if yes, return the data directly
	// if not, proceed with the OCR request
	var cachedResponse *data.LLMAnalyzedPortfolio
	cachedResponse, err = getFromCache[data.LLMAnalyzedPortfolio](
		ctx,
		app.RedisDB,
		redisKey,
	)
	if err == nil && cachedResponse != nil {
		err = app.writeJSON(w, http.StatusOK, envelope{"ocr_analysis": cachedResponse}, nil)
		if err != nil {
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// process the OCR request
	ocrResponse, err := app.proces1sOCRRequestHelper(input.URL)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	// check if the OCR response is empty/length is 0, if it is return a message saying we could not process the image
	if len(ocrResponse.ParsedResults) == 0 {
		v.AddError("error", "Could not process the reciept image")
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	// send ocrRespinse to our LLM buildOCRRecieptAnalysisRequest
	llmOCRRecieptAnalysis, err := app.buildOCRRecieptAnalysisLLMRequest(ocrResponse)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	// save the response in REDIS
	err = setToCache(
		ctx,
		app.RedisDB,
		redisKey,
		llmOCRRecieptAnalysis,
		data.DefaultRedisOCRTTL,
	)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	// send the response
	err = app.writeJSON(w, http.StatusOK, envelope{"ocr_analysis": llmOCRRecieptAnalysis}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}

}

// getExpenseIncomeSummaryReportHandler() is a handler that returns the expense and income summary report
// This will return a summary for the current year of each month's total income and total expenses
func (app *application) getExpenseIncomeSummaryReportHandler(w http.ResponseWriter, r *http.Request) {
	// Get the user ID
	user := app.contextGetUser(r)
	redisKey := fmt.Sprintf("%s%d", data.RedisExpenseIncomeSummaryPrefix, user.ID)
	ctx := context.Background()

	// Attempt to get cached data from Redis
	cachedData, err := getFromCache[[]*data.ExpensesIncomesMonthlySummary](ctx, app.RedisDB, redisKey)
	if err == nil && cachedData != nil {
		// If found in cache, return the cached response
		err = app.writeJSON(w, http.StatusOK, envelope{"expense_income_summary_report": cachedData}, nil)
		if err != nil {
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// If not found in cache or an error occurred, proceed to fetch from the database
	expenseIncomeSummaryReport, err := app.models.PersonalFinancePortfolio.GetExpenseIncomeSummaryReport(user.ID)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrGeneralRecordNotFound):
			app.notFoundResponse(w, r) // No finance data found
		default:
			app.serverErrorResponse(w, r, err) // Other error
		}
		return
	}

	// Cache the result in Redis
	err = setToCache(ctx, app.RedisDB, redisKey, &expenseIncomeSummaryReport, data.DefaultRedisExpenseIncomeSummaryTTL) // Cache for 24 hours
	if err != nil {
		app.logger.Info("Error caching data:", zap.Error(err)) // Log but don't stop execution
	}

	// Return the response
	err = app.writeJSON(w, http.StatusOK, envelope{"expense_income_summary_report": expenseIncomeSummaryReport}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
