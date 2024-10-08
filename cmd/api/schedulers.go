package main

import (
	"database/sql"
	"errors"
	"time"

	"github.com/Blue-Davinci/OptiVest/internal/data"
	"go.uber.org/zap"
)

// trackMonthlyGoals() is a cronjob method that sets a cronJob to run at the end of every month
// to track all the goals that have been set by the users and update their progress
// We use 28-31 to ensure that the cronjob runs at the end of all different months
func (app *application) trackMonthlyGoalsScheduleHandler() {
	app.logger.Info("Starting the monthly goal tracking cron job..", zap.String("time", time.Now().String()))
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
	app.logger.Info("Starting the goal progress update tracking cron job..", zap.String("time", time.Now().String()))
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
	app.logger.Info("Starting the group invitation tracking cron job..", zap.String("time", time.Now().String()))
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
	app.logger.Info("Starting the recurring expenses tracking cron job..", zap.String("time", time.Now().String()))
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

// trackOverdueDebtsHandler() is the cronjob method that will track all overdue debts
func (app *application) trackOverdueDebtsHandler() {
	app.logger.Info("Starting the overdue debt tracking cron job..", zap.String("time", time.Now().String()))
	updateInterval := "0 0 * * *"

	_, err := app.config.scheduler.trackOverdueDebts.AddFunc(updateInterval, app.trackOverdueDebts)
	if err != nil {
		app.logger.Error("Error adding [trackOverdueDebts] to scheduler", zap.Error(err))
	}
	// Run the tracking first before starting the cron
	app.trackOverdueDebts()
	// start the cron scheduler
	app.config.scheduler.trackOverdueDebts.Start()
}

// trackExpiredNotificationsHandler() is the method called by the cronjob to track all expired notifications
// Will run every night
func (app *application) trackExpiredNotificationsHandler() {
	app.logger.Info("Starting the expired notifications tracking cron job..", zap.String("time", time.Now().String()))
	updateInterval := "0 0 * * *"

	_, err := app.config.scheduler.trackRecurringExpenses.AddFunc(updateInterval, app.trackExpiredNotifications)
	if err != nil {
		app.logger.Error("Error adding [trackRecurringExpenses] to scheduler", zap.Error(err))
	}
	// Run the tracking first before starting the cron
	app.trackExpiredNotifications()
	// start the cron scheduler
	app.config.scheduler.trackRecurringExpenses.Start()
}

// =================================================================================================================
// Handler Functions
// ==================================================================================================================

// trackGoalProgressStatus() is the method called by the cronjob to update the progress of all the goals that have expired
// It will be called every day at midnight to update the progress of the expired goals.
func (app *application) trackGoalProgressStatus() {
	app.logger.Info("Starting the goal progress status tracking cron job...", zap.String("time", time.Now().String()))
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
	app.logger.Info("Starting the monthly goals tracking cron job...", zap.String("time", time.Now().String()))
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
	app.logger.Info("Starting the expired group invitations tracking cron job..", zap.String("time", time.Now().String()))
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

// trackOverdueDebts() is the method called by the cronjob to track all the debts that are overdue
// Just like the trackRecurringExpenses, we will need to pass a burst and offset. After each burst, wwe recieve the  debts than need to be tracked
// or rather the debts that need accrued interest recalculation
// For each debt, recalculate the updated interest, update the accrued interest and remaining balance
// After that, save the updated debt record back, and send a notification to the user
// After processing we increment the offset by the burst and repeat the process until we get
// we are in the last page of the records.
func (app *application) trackOverdueDebts() {
	// burst
	burst := app.config.limit.overdueDebtTrackerBurstLimit
	currentPage := 1
	for {
		//make a filter
		filter := data.Filters{
			Page:     currentPage,
			PageSize: burst,
		}
		// get all overdue debt
		debts, metadata, err := app.models.FinancialTrackingManager.GetAllOverdueDebts(filter)
		if err != nil {
			if errors.Is(err, data.ErrGeneralRecordNotFound) {
				app.logger.Info("No more overdue debts to track", zap.Error(err))
				break
			}
			app.logger.Error("Error tracking overdue debts", zap.Error(err))
			break
		}
		for _, debt := range debts {
			//app.logger.Info("Processing debt ID:", zap.Int64("debtID", debt.ID))
			// calculate the updated interest
			accruedInterest, err := app.calculateInterestPayment(debt)
			if err != nil {
				app.logger.Info("Error calculating interest for debt ID:", (zap.Error(err)))
				continue
			}
			// Update the accrued interest and remaining balance
			debt.AccruedInterest = accruedInterest
			debt.RemainingBalance = debt.RemainingBalance.Add(accruedInterest) // Add interest to balance
			debt.InterestLastCalculated = time.Now()                           // Update interest last calculated date
			// Step 3: Save updated debt back to the database
			err = app.models.FinancialTrackingManager.UpdateDebtByID(debt.UserID, debt)
			if err != nil {
				app.logger.Info("Error updating debt", zap.Error(err))
				continue
			}
			// ToDO: SEND notification
		}
		// Check if this is the last page of records
		if metadata.LastPage == metadata.CurrentPage {
			app.logger.Info("All overdue debtsprocessed, ending tracking.")
			break
		}

		// Move to the next page
		currentPage = metadata.CurrentPage + 1

	}
}

// trackExpiredNotifications() is the method called by the cronjob to track all the notifications that have expired
// It will be called every day at midnight to update the expired notifications.
// We will use GetAllExpiredNotifications() to get all expired notifications passing in a filter
// to provide the limit and offset. We will loop through them, updating the status of each notification
// to expired saving them using UpdateNotificationReadAtAndStatus()
func (app *application) trackExpiredNotifications() {
	app.logger.Info("Tracking expired notifications", zap.String("time", time.Now().String()))
	// Define burst size and start from the first page
	burst := app.config.limit.expiredNotificationTrackerBurstLimit
	currentPage := 1 // Start with page 1
	for {
		// Create filter with current page and burst size
		filter := data.Filters{
			Page:     currentPage,
			PageSize: burst,
		}
		// Retrieve notifications that need to be tracked for the current page
		expiredNotifications, metadata, err := app.models.NotificationManager.GetAllExpiredNotifications(filter)
		if err != nil {
			// Handle case where no more records are found, break out of the loop
			if errors.Is(err, data.ErrGeneralRecordNotFound) {
				app.logger.Info("No more expired notifications to track")
				break
			}
			// Log any other errors and stop further processing
			app.logger.Error("Error tracking expired notifications", zap.Error(err))
			break
		}
		// Process each expired notification in the batch
		for _, expiredNotification := range expiredNotifications {
			// Update the status of the notification to expired
			err := app.models.NotificationManager.UpdateNotificationReadAtAndStatus(
				expiredNotification.ID,
				sql.NullTime{Time: time.Time{}, Valid: false},
				data.NotificationStatusTypeExpired,
			)
			if err != nil {
				app.logger.Error("Error updating notification status to expired", zap.Error(err))
				continue
			}
		}
		// Check if this is the last page of records
		if metadata.LastPage == metadata.CurrentPage {
			app.logger.Info("All expired notifications processed. Ending tracking.")
			break
		}
		// Move to the next page
		currentPage = metadata.CurrentPage + 1
	}
}
