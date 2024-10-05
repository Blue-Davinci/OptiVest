package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

func (app *application) server() error {
	// declare our http server
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", app.config.port),
		Handler:      app.routes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}
	// make a channel to listen for shutdown signals
	shutdownChan := make(chan error)
	// start a background routine, this will listen to any shutdown signals
	go func() {
		// make a quit channel
		quit := make(chan os.Signal, 1)
		// listen for the SIGINT and SIGTERM signals
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		// read signal from the quit channel and will wait till there is an actual signal
		s := <-quit
		// printout the signal details
		app.logger.Info("shutting down server", zap.String("signal", s.String()))
		// make a 20sec context
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		err := srv.Shutdown(ctx)
		if err != nil {
			shutdownChan <- err
		}
		// Log a message to say that we're waiting for any background goroutines to
		// complete their tasks.
		app.logger.Info("completing background tasks...", zap.String("addr", srv.Addr))
		// wait for any background tasks to complete
		app.wg.Wait()
		//stop the cron job schedulers
		app.stopCronJobs(
			app.config.scheduler.trackMonthlyGoalsCron,
			app.config.scheduler.trackGoalProgressStatus,
			app.config.scheduler.trackExpiredGroupInvitations,
			app.config.scheduler.trackRecurringExpenses,
		)
		// Call Shutdown() on our server, passing in the context we just made.
		shutdownChan <- srv.Shutdown(ctx)
	}()
	// start the server printing out our main settings
	app.logger.Info("starting server", zap.String("addr", srv.Addr), zap.String("env", app.config.env))
	if err := srv.ListenAndServe(); err != nil {
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	}
	// Otherwise, we wait to receive the return value from Shutdown() on the
	// shutdownError channel. If return value is an error, we know that there was a
	// problem with the graceful shutdown and we return the error.
	err := <-shutdownChan
	if err != nil {
		return err
	}
	// Exiting....
	app.logger.Info("stopped server", zap.String("addr", srv.Addr))
	return nil
}

// stopCronJobs() essentially stopns all the cron jobs that are running in the application
func (app *application) stopCronJobs(cronJobs ...*cron.Cron) {
	app.logger.Info("stopping cron jobs..", zap.Int("count", len(cronJobs)))
	for _, cronJob := range cronJobs {
		ctx := cronJob.Stop()
		<-ctx.Done()
	}

}
