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
func (app *application) buildLLMRequest(user *data.User, goals *data.InvestmentGoal, investmentAnalysis *data.InvestmentAnalysis) (*LLMAnalyzedPortfolio, error) {
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
	portfolioData, err := json.Marshal(profile)
	if err != nil {
		app.logger.Info("Error marshalling portfolio data")
		return nil, err
	}
	finalLLMRequest := fmt.Sprintf(instructions, string(portfolioData))

	// Send the request
	url := app.config.api.apikeys.sambanova.url
	apiKey := app.config.api.apikeys.sambanova.key

	fullResponse, err := app.LLMRequest(url, map[string]string{
		"Authorization": "Bearer " + apiKey,
	}, finalLLMRequest)
	if err != nil {
		return nil, err
	}

	// Now you can work with the fullResponse as needed
	//fmt.Println("Full Response:", fullResponse)
	header, llmAnalysis, footer, err := parseLLMResponse(fullResponse)
	if err != nil {
		return nil, err
	}
	// create a LLMAnalyzedPortfolio
	llmAnalyzedPortfolio := &LLMAnalyzedPortfolio{
		Header:   header,
		Analysis: llmAnalysis,
		Footer:   footer,
	}

	return llmAnalyzedPortfolio, nil
}

// parseLLMResponse parses the response from the LLM API
// and returns the top section, JSON data, and bottom section.
func parseLLMResponse(response string) (string, map[string]interface{}, string, error) {
	// Split the response into parts
	sections := strings.Split(response, "```json")
	if len(sections) < 2 {
		return "", nil, "", errors.New("no JSON section found")
	}

	// Get the non-JSON top section
	topSection := strings.TrimSpace(sections[0])

	// Split the remaining parts to extract the JSON and bottom sections
	jsonAndBottom := strings.Split(sections[1], "```")
	if len(jsonAndBottom) < 2 {
		return "", nil, "", errors.New("incomplete JSON section")
	}

	// Parse the JSON part
	var jsonData map[string]interface{}
	err := json.Unmarshal([]byte(strings.TrimSpace(jsonAndBottom[0])), &jsonData)
	if err != nil {
		return "", nil, "", err
	}

	// Get the remaining bottom section
	bottomSection := strings.TrimSpace(jsonAndBottom[1])

	return topSection, jsonData, bottomSection, nil
}
