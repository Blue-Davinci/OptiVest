package main

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/Blue-Davinci/OptiVest/internal/data"
)

// fakeTimeSeries returns a deterministic, hand-crafted TimeSeriesDailyResponse
// that lives entirely in memory. It is intentionally small so the tests run
// instantly, but contains enough days within the lookback window
// (time.Now().Year()-4 .. now) for performAndLogCalculations to produce
// non-trivial returns / Sharpe / Sortino values.
//
// Closing prices are deliberately monotone so the returns are easy to reason
// about: each successive close is +1.0 above the previous one.
func fakeTimeSeries(t *testing.T) *data.TimeSeriesDailyResponse {
	t.Helper()
	year := time.Now().Year()
	series := map[string]data.TimeSeriesDailyData{}
	// Generate ~120 trading-day-ish entries across the last 3 years so the
	// year-filter in getAverageDailyReturn definitely keeps them.
	d := time.Date(year-3, time.January, 4, 0, 0, 0, 0, time.UTC)
	close := decimal.NewFromFloat(100)
	for i := 0; i < 120; i++ {
		key := d.Format("2006-01-02")
		series[key] = data.TimeSeriesDailyData{
			Open:   close,
			High:   close.Add(decimal.NewFromFloat(0.5)),
			Low:    close.Sub(decimal.NewFromFloat(0.5)),
			Close:  close,
			Volume: decimal.NewFromInt(1000),
		}
		// Advance by 3 days to space things out into 4 years of coverage.
		d = d.AddDate(0, 0, 3)
		close = close.Add(decimal.NewFromFloat(1))
	}
	return &data.TimeSeriesDailyResponse{
		MetaData:        data.MetaData{},
		DailyTimeSeries: series,
	}
}

// observedApp builds an *application wrapped around an in-memory zap observer
// so tests can assert on the structured log lines the production code emits.
// The observer captures every log entry; calling .All() returns them in order.
func observedApp(t *testing.T) (*application, *observer.ObservedLogs) {
	t.Helper()
	core, logs := observer.New(zap.InfoLevel)
	return &application{
		logger: zap.New(core),
	}, logs
}

// TestPerformAndLogCalculations_Deterministic asserts the function is a pure
// transform: identical input must yield identical output across calls. This
// is the property the cache-hit path relies on.
func TestPerformAndLogCalculations_Deterministic(t *testing.T) {
	app, _ := observedApp(t)
	ts := fakeTimeSeries(t)
	rf := decimal.NewFromFloat(0.02)

	r1, sh1, so1 := app.performAndLogCalculations(context.Background(), ts, rf)
	r2, sh2, so2 := app.performAndLogCalculations(context.Background(), ts, rf)

	if !sh1.Equal(sh2) {
		t.Errorf("sharpe ratio non-deterministic: first=%s second=%s", sh1, sh2)
	}
	if !so1.Equal(so2) {
		t.Errorf("sortino ratio non-deterministic: first=%s second=%s", so1, so2)
	}
	if len(r1) != len(r2) {
		t.Fatalf("returns slice length mismatch: first=%d second=%d", len(r1), len(r2))
	}
	for i := range r1 {
		if !r1[i].Equal(r2[i]) {
			t.Errorf("return[%d] mismatch: first=%s second=%s", i, r1[i], r2[i])
		}
	}
}

// TestPerformAndLogCalculations_SingleAverageReturnLogPerCall is the direct
// regression test for the cache-hit duplicate-call bug. It calls the function
// exactly once and asserts exactly one "Average Daily Return" log line is
// emitted. Re-introducing the buggy double-call would produce two lines and
// fail this test loudly.
func TestPerformAndLogCalculations_SingleAverageReturnLogPerCall(t *testing.T) {
	app, logs := observedApp(t)
	ts := fakeTimeSeries(t)
	rf := decimal.NewFromFloat(0.02)

	app.performAndLogCalculations(context.Background(), ts, rf)

	got := logs.FilterMessage("Average Daily Return").Len()
	if got != 1 {
		t.Errorf("expected exactly 1 'Average Daily Return' log entry per call, got %d", got)
	}
}

// TestSharpeRatio_ZeroVolatility regression-tests the divide-by-zero guard.
// With identical returns the standard deviation is zero; before the guard,
// shopspring/decimal Div would panic and crash the stock pipeline. We expect
// a benign Zero sentinel instead.
func TestSharpeRatio_ZeroVolatility(t *testing.T) {
	flat := []decimal.Decimal{
		decimal.NewFromFloat(0.01),
		decimal.NewFromFloat(0.01),
		decimal.NewFromFloat(0.01),
	}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("sharpeRatio panicked on zero volatility: %v", r)
		}
	}()
	got := sharpeRatio(flat, decimal.NewFromFloat(0.005))
	if !got.IsZero() {
		t.Errorf("sharpeRatio with zero volatility = %s, want 0", got)
	}
}

// TestSortinoRatio_ZeroDownsideDeviation regression-tests the matching guard
// in sortinoRatio. A series with no negative returns has zero downside
// deviation; we want a Zero sentinel instead of a panic.
func TestSortinoRatio_ZeroDownsideDeviation(t *testing.T) {
	allUp := []decimal.Decimal{
		decimal.NewFromFloat(0.01),
		decimal.NewFromFloat(0.02),
		decimal.NewFromFloat(0.03),
	}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("sortinoRatio panicked on zero downside deviation: %v", r)
		}
	}()
	got := sortinoRatio(allUp, decimal.NewFromFloat(0.005))
	if !got.IsZero() {
		t.Errorf("sortinoRatio with zero downside = %s, want 0", got)
	}
}

// TestFilterTimeSeriesBetweenYears_ChronologicalOrder asserts the helper
// returns its results in date-ascending order regardless of input map
// iteration order. Without the sort step in the production code, returns
// computed from this slice (price[i]/price[i-1]) would be non-deterministic.
//
// We loop a few times to give Go's randomized map iteration a chance to
// expose unsorted output if the sort step is removed.
func TestFilterTimeSeriesBetweenYears_ChronologicalOrder(t *testing.T) {
	year := time.Now().Year()
	series := map[string]data.TimeSeriesDailyData{}
	// Insert dates in scrambled order; the year-stamp encodes the expected
	// ordinal so we can verify the output came out monotonically increasing.
	scrambled := []struct {
		date string
		ord  int
	}{
		{date: time.Date(year-2, time.March, 15, 0, 0, 0, 0, time.UTC).Format("2006-01-02"), ord: 3},
		{date: time.Date(year-3, time.January, 4, 0, 0, 0, 0, time.UTC).Format("2006-01-02"), ord: 1},
		{date: time.Date(year-2, time.November, 22, 0, 0, 0, 0, time.UTC).Format("2006-01-02"), ord: 4},
		{date: time.Date(year-3, time.July, 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02"), ord: 2},
		{date: time.Date(year-1, time.February, 9, 0, 0, 0, 0, time.UTC).Format("2006-01-02"), ord: 5},
	}
	for _, e := range scrambled {
		series[e.date] = data.TimeSeriesDailyData{
			Close:  decimal.NewFromInt(int64(e.ord)),
			Volume: decimal.NewFromInt(1000),
		}
	}
	resp := &data.TimeSeriesDailyResponse{DailyTimeSeries: series}

	for attempt := 0; attempt < 20; attempt++ {
		out := filterTimeSeriesBetweenYears(resp, year-3)
		if len(out) != len(scrambled) {
			t.Fatalf("attempt %d: filter dropped entries, got %d want %d", attempt, len(out), len(scrambled))
		}
		for i := 0; i < len(out); i++ {
			expected := decimal.NewFromInt(int64(i + 1))
			if !out[i].Close.Equal(expected) {
				t.Fatalf("attempt %d: out[%d].Close = %s, want %s (chronological order broken)",
					attempt, i, out[i].Close, expected)
			}
		}
	}
}
