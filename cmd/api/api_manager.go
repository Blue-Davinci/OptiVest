package main

import (
	"context"
	"fmt"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"

	"github.com/Blue-Davinci/OptiVest/internal/data"
)

// convertAndGetExchangeRate gets and returns the exchange rate between the
// source and target currencies. It first checks Redis for a cached value and
// falls back to the upstream exchange-rate API on miss, caching the resulting
// rate with the configured TTL.
//
// The caller's ctx flows into both the Redis read/write and the outbound HTTP
// call so that handler-side disconnects propagate cancellation downstream.
//
// API format is: https://v6.exchangerate-api.com/v6/<api-key>/pair/EUR/GBP
func (app *application) convertAndGetExchangeRate(ctx context.Context, source_currency, target_currency string) (*data.ExchangeRateResponse, error) {
	if source_currency == "" || target_currency == "" {
		return nil, data.ErrorEmptyCurrency
	}

	redisKey := fmt.Sprintf("%s:%s:%s", data.RedisExchangeRatePrefix, source_currency, target_currency)

	cachedRate, err := app.RedisDB.Get(ctx, redisKey).Result()
	if err == nil {
		app.logger.Info("Using cached exchange rate", zap.String("rate", cachedRate))
		conversionRate, err := decimal.NewFromString(cachedRate)
		if err == nil {
			return &data.ExchangeRateResponse{
				ConversionRate: conversionRate,
				BaseCode:       source_currency,
				TargetCode:     target_currency,
			}, nil
		}
	}

	url := fmt.Sprintf("%s/%s/pair/%s/%s", app.config.api.apikeys.exchangerates.url,
		app.config.api.apikeys.exchangerates.key, source_currency, target_currency)

	exchange, err := GETRequest[data.ExchangeRateResponse](ctx, app.http_client, url, nil)
	if err != nil {
		return nil, err
	}
	app.logger.Info("Fetched exchange rate",
		zap.String("from", source_currency),
		zap.String("to", target_currency),
		zap.String("rate", exchange.ConversionRate.String()),
	)
	// Cache-write failure is not fatal - we already have a valid upstream
	// response. Aborting the request here turned every Redis blip into a
	// user-visible 5xx on the FX path, which then cascaded into broken
	// portfolio analyses for any non-USD-denominated holdings.
	if err := app.RedisDB.Set(ctx, redisKey, exchange.ConversionRate.String(), data.APIExchangeCacheTTL).Err(); err != nil {
		app.logger.Error("Failed to cache exchange rate in Redis",
			zap.String("from", source_currency),
			zap.String("to", target_currency),
			zap.Error(err),
		)
	}

	return &exchange, nil
}
