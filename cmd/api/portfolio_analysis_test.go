package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Blue-Davinci/OptiVest/internal/data"
	"golang.org/x/sync/singleflight"
)

// makeStockWorkers builds n no-op workers that sleep for `each` and return
// a non-nil error from worker index `errAt` (or nil if errAt < 0). The
// returned counter records peak concurrency observed across all workers.
func makeStockWorkers(n int, each time.Duration, errAt int) ([]portfolioWorker, *int64, *int64) {
	var inFlight, peak int64
	workers := make([]portfolioWorker, 0, n)
	for i := 0; i < n; i++ {
		i := i
		workers = append(workers, portfolioWorker{
			label: fmt.Sprintf("stock:%d", i),
			do: func(ctx context.Context) error {
				cur := atomic.AddInt64(&inFlight, 1)
				defer atomic.AddInt64(&inFlight, -1)
				for {
					old := atomic.LoadInt64(&peak)
					if cur <= old || atomic.CompareAndSwapInt64(&peak, old, cur) {
						break
					}
				}
				select {
				case <-time.After(each):
				case <-ctx.Done():
					return ctx.Err()
				}
				if i == errAt {
					return errors.New("worker boom")
				}
				return nil
			},
		})
	}
	return workers, &inFlight, &peak
}

// TestRunPortfolioWorkers_RespectsWorkerLimit verifies bounded concurrency:
// peak in-flight workers never exceeds the configured limit even when many
// workers are submitted.
func TestRunPortfolioWorkers_RespectsWorkerLimit(t *testing.T) {
	cases := []struct{ workers, limit int }{
		{12, 1}, // serial fallback path
		{12, 4},
		{24, 6},
		{24, 8},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("workers=%d_limit=%d", c.workers, c.limit), func(t *testing.T) {
			workers, _, peak := makeStockWorkers(c.workers, 30*time.Millisecond, -1)
			if err := runPortfolioWorkers(context.Background(), c.limit, workers); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := atomic.LoadInt64(peak); got > int64(c.limit) {
				t.Fatalf("peak in-flight=%d exceeded configured limit=%d", got, c.limit)
			}
		})
	}
}

// TestRunPortfolioWorkers_ConcurrentFasterThanSerial asserts the orchestrator
// actually runs work in parallel: with limit=N and N workers each sleeping
// `d`, total wall time must be closer to d than to N*d. We use 4x slack to
// stay robust on slow CI runners but still catch a regression to serial.
func TestRunPortfolioWorkers_ConcurrentFasterThanSerial(t *testing.T) {
	const (
		n     = 8
		each  = 50 * time.Millisecond
		limit = n // all in flight at once
	)
	workers, _, _ := makeStockWorkers(n, each, -1)
	start := time.Now()
	if err := runPortfolioWorkers(context.Background(), limit, workers); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	dur := time.Since(start)
	serialBudget := time.Duration(n) * each
	maxAllowed := each * 4 // ~200ms ceiling for 50ms work; serial would be 400ms
	if dur >= serialBudget {
		t.Fatalf("expected concurrent execution <%v, got %v (serial budget %v)", maxAllowed, dur, serialBudget)
	}
	if dur > maxAllowed {
		t.Fatalf("execution slower than expected: got %v, want <%v (each=%v)", dur, maxAllowed, each)
	}
}

// TestRunPortfolioWorkers_FirstErrorCancelsSiblings verifies errgroup
// semantics: when one worker errors, the derived ctx is cancelled and
// in-flight siblings observe ctx.Done() and return ctx.Err(); g.Wait()
// returns the first error.
func TestRunPortfolioWorkers_FirstErrorCancelsSiblings(t *testing.T) {
	const (
		n     = 6
		limit = 6
	)
	var siblingsCancelled int64
	workers := make([]portfolioWorker, 0, n)
	for i := 0; i < n; i++ {
		i := i
		workers = append(workers, portfolioWorker{
			label: fmt.Sprintf("worker:%d", i),
			do: func(ctx context.Context) error {
				if i == 0 {
					// Brief delay so siblings are guaranteed to be in flight.
					time.Sleep(20 * time.Millisecond)
					return errors.New("boom")
				}
				select {
				case <-time.After(2 * time.Second):
					// should not get here in <2s
					return nil
				case <-ctx.Done():
					atomic.AddInt64(&siblingsCancelled, 1)
					return ctx.Err()
				}
			},
		})
	}
	start := time.Now()
	err := runPortfolioWorkers(context.Background(), limit, workers)
	dur := time.Since(start)
	if err == nil || err.Error() != "boom" {
		t.Fatalf("expected first error 'boom', got %v", err)
	}
	if dur > 500*time.Millisecond {
		t.Fatalf("siblings did not cancel promptly: took %v", dur)
	}
	if atomic.LoadInt64(&siblingsCancelled) < int64(n-1) {
		t.Fatalf("expected %d siblings to observe cancellation, got %d", n-1, siblingsCancelled)
	}
}

// TestRunPortfolioWorkers_ErrMapperRunsForBondSentinel verifies the bond
// soft-degrade path: a worker returning an error matching "failed to get*"
// is mapped to data.ErrFailedToGetBondData via the worker's errMapper.
func TestRunPortfolioWorkers_ErrMapperRunsForBondSentinel(t *testing.T) {
	bondMapper := func(err error) error {
		if err != nil && containsFailedToGet(err.Error()) {
			return data.ErrFailedToGetBondData
		}
		return err
	}
	worker := portfolioWorker{
		label: "bond:TLT",
		do: func(ctx context.Context) error {
			return errors.New("failed to get bond data: upstream 503")
		},
		errMapper: bondMapper,
	}
	err := runPortfolioWorkers(context.Background(), 1, []portfolioWorker{worker})
	if !errors.Is(err, data.ErrFailedToGetBondData) {
		t.Fatalf("expected ErrFailedToGetBondData, got %v", err)
	}
}

func containsFailedToGet(s string) bool {
	// Local copy so the test does not depend on the production helper signature.
	const needle = "failed to get"
	for i := 0; i+len(needle) <= len(s); i++ {
		if s[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

// TestRunPortfolioWorkers_RespectsParentCancel ensures that cancelling the
// parent context (e.g. client disconnect) propagates: in-flight workers see
// ctx.Done() and the orchestrator returns context.Canceled (or the parent
// error), not nil.
func TestRunPortfolioWorkers_RespectsParentCancel(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	workers, _, _ := makeStockWorkers(8, 1*time.Second, -1)
	go func() {
		time.Sleep(40 * time.Millisecond)
		cancel()
	}()
	start := time.Now()
	err := runPortfolioWorkers(parent, 8, workers)
	dur := time.Since(start)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if dur > 500*time.Millisecond {
		t.Fatalf("workers did not abort on parent cancel: took %v", dur)
	}
}

// TestSingleflightDoTyped_CollapsesIdenticalKeys verifies that N concurrent
// callers asking for the same key issue exactly one underlying fetch and all
// receive the same value, and that the collapsed counter increments.
func TestSingleflightDoTyped_CollapsesIdenticalKeys(t *testing.T) {
	var sf singleflight.Group
	var fetches int64
	const n = 10

	// Reset the counter so we can assert against a known starting point.
	startCollapsed := portfolioSingleflightCollapsed.Value()

	var wg sync.WaitGroup
	results := make([]int, n)
	errs := make([]error, n)
	wg.Add(n)
	for i := 0; i < n; i++ {
		i := i
		go func() {
			defer wg.Done()
			v, err := singleflightDoTyped(&sf, "test:dedup-key", func() (int, error) {
				atomic.AddInt64(&fetches, 1)
				time.Sleep(50 * time.Millisecond)
				return 42, nil
			})
			results[i] = v
			errs[i] = err
		}()
	}
	wg.Wait()

	if got := atomic.LoadInt64(&fetches); got != 1 {
		t.Fatalf("expected exactly 1 underlying fetch, got %d", got)
	}
	for i, v := range results {
		if v != 42 {
			t.Fatalf("caller %d got %d, want 42", i, v)
		}
		if errs[i] != nil {
			t.Fatalf("caller %d unexpected err: %v", i, errs[i])
		}
	}
	endCollapsed := portfolioSingleflightCollapsed.Value()
	if endCollapsed-startCollapsed < 1 {
		t.Fatalf("expected collapsed counter to increment, got delta=%d", endCollapsed-startCollapsed)
	}
}

// TestSingleflightDoTyped_PropagatesError ensures errors from the leader
// fetch are forwarded to all waiters (not swallowed).
func TestSingleflightDoTyped_PropagatesError(t *testing.T) {
	var sf singleflight.Group
	want := errors.New("upstream down")
	_, got := singleflightDoTyped(&sf, "test:err-key", func() (string, error) {
		return "", want
	})
	if !errors.Is(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// TestTrackWorker_GaugeAndHighWaterMark verifies the in-flight gauge and the
// max-observed counter behave correctly under concurrent enter/exit.
func TestTrackWorker_GaugeAndHighWaterMark(t *testing.T) {
	// Snapshot starting active count so test is order-independent. We do
	// NOT snapshot the HWM because trackWorker's HWM is process-wide and
	// monotonic; it may already be >= n from earlier tests, in which case
	// the CAS branch never fires and the value stays put. The invariant we
	// can prove is "HWM is at least the peak active in this test".
	startActive := portfolioWorkersActive.Load()

	const n = 16
	releases := make([]func(), n)
	var wg sync.WaitGroup
	wg.Add(n)
	ready := make(chan struct{})
	for i := 0; i < n; i++ {
		i := i
		go func() {
			defer wg.Done()
			done := trackWorker()
			releases[i] = done
			<-ready
		}()
	}
	// Wait for all workers to have entered (poll the gauge).
	deadline := time.Now().Add(2 * time.Second)
	for portfolioWorkersActive.Load() < startActive+n && time.Now().Before(deadline) {
		time.Sleep(2 * time.Millisecond)
	}
	if got := portfolioWorkersActive.Load(); got != startActive+n {
		t.Fatalf("expected active=%d, got %d", startActive+n, got)
	}
	if got, want := portfolioWorkersMaxObservedHigh.Load(), startActive+n; got < want {
		t.Fatalf("expected high-water mark >= %d (peak active), got %d", want, got)
	}
	close(ready)
	wg.Wait()
	for _, r := range releases {
		r()
	}
	if got := portfolioWorkersActive.Load(); got != startActive {
		t.Fatalf("expected active to return to %d, got %d", startActive, got)
	}
}

// TestRunPortfolioWorkers_RaceFreeManyWorkers stress-tests the orchestrator
// under -race with many workers and concurrent reads/writes on the gauge to
// catch any data races introduced by future refactors.
func TestRunPortfolioWorkers_RaceFreeManyWorkers(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in -short mode")
	}
	const n = 100
	workers, _, _ := makeStockWorkers(n, time.Millisecond, -1)
	if err := runPortfolioWorkers(context.Background(), 8, workers); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
