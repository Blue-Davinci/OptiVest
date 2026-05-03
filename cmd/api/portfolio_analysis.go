package main

import (
	"context"
	"errors"
	"expvar"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Blue-Davinci/OptiVest/internal/data"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/singleflight"
)

// ---------------------------------------------------------------------------
// expvar metrics for the investment-portfolio analysis pipeline.
//
// All counters are monotonic int64s; gauges are atomic.Int64s exposed via
// expvar.Func so they reflect live values on each /debug/vars scrape rather
// than a snapshot at process start.
//
// Operationally these are the four signals SRE should watch:
//   - portfolio_analysis_runs_total            -> request rate
//   - portfolio_analysis_errors_total          -> error rate (alert on ratio)
//   - portfolio_analysis_duration_ms           -> last run latency (p99 via scrape)
//   - portfolio_analysis_workers_active        -> live concurrency (saturation)
//
// portfolio_singleflight_collapsed_total tells you how often the in-flight
// dedup actually saved an upstream call; a sustained zero suggests either the
// cache is warm enough to never miss in parallel or the workload is too
// sequential to benefit, which is fine.
// ---------------------------------------------------------------------------

var (
	portfolioAnalysisRuns           = expvar.NewInt("portfolio_analysis_runs_total")
	portfolioAnalysisErrors         = expvar.NewInt("portfolio_analysis_errors_total")
	portfolioAnalysisDurationMS     = expvar.NewInt("portfolio_analysis_duration_ms")
	portfolioSingleflightCollapsed  = expvar.NewInt("portfolio_singleflight_collapsed_total")
	portfolioWorkersActive          atomic.Int64
	portfolioWorkersMaxObservedHigh atomic.Int64
)

// init wires the gauges. Counters are already published via expvar.NewInt.
// We only need expvar.Publish for the live-read gauges.
func init() {
	expvar.Publish("portfolio_analysis_workers_active", expvar.Func(func() any {
		return portfolioWorkersActive.Load()
	}))
	expvar.Publish("portfolio_analysis_workers_max_observed", expvar.Func(func() any {
		return portfolioWorkersMaxObservedHigh.Load()
	}))
}

// trackWorker increments the in-flight gauge, updates the high-water mark,
// and returns a function that the caller defers to decrement the gauge.
// Using atomic operations means we do not introduce lock contention into the
// hot per-asset path.
func trackWorker() func() {
	cur := portfolioWorkersActive.Add(1)
	for {
		hi := portfolioWorkersMaxObservedHigh.Load()
		if cur <= hi || portfolioWorkersMaxObservedHigh.CompareAndSwap(hi, cur) {
			break
		}
	}
	return func() { portfolioWorkersActive.Add(-1) }
}

// singleflightDoTyped is a thin generic wrapper around singleflight.Group.Do
// that returns a typed value (avoiding the interface{} cast at every callsite)
// and increments portfolio_singleflight_collapsed_total when the result was
// shared with at least one waiter. The shared bool is exactly what
// singleflight.Group exposes for this purpose, so no extra bookkeeping is
// required.
//
// The key must be globally unique for the resource being fetched (e.g.
// "fmp:sector-performance", "av:timeseries:AAPL"). Keys collide silently if
// two different fetchers share one, so prefix per-vendor.
func singleflightDoTyped[T any](sf *singleflight.Group, key string, fn func() (T, error)) (T, error) {
	v, err, shared := sf.Do(key, func() (any, error) {
		return fn()
	})
	if shared {
		portfolioSingleflightCollapsed.Add(1)
	}
	if err != nil {
		var zero T
		return zero, err
	}
	out, ok := v.(T)
	if !ok {
		var zero T
		return zero, fmt.Errorf("singleflight: type assertion failed for key %q", key)
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// performInvestmentPortfolioAnalysis: bounded-concurrency variant.
//
// The previous implementation walked StockAnalysis and BondAnalysis serially.
// For a typical portfolio (10-15 assets * ~2s of upstream HTTP per asset) that
// produced ~25-30s of wall-clock latency on the cache-cold path, even though
// each per-asset call is independent.
//
// This version uses a bounded errgroup (cap = cfg.portfolio.workerLimit) so
// up to N workers run in parallel:
//   - first non-nil error cancels the derived ctx -> in-flight upstream HTTP
//     calls and DB INSERTs see ctx.Done() and abort cleanly (already wired
//     end-to-end after the P2 data-layer ctx propagation refactor)
//   - the worker cap respects upstream API rate limits (default 6 maps to
//     ~12 Alpha Vantage calls per analysis, well under the Premium 75/min)
//   - bond-error mapping (failed-to-get -> data.ErrFailedToGetBondData) is
//     preserved by translating inside the worker closure so g.Wait() returns
//     the sentinel
//
// Race-safety: each goroutine writes to a *different* element of the slice
// (&investmentAnalysis.StockAnalysis[i] / &investmentAnalysis.BondAnalysis[i]),
// so there is no aliasing. Verified with -race in the test suite.
//
// Backwards compatibility: setting -portfolio-worker-limit=1 reproduces the
// previous serial behaviour exactly (errgroup with limit 1 is sequential).
// ---------------------------------------------------------------------------
func (app *application) performInvestmentPortfolioAnalysis(ctx context.Context, investmentAnalysis *data.InvestmentAnalysis, user *data.User) error {
	portfolioAnalysisRuns.Add(1)
	start := time.Now()
	defer func() {
		portfolioAnalysisDurationMS.Set(time.Since(start).Milliseconds())
	}()

	if string(user.TimeHorizon.TimeHorizonType) == "" {
		user.TimeHorizon = app.models.Users.MapTimeHorizonTypeToConstant("short")
	}

	// Risk-free rate is a single FRED call shared by every per-asset worker;
	// fetch it once before fanning out so all workers reuse the same value.
	riskFreeRate, err := app.getRiskMetrics(ctx, string(user.TimeHorizon.TimeHorizonType))
	if err != nil {
		portfolioAnalysisErrors.Add(1)
		return err
	}

	workers := make([]portfolioWorker, 0, len(investmentAnalysis.StockAnalysis)+len(investmentAnalysis.BondAnalysis))
	for i := range investmentAnalysis.StockAnalysis {
		stock := &investmentAnalysis.StockAnalysis[i]
		workers = append(workers, portfolioWorker{
			label: "stock:" + stock.StockSymbol,
			do: func(wctx context.Context) error {
				return app.updateStockAnalysis(wctx, user.ID, stock, riskFreeRate)
			},
		})
	}
	for i := range investmentAnalysis.BondAnalysis {
		bond := &investmentAnalysis.BondAnalysis[i]
		workers = append(workers, portfolioWorker{
			label: "bond:" + bond.BondSymbol,
			do: func(wctx context.Context) error {
				return app.updateBondAnalysis(wctx, user.ID, bond, riskFreeRate)
			},
			// Preserve the legacy mapping: any "failed to get*" upstream
			// failure becomes the sentinel so the handler can soft-degrade
			// instead of returning 500. A follow-up could replace this with
			// an errors.Is sentinel at the data-fetch layer.
			errMapper: func(err error) error {
				if err != nil && strings.Contains(err.Error(), "failed to get") {
					return data.ErrFailedToGetBondData
				}
				return err
			},
		})
	}

	limit := app.portfolioWorkerLimit()
	if err := runPortfolioWorkers(ctx, limit, workers); err != nil {
		portfolioAnalysisErrors.Add(1)
		// Do not log ErrFailedToGetBondData as an error: it is a known
		// soft-degrade path the handler treats as a partial-success case.
		if !errors.Is(err, data.ErrFailedToGetBondData) {
			app.logger.Error("portfolio analysis worker error",
				zap.Int64("user_id", user.ID),
				zap.Int("worker_limit", limit),
				zap.Error(err))
		}
		return err
	}

	app.logger.Info("portfolio analysis completed",
		zap.Int64("user_id", user.ID),
		zap.Int("stocks", len(investmentAnalysis.StockAnalysis)),
		zap.Int("bonds", len(investmentAnalysis.BondAnalysis)),
		zap.Int("worker_limit", limit),
		zap.String("risk_free_rate", riskFreeRate.String()),
		zap.Duration("duration", time.Since(start)),
	)
	// Alternative-investment analysis is intentionally not wired up here yet
	// (matches pre-P3 behaviour). When it lands it should plug into the same
	// errgroup loop above so it shares the worker cap and cancellation.
	return nil
}

// portfolioWorker is one unit of analysis work (a stock or a bond). do is
// invoked on a goroutine inside runPortfolioWorkers; errMapper, if non-nil,
// translates a non-nil error from do into a domain sentinel (e.g.
// failed-to-get -> ErrFailedToGetBondData).
type portfolioWorker struct {
	label     string
	do        func(ctx context.Context) error
	errMapper func(error) error
}

// runPortfolioWorkers fans the supplied workers out into a bounded errgroup
// (cap = limit) and waits for all of them. First non-nil error cancels the
// derived context, in-flight workers see ctx.Done() and abort. The function
// is the single source of truth for the analysis concurrency policy and is
// exercised directly by the test suite with mock workers.
func runPortfolioWorkers(ctx context.Context, limit int, workers []portfolioWorker) error {
	if limit < 1 {
		// Defensive: validateConfig rejects this at boot, but never call
		// errgroup.SetLimit(0) which would block Go forever.
		limit = 1
	}
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(limit)
	for _, w := range workers {
		w := w
		g.Go(func() error {
			done := trackWorker()
			defer done()
			err := w.do(gctx)
			if err != nil && w.errMapper != nil {
				err = w.errMapper(err)
			}
			return err
		})
	}
	return g.Wait()
}

// portfolioWorkerLimit returns the configured concurrency cap, falling back
// to a serial limit of 1 if the runtime config has been mutated to an
// invalid value (validateConfig already rejects this at boot).
func (app *application) portfolioWorkerLimit() int {
	if app.config.portfolio.workerLimit < 1 {
		return 1
	}
	return app.config.portfolio.workerLimit
}
