package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Blue-Davinci/OptiVest/internal/data"
	"github.com/Blue-Davinci/OptiVest/internal/database"
)

type LLMAnalyzedPortfolio struct {
	// string, map[string]interface{}, string
	Header   string
	Analysis map[string]interface{}
	Footer   string
}
type UserPortfolioProfile struct {
	UserTimeHorizon    database.NullTimeHorizonType   `json:"user_time_horizon"`
	UserRiskTolerance  database.NullRiskToleranceType `json:"user_risk_tolerance"`
	InvestmentGoals    *data.InvestmentGoal           `json:"investment_goals"`
	InvestmentAnalysis data.InvestmentAnalysis        `json:"investment_analysis"`
}
type UserPersonalFinanceProfile struct {
	UserTimeHorizon         database.NullTimeHorizonType   `json:"user_time_horizon"`
	UserRiskTolerance       database.NullRiskToleranceType `json:"user_risk_tolerance"`
	PersonalFinanceAnalysis data.UnifiedFinanceAnalysis    `json:"personal_finance_analysis"`
}
type Chunk struct {
	ID                string   `json:"id"`
	Object            string   `json:"object"`
	Model             string   `json:"model"`
	SystemFingerprint string   `json:"system_fingerprint"`
	Choices           []Choice `json:"choices"`
}

type Choice struct {
	Index        int     `json:"index"`
	Delta        Delta   `json:"delta"`
	FinishReason *string `json:"finish_reason"`
}

type Delta struct {
	Content string `json:"content"`
}

// buildLLMRequest sends a request to the LLM API with the user's profile and investment analysis data
// and returns the analyzed portfolio data.
func (app *application) buildInvestmentPortfolioLLMRequest(user *data.User, goals *data.InvestmentGoal, investmentAnalysis *data.InvestmentAnalysis) (*LLMAnalyzedPortfolio, error) {
	// Create a profile
	profile := UserPortfolioProfile{
		UserTimeHorizon:    user.TimeHorizon,
		UserRiskTolerance:  user.RiskTolerance,
		InvestmentGoals:    goals,
		InvestmentAnalysis: *investmentAnalysis,
	}
	instructions := `{
    "messages": [
	{
	  "role": "system",
	  "content": "You are a financial assistant that provides detailed portfolio analysis, personalized recommendations, and rebalancing plans. Your task is to analyze a user's financial portfolio, which may include stocks, bonds, alternative investments, and goals. Use the provided financial data to compute performance metrics such as ROI, Sharpe ratio, Sortino ratio, and sector performance. Additionally, consider the user's risk tolerance, time horizon, and external economic factors such as inflation and interest rates. Your output must be structured strictly in JSON format and organized into five key sections: \n\n1. 'summary': Provide a high-level overview of the user's portfolio performance and the general state of each asset class (stocks, bonds, alternatives). Mention any significant trends or red flags. The output should be JSON formatted, including fields like 'portfolio_value', 'alternative_investments_value', 'total_net_worth', 'asset_allocation', and 'sector_allocation'. \n\n2. 'metrics': Include specific financial metrics for each asset (e.g., ROI, Sharpe ratio, Sortino ratio, sector performance). The response should break this down by asset class and individual investments, all formatted as JSON. Example fields: 'sharpe_ratio', 'sortino_ratio', 'roi', 'volatility', 'beta', 'treynor_ratio', 'jensen_alpha'. \n\n3. 'actions': Suggest actions to improve portfolio performance, such as rebalancing, changing allocations, or buying/selling assets. These actions should be clear, measurable, and formatted in JSON with fields like 'rebalancing', 'sector_rotation', 'tax_loss_harvesting', 'diversification'. \n\n4. 'recommended_investment': Recommend specific investment options or asset classes to consider, based on the user's risk tolerance, goals, and external factors. Each recommendation should be structured as a JSON array. Example fields: 'stocks', 'bonds', 'alternatives'. Each object in these arrays should include the asset 'symbol' or 'investment_type' and 'weight'. \n\n5. 'overall_summary': Conclude with an overarching summary that ties together all recommendations and provides a long-term outlook for the portfolio, including risk mitigation strategies. The output must be JSON, containing fields like 'portfolio_risk', 'expected_return', 'market_sentiment', 'interest_rate_outlook', 'inflation_outlook'. \n\nAdditionally, in this section, provide a detailed **text summary** of the overall analysis that clearly summarizes all key findings, insights, and recommendations for the user. This text summary should give a concise but comprehensive overview of the entire portfolio analysis in plain language."
	},
	{
	  "role": "user",
	  "content": "Analyze the following portfolio and provide detailed insights and actionable recommendations for: \n1) Asset allocation (e.g., how much to invest in stocks, bonds, alternatives). \n2) Sector allocation (e.g., performance of technology, healthcare, etc.). \n3) Diversification (e.g., balancing between high-risk and low-risk investments). \n4) Performance (e.g., calculating ROI, Sharpe ratio, volatility). \n5) Market sentiment (based on external factors like interest rate hikes and inflation). \n6) Provide a detailed rebalancing plan and investment suggestions tailored to the user's goals, risk tolerance, and current market conditions. The response must be returned in fully structured JSON format for each of the aforementioned sections, including a text summary of the entire analysis in the 'overall_summary' section."
	},
        {
            "role": "user",
            "content": %s
        }
    ],
    "stop": [
        "<|eot_id|>"
    ],
    "model": "Meta-Llama-3.1-405B-Instruct",
    "stream": true,
    "stream_options": {
        "include_usage": true
    }
}`
	return app.buildLLMRequestHelper(profile, instructions)
}

func (app *application) buildPersonalFinanceLLMRequest(user *data.User, unifiedPersonalFinanceAnalysis *data.UnifiedFinanceAnalysis) (*LLMAnalyzedPortfolio, error) {
	// Create a profile
	profile := UserPersonalFinanceProfile{
		UserTimeHorizon:         user.TimeHorizon,
		UserRiskTolerance:       user.RiskTolerance,
		PersonalFinanceAnalysis: *unifiedPersonalFinanceAnalysis,
	}

	instructions := `{
		"messages": [
		  {
			"role": "system",
			"content": "You are a financial assistant that analyzes personal finances, providing detailed budget optimization suggestions, savings opportunities, and debt management strategies. Your task is to analyze the user's financial profile, including income, expenses, debts, savings, and goals. Ensure that your response is strictly in JSON format, structured into five sections: \n\n1. 'summary': Provide a high-level overview of your cash flow, budget utilization, and debt situation. Mention any significant trends, such as over-budget behavior, surplus funds, or debt accumulation. If there are no debts, mention this explicitly. Additionally, provide suggestions for increasing income, such as side jobs, investment opportunities, or skill improvements. This section should include fields like 'cash_flow_summary', 'total_income', 'total_expenses', 'total_debt', 'budget_utilization', 'over_budget', 'debt_trend', and 'income_opportunities'. Handle missing data by indicating 'data unavailable' in relevant fields.\n\n2. 'metrics': Include specific financial metrics, such as income vs. expenses, savings rate, debt-to-income ratio, and progress towards goals. The output should be formatted as JSON and include fields like 'income_vs_expenses', 'savings_rate', 'debt_to_income_ratio', and 'progress_towards_goals' (an array with goal names and progress percentages). If data is missing, clearly indicate so.\n\n3. 'actions': Suggest actionable steps for optimizing your financial health. These steps can include budget adjustments, debt payoff strategies, or reallocating funds towards key goals. Ensure that the actions are measurable and specific, including fields like 'budget_optimization', 'debt_payoff_plan', 'fund_reallocation' (an array indicating which funds to move and where), and 'emergency_fund_recommendation'. Additionally, include fund reallocations and debt payoff percentages in decimal format for frontend parsing.\n\n4. 'recommended_goal_adjustments': Suggest adjustments to goal priorities if you are behind on key financial goals. Structure this as an array with fields like 'goal_name', 'recommended_funds_allocation', and 'progress_needed'. If goals are met, state this clearly.\n\n5. 'overall_summary': Provide an overarching summary that ties together all recommendations and offers long-term financial advice. This section should contain fields like 'summary_text' and 'financial_advice'. Finally, provide a **text summary** as a field in the JSON that summarizes all findings and recommendations in plain language. \n\nEnsure that any missing data (e.g., no debts) is handled gracefully, and percentages are provided in decimal format where applicable for easy use on the frontend."
		  },
		  {
			"role": "user",
			"content": "Analyze the following financial data for the user and provide detailed suggestions based on their specific income, expenses, debts, budgets, and goals. Include sections for summary, metrics, actions, recommended goal adjustments, and overall recommendations."
		  },
		  {
			"role": "user",
			"content": %s
		  }
		],
		"stop": [
		  "<|eot_id|>"
		],
		"model": "Meta-Llama-3.1-405B-Instruct",
		"stream": true,
		"stream_options": {
		  "include_usage": true
		}
	  }`

	return app.buildLLMRequestHelper(profile, instructions)
}

// buildOCRRecieptAnalysisRequest sends a request to the LLM API with the OCR analysis data
// and returns the analyzed OCR data.
func (app *application) buildOCRRecieptAnalysisLLMRequest(ocrAnalysis *data.OCRResponse) (*LLMAnalyzedPortfolio, error) {
	// no profile for this one, let us create instructions
	instructions := `
{
  "messages": [
    {
      "role": "system",
      "content": "You are tasked with analyzing OCR-processed data from a financial services company and extracting relevant details into a structured JSON format. The JSON output should contain the following fields: items_purchased, total_amount_spent, date_of_purchase, store_name, and payment_method. If any of these fields are missing or unclear, include null for that field. Handle edge cases such as incomplete or unclear data by including an 'error' key to describe any issues encountered. Avoid generating any code."
    },
    {
      "role": "user",
      "content": "Analyze the following OCR processed data and return the results in a structured JSON format including: 1) Items purchased, 2) Total amount spent, 3) Date of purchase, 4) Store name, 5) Payment method. If any data is missing, please indicate that in the JSON response using null values. If there is an error or the data is unclear, include an 'error' key with a description of the issue."
    },
    {
      "role": "user",
      "content": %s
    }
  ],
  "stop": ["<|eot_id|>"],
  "model": "Meta-Llama-3.1-405B-Instruct",
  "stream": true,
  "stream_options": {
    "include_usage": true
  }
}
`
	return app.buildLLMRequestHelper(ocrAnalysis, instructions)
}

// buildLLMRequestHelper builds and sends the LLM request for analysis
func (app *application) buildLLMRequestHelper(profile interface{}, instructionsTemplate string) (*LLMAnalyzedPortfolio, error) {
	// Marshal the profile data
	profileData, err := json.Marshal(profile)
	if err != nil {
		app.logger.Info("Error marshalling profile data")
		return nil, err
	}

	// Create the final LLM request
	finalLLMRequest := fmt.Sprintf(instructionsTemplate, string(profileData))
	//app.logger.Info("Final LLM Request:", zap.Any("final_llm_request", finalLLMRequest))

	// Send the request
	url := app.config.api.apikeys.sambanova.url
	apiKey := app.config.api.apikeys.sambanova.key

	fullResponse, err := app.LLMRequest(url, map[string]string{
		"Authorization": "Bearer " + apiKey,
	}, finalLLMRequest)
	if err != nil {
		return nil, err
	}

	// Log the full response
	//app.logger.Info("Full Response:", zap.String("full_response", fullResponse))

	// Parse the response
	header, llmAnalysis, footer, err := parseLLMResponse(fullResponse)
	if err != nil {
		return nil, err
	}

	// Return the analyzed portfolio
	llmAnalyzedPortfolio := &LLMAnalyzedPortfolio{
		Header:   header,
		Analysis: llmAnalysis,
		Footer:   footer,
	}
	app.logger.Info("Done parsing LLM response and building analyzed portfolio")
	return llmAnalyzedPortfolio, nil
}

// parseLLMResponse parses the response from the LLM API
// and returns the top section, JSON data, and bottom section.
func parseLLMResponse(response string) (string, map[string]interface{}, string, error) {
	// Split the response into parts using "```json" as the first preference
	sections := strings.SplitN(response, "```json", 2)
	if len(sections) < 2 {
		// If "```json" is not found, try splitting by just "```"
		sections = strings.SplitN(response, "```", 2)
		if len(sections) < 2 {
			return "", nil, "", errors.New("no JSON section found")
		}
	}

	// Get the non-JSON top section
	topSection := strings.TrimSpace(sections[0])

	// Split the remaining parts to extract the JSON and bottom sections
	jsonAndBottom := strings.SplitN(sections[1], "```", 2)
	if len(jsonAndBottom) < 2 {
		return "", nil, "", errors.New("incomplete JSON section")
	}

	// Trim the JSON part and handle potential extra spaces or formatting issues
	jsonPart := strings.TrimSpace(jsonAndBottom[0])

	// Check if the JSON part is empty
	if jsonPart == "" {
		return topSection, nil, strings.TrimSpace(jsonAndBottom[1]), errors.New("parsed JSON part is empty")
	}

	// Parse the JSON part
	var jsonData map[string]interface{}
	err := json.Unmarshal([]byte(jsonPart), &jsonData)
	if err != nil {
		return "", nil, "", fmt.Errorf("error parsing JSON: %w", err)
	}

	// Get the remaining bottom section
	bottomSection := strings.TrimSpace(jsonAndBottom[1])

	return topSection, jsonData, bottomSection, nil
}
