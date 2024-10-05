package main

import (
	"errors"
	"time"

	"github.com/Blue-Davinci/OptiVest/internal/data"
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

// trackRecurringExpensesHandler() is a cronjob method that sets a cronJob to run at the end of every day
// to track all the recurring expenses for users
func (app *application) trackRecurringExpensesHandler() {
	app.logger.Info("Starting the recurring expenses tracking handler..", zap.String("time", time.Now().String()))
	updateInterval := "0 0 * * *"

	_, err := app.config.scheduler.trackRecurringExpenses.AddFunc(updateInterval, app.trackRecurringExpenses)
	if err != nil {
		app.logger.Error("Error adding [trackRecurringExpenses] to scheduler", zap.Error(err))
	}
	// Run the tracking first before starting the cron
	app.trackRecurringExpenses()
	// start the cron scheduler
	app.config.scheduler.trackRecurringExpenses.Start()
}

// trackGoalProgressStatus() is the method called by the cronjob to update the progress of all the goals that have expired
// It will be called every day at midnight to update the progress of the expired goals.
func (app *application) trackGoalProgressStatus() {
	app.logger.Info("Tracking goal progress status", zap.String("time", time.Now().String()))
	err := app.models.FinancialManager.UpdateGoalProgressOnExpiredGoals()
	if err != nil {
		switch {
		case errors.Is(err, data.ErrEditConflict):
			app.logger.Info("Edit conflict while updating the Goal progress status", zap.Error(err))
		default:
			app.logger.Error("Error tracking goal progress status", zap.Error(err))
		}
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

// trackRecurringExpenses() is the method called by the cronjob to track all the recurring expenses for users
// We will need to pass a burst and offset. After each burst, wwe recieve the  expenses than need to be tracked
// For each of those expenses, we add them to the expenses table after which we update the next tracking date
// of the current recurring expense.
// After processing we increment the offset by the burst and repeat the process until we get
// an ErrGenerealRecordNotFound error which just means we have no more expenses to track and we can stop
func (app *application) trackRecurringExpenses() {
	app.logger.Info("Tracking recurring expenses", zap.String("time", time.Now().String()))

	// Define burst size and start from the first page
	burst := app.config.limit.recurringExpenseTrackerBurstLimit
	currentPage := 1 // Start with page 1

	for {
		// Create filter with current page and burst size
		filter := data.Filters{
			Page:     currentPage,
			PageSize: burst,
		}

		// Retrieve expenses that need to be tracked for the current page
		recurringExpensesToTrack, metadata, err := app.models.FinancialTrackingManager.GetAllRecurringExpensesDueForProcessing(filter)
		if err != nil {
			// Handle case where no more records are found, break out of the loop
			if errors.Is(err, data.ErrGeneralRecordNotFound) {
				app.logger.Info("No more recurring expenses to track", zap.Error(err))
				break
			}
			// Log any other errors and stop further processing
			app.logger.Error("Error tracking recurring expenses", zap.Error(err))
			break
		}

		// Process each recurring expense in the batch
		for _, recurringExpenseToTrack := range recurringExpensesToTrack {
			// Create a new expense record
			expense := &data.Expense{
				BudgetID:     recurringExpenseToTrack.BudgetID,
				Name:         recurringExpenseToTrack.Name,
				Category:     "recurring",
				Amount:       recurringExpenseToTrack.Amount,
				IsRecurring:  true,
				Description:  recurringExpenseToTrack.Description,
				DateOccurred: time.Now(),
			}

			// Add the expense to the expenses table
			err := app.models.FinancialTrackingManager.CreateNewExpense(recurringExpenseToTrack.UserID, expense)
			if err != nil {
				app.logger.Error("Error adding recurring expense to expenses table", zap.Error(err))
				continue
			}

			// Update the next tracking date for the current recurring expense
			recurringExpenseToTrack.CalculateNextOccurrence()
			err = app.models.FinancialTrackingManager.UpdateRecurringExpenseByID(recurringExpenseToTrack.UserID, recurringExpenseToTrack)
			if err != nil {
				app.logger.Error("Error updating recurring expense", zap.Error(err))
				continue
			}
		}

		// Check if this is the last page of records
		if metadata.LastPage == metadata.CurrentPage {
			app.logger.Info("All recurring expenses processed. Ending tracking.")
			break
		}

		// Move to the next page
		currentPage = metadata.CurrentPage + 1
	}
}
