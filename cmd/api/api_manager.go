package main

import (
	"context"
	"fmt"

	"github.com/Blue-Davinci/OptiVest/internal/data"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// convertAndGetExchangeRate() gets and returns the exchange rate by providing the source and target currencies.
// It will be used to make GET requests to exhcange rate API
// We first verify if the source and target currency are provided after which we verify
// if thos exchange rate has been cached in our REDIS database. If it is cached, we return the cached rate.
// If it is not cached, we make a GET request to the exchange rate API and cache the conversion rate in REDIS.
//
// Api format is: https://v6.exchangerate-api.com/v6/<api-key>/pair/EUR/GBP
func (app *application) convertAndGetExchangeRate(source_currency, target_currency string) (*data.ExchangeRateResponse, error) {
	// Quick validation
	if source_currency == "" || target_currency == "" {
		//app.logger.Error("Empty currency provided", zap.String("source_currency", source_currency), zap.String("target_currency", target_currency))
		return nil, data.ErrorEmptyCurrency
	}

	// Create a Redis key to save our exchange rate
	redisKey := fmt.Sprintf("%s:%s:%s", data.RedisExchangeRatePrefix, source_currency, target_currency)

	// Try to get the exchange rate from Redis
	cachedRate, err := app.RedisDB.Get(context.Background(), redisKey).Result()
	if err == nil {
		// Convert the cached rate to decimal
		app.logger.Info("Using cached exchange rate", zap.String("rate", cachedRate))
		conversionRate, err := decimal.NewFromString(cachedRate)
		if err == nil {
			// Construct the response with the cached rate
			return &data.ExchangeRateResponse{
				ConversionRate: conversionRate,
				BaseCode:       source_currency,
				TargetCode:     target_currency,
			}, nil
		}
	}

	// Construct the URL for the API call
	url := fmt.Sprintf("%s/%s/pair/%s/%s", app.config.api.apikeys.exchangerates.url,
		app.config.api.apikeys.exchangerates.key, source_currency, target_currency)

	// Make the API request
	exchange, err := GETRequest[data.ExchangeRateResponse](app.http_client, url, nil)
	if err != nil {
		return nil, err
	}
	app.logger.Info("Got exchange rate", zap.Any("exchange", exchange))
	// Cache only the conversion rate in Redis with TTL
	err = app.RedisDB.Set(context.Background(), redisKey, exchange.ConversionRate.String(), data.APIExchangeCacheTTL).Err() // Set TTL to 1 hour
	if err != nil {
		return nil, err
	}

	return &exchange, nil
}
