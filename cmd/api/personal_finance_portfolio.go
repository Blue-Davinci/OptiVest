package main

import (
	"errors"
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
		StartDate string
	}
	//validate if queries are provided
	v := validator.New()
	// Call r.URL.Query() to get the url.Values map containing the query string data.
	qs := r.URL.Query()
	// use our helpers to convert the queries
	input.StartDate = app.readString(qs, "date", "")
	// If StartDate is empty, use today's date
	if input.StartDate == "" {
		app.logger.Info("No date provided, using today's date")
		input.StartDate = time.Now().Format("2006-01-02")
	} else {
		app.logger.Info("Date provided, using the provided date", zap.Any("date", input.StartDate))
	}

	// Convert input.StartDate to a time.Time object
	startDate, err := time.Parse("2006-01-02", input.StartDate)
	if err != nil {
		v.AddError("start_date", "must be a valid date in the format YYYY-MM-DD")
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	// get the user ID
	userID := app.contextGetUser(r).ID
	// get the personal finance prediction
	personalFinancePrediction, err := app.models.PersonalFinancePortfolio.GetPersonalFinanceDataForMonthByUserID(userID, startDate)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	// get goals analysis
	goals, err := app.models.FinancialManager.GetGoalsForUserInvestmentHelper(userID)
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
	info, err := app.models.PersonalFinancePortfolio.ProcessPersonalFinanceData(personalFinancePrediction)
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
		map[string]string{"X-API-KEY": app.config.api.apikeys.optivestmicroservice.key},
		info,
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
