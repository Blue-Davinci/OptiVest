package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

// Server timing constants. ReadHeaderTimeout is the most important addition
// here: without it a single slow client can occupy a goroutine indefinitely
// (Slowloris). It applies only to the request header phase, so it is safe to
// set tightly even on the SSE server which streams responses for hours.
const (
	apiReadHeaderTimeout = 5 * time.Second
	apiIdleTimeout       = time.Minute
	apiReadTimeout       = 10 * time.Second
	apiWriteTimeout      = 60 * time.Second

	sseReadHeaderTimeout = 5 * time.Second
	sseReadTimeout       = 10 * time.Second
	// SSE responses are open-ended streams. WriteTimeout=0 disables the
	// per-connection write deadline; correctness is enforced by the per-
	// notification timeout in PublishNotification (see notifications.go).
	sseWriteTimeout = 0
	sseIdleTimeout  = 0

	gracefulShutdownTimeout = 20 * time.Second
)

// connIDCounter is the source for per-connection IDs injected via ConnContext.
// An atomic int64 is sufficient: monotonic, contention-free, and the absolute
// value carries no security meaning (it is only used to group log lines).
var connIDCounter atomic.Int64

// connContext is the per-connection context provider for both http.Servers.
// It stamps a monotonically-increasing connection ID into the request's
// context so log entries from the same TCP connection can be correlated.
func connContext(ctx context.Context, _ net.Conn) context.Context {
	return context.WithValue(ctx, connIDContextKey, connIDCounter.Add(1))
}

// apiServerConfig returns an http.Server pre-configured with the production
// hardening for the main API: tight ReadHeaderTimeout for Slowloris
// protection, ErrorLog routed through zap, BaseContext tied to the
// application's lifecycle so shutdown cancels in-flight handlers, and
// ConnContext stamping a per-connection ID for log correlation.
//
// The handler is injected so tests can substitute a no-op mux without having
// to materialize the full routes() tree (which depends on the expvar metric
// publishers and the entire middleware chain).
func (app *application) apiServerConfig(handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              fmt.Sprintf(":%d", app.config.port),
		Handler:           handler,
		ErrorLog:          zap.NewStdLog(app.logger),
		ReadHeaderTimeout: apiReadHeaderTimeout,
		ReadTimeout:       apiReadTimeout,
		WriteTimeout:      apiWriteTimeout,
		IdleTimeout:       apiIdleTimeout,
		BaseContext:       func(_ net.Listener) context.Context { return app.ctx },
		ConnContext:       connContext,
	}
}

// sseServerConfig returns the SSE HTTP server, pre-configured with the same
// hardening as apiServerConfig but with WriteTimeout disabled because SSE
// responses are long-lived streams. Per-message correctness is enforced
// instead by the timeout in PublishNotification (see notifications.go).
func (app *application) sseServerConfig(handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              fmt.Sprintf(":%d", app.config.ws.port),
		Handler:           handler,
		ErrorLog:          zap.NewStdLog(app.logger),
		ReadHeaderTimeout: sseReadHeaderTimeout,
		ReadTimeout:       sseReadTimeout,
		WriteTimeout:      sseWriteTimeout,
		IdleTimeout:       sseIdleTimeout,
		BaseContext:       func(_ net.Listener) context.Context { return app.ctx },
		ConnContext:       connContext,
	}
}

// buildAPIServer wires the production routes into apiServerConfig. Kept as a
// thin wrapper so tests can call apiServerConfig directly with a stub handler.
func (app *application) buildAPIServer() *http.Server {
	return app.apiServerConfig(app.routes())
}

// buildSSEServer wires the production SSE routes into sseServerConfig.
func (app *application) buildSSEServer() *http.Server {
	return app.sseServerConfig(app.sseRoutes())
}

// server starts both the API and SSE HTTP servers and orchestrates a single
// graceful-shutdown path triggered by app.ctx (which itself fans out from
// signal.NotifyContext in main()).
//
// Previously the SSE server was launched as `go app.serveSSE()` with errors
// only logged internally — a bind failure on the SSE port would silently keep
// the process running. Both servers now feed errors back into a single channel
// so any startup failure is fatal.
func (app *application) server() error {
	srv := app.buildAPIServer()
	sseSrv := app.buildSSEServer()

	// shutdownChan carries the result of the graceful-shutdown attempt.
	shutdownChan := make(chan error, 1)

	// serveErrChan carries unexpected errors from either ListenAndServe call.
	// Buffered to two so neither goroutine ever blocks on send if shutdown
	// fires first.
	serveErrChan := make(chan error, 2)

	// Graceful shutdown coordinator: waits for app.ctx to cancel, then drains
	// both servers within gracefulShutdownTimeout, waits for app.wg-tracked
	// background goroutines, and finally stops every cron scheduler.
	go func() {
		<-app.ctx.Done()
		app.logger.Info("shutting down servers",
			zap.String("api_addr", srv.Addr),
			zap.String("sse_addr", sseSrv.Addr),
		)

		ctx, cancel := context.WithTimeout(context.Background(), gracefulShutdownTimeout)
		defer cancel()

		apiErr := srv.Shutdown(ctx)
		sseErr := sseSrv.Shutdown(ctx)

		app.logger.Info("completing background tasks...")
		app.wg.Wait()

		app.stopCronJobs(
			app.config.scheduler.trackMonthlyGoalsCron,
			app.config.scheduler.trackGoalProgressStatus,
			app.config.scheduler.trackExpiredGroupInvitations,
			app.config.scheduler.trackRecurringExpenses,
			app.config.scheduler.trackOverdueDebts,
			app.config.scheduler.trackExpiredNotifications,
			app.config.scheduler.rssFeedScraper,
		)

		shutdownChan <- errors.Join(apiErr, sseErr)
	}()

	go func() {
		app.logger.Info("starting SSE server",
			zap.Int("port", app.config.ws.port),
		)
		if err := sseSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErrChan <- fmt.Errorf("sse server: %w", err)
		}
	}()

	app.logger.Info("starting API server",
		zap.String("addr", srv.Addr),
		zap.String("env", app.config.env),
	)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("api server: %w", err)
	}

	// At this point ListenAndServe has returned either http.ErrServerClosed
	// (graceful shutdown is in progress) or never returned because the SSE
	// server fell over first. Wait for one of: the SSE serve loop reporting
	// an error, or the shutdown coordinator finishing.
	select {
	case err := <-serveErrChan:
		return err
	case err := <-shutdownChan:
		if err != nil {
			return err
		}
	}

	app.logger.Info("stopped servers", zap.String("addr", srv.Addr))
	return nil
}

// stopCronJobs stops every cron scheduler in turn and blocks until each one's
// Stop()-returned context fires, guaranteeing in-flight job runs have settled
// before the process exits.
func (app *application) stopCronJobs(cronJobs ...*cron.Cron) {
	app.logger.Info("stopping cron jobs..", zap.Int("count", len(cronJobs)))
	for _, cronJob := range cronJobs {
		ctx := cronJob.Stop()
		<-ctx.Done()
	}
}
