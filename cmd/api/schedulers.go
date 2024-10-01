package main

import (
	"time"

	"go.uber.org/zap"
)

// trackMonthlyGoals() is a cronjob method that sets a cronJob to run at the end of every month
// to track all the goals that have been set by the users and update their progress
// We use 28-31 to ensure that the cronjob runs at the end of all different months
func (app *application) trackMonthlyGoalsScheduleHandler() {
	app.logger.Info("Starting the goal tracking handler..", zap.String("time", time.Now().String()))
	trackingInterval := "0 0 28-31 * *"

	_, err := app.config.scheduler.trackMonthlyGoalsCron.AddFunc(trackingInterval, app.trackMonthlyGoals)
	if err != nil {
		app.logger.Error("Error adding [trackMonthlyGoals] to scheduler", zap.Error(err))
	}
	// Run the tracking first before starting the cron
	app.trackMonthlyGoals()
	// start the cron scheduler
	app.config.scheduler.trackMonthlyGoalsCron.Start()
}

// trackMonthlyGoals() is the method called by the cronjob to track all the goals that have been set by the users
// It will be called every end of month at midnight to update the monthly goals.
// We call GetAndSaveAllGoalsForTracking() that performs both of this tasks:
// 1. Get all the goals that are due for tracking
// 2. Update the goals that are due for tracking
func (app *application) trackMonthlyGoals() {
	app.logger.Info("Tracking monthly goals", zap.String("time", time.Now().String()))
	now := time.Now()
	if app.isLastDayOfMonth(now) {
		trackedGoals, err := app.models.FinancialManager.GetAndSaveAllGoalsForTracking(int32(app.config.limit.monthlyGoalProcessingBatchLimit))
		if err != nil {
			app.logger.Error("Error tracking monthly goals", zap.Error(err))
		}
		// ToDO: Send Noitification to the users
		app.logger.Info("tracked monthly goals", zap.Int("tracked goal count", len(trackedGoals)))
	} else {
		app.logger.Info("not the last day of the month, skipping check", zap.String("time", now.String()))
	}
}
