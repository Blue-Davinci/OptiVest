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

// updateGoalProgressOnExpiredGoalsHandler() is a cronjob method that sets a cronJob to run at the end of every day
// to update the progress of all the goals that have expired
func (app *application) updateGoalProgressOnExpiredGoalsHandler() {
	app.logger.Info("Starting the goal progress update handler..", zap.String("time", time.Now().String()))
	updateInterval := "0 0 * * *"

	_, err := app.config.scheduler.trackGoalProgressStatus.AddFunc(updateInterval, app.trackGoalProgressStatus)
	if err != nil {
		app.logger.Error("Error adding [trackGoalProgressStatus] to scheduler", zap.Error(err))
	}
	// Run the tracking first before starting the cron
	app.trackGoalProgressStatus()
	// start the cron scheduler
	app.config.scheduler.trackGoalProgressStatus.Start()
}

// trackExpiredGroupInvitationsHandler() is a cronjob method that sets a cronJob to run at the end of every day
// to track all the group invitations that have expired
func (app *application) trackExpiredGroupInvitationsHandler() {
	app.logger.Info("Starting the group invitation tracking handler..", zap.String("time", time.Now().String()))
	updateInterval := "0 0 * * *"

	_, err := app.config.scheduler.trackExpiredGroupInvitations.AddFunc(updateInterval, app.trackExpiredGroupInvitations)
	if err != nil {
		app.logger.Error("Error adding [trackExpiredGroupInvitations] to scheduler", zap.Error(err))
	}
	// Run the tracking first before starting the cron
	app.trackExpiredGroupInvitations()
	// start the cron scheduler
	app.config.scheduler.trackExpiredGroupInvitations.Start()
}

// trackGoalProgressStatus() is the method called by the cronjob to update the progress of all the goals that have expired
// It will be called every day at midnight to update the progress of the expired goals.
func (app *application) trackGoalProgressStatus() {
	app.logger.Info("Tracking goal progress status", zap.String("time", time.Now().String()))
	err := app.models.FinancialManager.UpdateGoalProgressOnExpiredGoals()
	if err != nil {
		app.logger.Error("Error tracking goal progress status", zap.Error(err))
	}
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
		trackedGoals, err := app.models.FinancialManager.GetAndSaveAllGoalsForTracking()
		if err != nil {
			app.logger.Error("Error tracking monthly goals", zap.Error(err))
		}
		// ToDO: Send Noitification to the users
		app.logger.Info("tracked monthly goals", zap.Int("tracked goal count", len(trackedGoals)))
	} else {
		app.logger.Info("not the last day of the month, skipping check", zap.String("time", now.String()))
	}
}

// trackExpiredGroupInvitations() is the method called by the cronjob to track all the group invitations that have expired
// It will be called every day at midnight to update the expired group invitations.
func (app *application) trackExpiredGroupInvitations() {
	app.logger.Info("Tracking expired group invitations", zap.String("time", time.Now().String()))
	err := app.models.FinancialGroupManager.UpdateExpiredGroupInvitations()
	if err != nil {
		app.logger.Error("Error tracking expired group invitations", zap.Error(err))
	}
}
