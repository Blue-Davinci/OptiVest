package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/justinas/alice"
)

func (app *application) sseRoutes() http.Handler {
	router := chi.NewRouter()
	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   app.config.cors.trustedOrigins,
		AllowedMethods:   []string{"GET"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"link"},
		AllowCredentials: false,
		MaxAge:           300, // Maximum value not ignored bycls any of major browsers
	}))
	//Use alice to make a global middleware chain.
	sseMiddleware := alice.New(app.recoverPanic, app.authenticate, app.requireAuthenticatedUser, app.requireActivatedUser).Then
	// Make our categorized routes
	v1Router := chi.NewRouter()
	v1Router.With(sseMiddleware).Get("/sse", app.ServeSSE)
	// Moount the v1Router to the main base router
	router.Mount("/v1", v1Router)
	return router
}

// routes() is a method that returns a http.Handler that contains all the routes for the application
func (app *application) routes() http.Handler {
	router := chi.NewRouter()
	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   app.config.cors.trustedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "PATCH"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"link"},
		AllowCredentials: false,
		MaxAge:           300, // Maximum value not ignored by any of major browsers
	}))
	//Use alice to make a global middleware chain.
	globalMiddleware := alice.New(app.metrics, app.recoverPanic, app.rateLimit, app.authenticate).Then

	// dynamic protected middleware
	dynamicMiddleware := alice.New(app.requireAuthenticatedUser, app.requireActivatedUser)

	// Apply the global middleware to the router
	router.Use(globalMiddleware)

	// Make our categorized routes
	v1Router := chi.NewRouter()

	v1Router.Mount("/users", app.userRoutes(&dynamicMiddleware))
	v1Router.Mount("/api", app.apiKeyRoutes())
	v1Router.With(dynamicMiddleware.Then).Mount("/budgets", app.budgetRoutes())
	v1Router.With(dynamicMiddleware.Then).Mount("/goals", app.goalRoutes())
	v1Router.With(dynamicMiddleware.Then).Mount("/groups", app.groupRoutes())
	v1Router.With(dynamicMiddleware.Then).Mount("/incomes", app.incomeRouter())
	v1Router.With(dynamicMiddleware.Then).Mount("/debts", app.debtRoutes())
	v1Router.With(dynamicMiddleware.Then).Mount("/expenses", app.expenseRoutes())
	v1Router.With(dynamicMiddleware.Then).Mount("/investments", app.investmentPortfolioRoutes())
	v1Router.With(dynamicMiddleware.Then).Mount("/personalfinance", app.personalFinanceRoutes())
	v1Router.With(dynamicMiddleware.Then).Mount("/feeds", app.feedRoutes())
	v1Router.With(dynamicMiddleware.Then).Mount("/awards", app.awardRoutes())
	v1Router.With(dynamicMiddleware.Then).Mount("/search-options", app.searchOptionRoutes())
	v1Router.With(dynamicMiddleware.Then).Mount("/notifications", app.notifications())
	v1Router.With(dynamicMiddleware.Then).Mount("/comments", app.comments())
	// mount general routes directly
	v1Router.Post("/contact-us", app.createContactUsHandler)

	// Moount the v1Router to the main base router
	router.Mount("/v1", v1Router)
	return router
}

// userRoutes() is a method that returns a chi.Router that contains all the routes for the users
func (app *application) userRoutes(dynamicMiddleware *alice.Chain) chi.Router {
	userRoutes := chi.NewRouter()
	userRoutes.Post("/", app.registerUserHandler)
	// /activation : for activating accounts
	userRoutes.Put("/activated", app.activateUserHandler)
	userRoutes.Put("/password", app.updateUserPasswordHandler)
	userRoutes.With(dynamicMiddleware.Then).Patch("/mfa", app.setupMFAHandler)
	userRoutes.With(dynamicMiddleware.Then).Patch("/mfa/verify", app.verifiy2FASetupHandler)
	// account
	userRoutes.With(dynamicMiddleware.Then).Get("/account", app.getUserInformationHandler)
	userRoutes.With(dynamicMiddleware.Then).Patch("/account", app.updateUserInformationHandler)
	// /logout : for logging out
	userRoutes.With(dynamicMiddleware.Then).Post("/logout", app.logoutUserHandler)
	return userRoutes
}

// apiKeyRoutes() is a method that returns a chi.Router that contains all the routes for the api keys
func (app *application) apiKeyRoutes() chi.Router {
	apiKeyRoutes := chi.NewRouter()
	// initial request for token
	apiKeyRoutes.Post("/authentication", app.createAuthenticationApiKeyHandler)
	apiKeyRoutes.Patch("/authentication/verify", app.validateMFALoginAttemptHandler)
	// /password-reset : for sending keys for resetting passwords
	apiKeyRoutes.Post("/password-reset", app.createPasswordResetTokenHandler)
	// manual token request
	apiKeyRoutes.Post("/resend-activation", app.createManualActivationTokenHandler)
	return apiKeyRoutes
}

// budgetRoutes() is a method that returns a chi.Router that contains all the routes for the budgets
func (app *application) budgetRoutes() chi.Router {
	budgetRoutes := chi.NewRouter()
	budgetRoutes.Get("/", app.getBudgetsForUserHandler)
	budgetRoutes.Get("/summary", app.getBudgetGoalExpenseSummaryHandler)
	budgetRoutes.Post("/", app.createNewBudgetdHandler)
	budgetRoutes.Patch("/{budgetID}", app.updateBudgetHandler)
	budgetRoutes.Delete("/{budgetID}", app.deleteBudgetByIDHandler)
	return budgetRoutes
}

// goalRoutes() is a method that returns a chi.Router that contains all the routes for the goals
func (app *application) goalRoutes() chi.Router {
	goalRoutes := chi.NewRouter()
	goalRoutes.Post("/", app.createNewGoalHandler)
	goalRoutes.Patch("/{goalID}", app.updatedGoalHandler)
	goalRoutes.Get("/progression", app.getAllGoalsWithProgressionByUserIDHandler)
	goalRoutes.Get("/tracking", app.getGoalTrackingHistoryHandler)
	// /plan : for creating a new plan for a goal
	goalRoutes.Post("/plan", app.createNewGoalPlanHandler)
	goalRoutes.Patch("/plan/{goalPlanID}", app.updatedGoalPlanHandler)
	goalRoutes.Get("/plan", app.getGoalPlansForUserHandler)
	return goalRoutes
}

// groupRoutes() is a method that returns a chi.Router that contains all the routes for the user groups
func (app *application) groupRoutes() chi.Router {
	groupRoutes := chi.NewRouter()
	groupRoutes.Get("/", app.getAllGroupsUserIsMemberOfHandler)
	groupRoutes.Get("/{groupID}", app.getDetailedGroupByIdHandler)
	groupRoutes.Post("/", app.createNewUserGroupHandler)
	groupRoutes.Patch("/{groupID}", app.updateUserGroupHandler)

	// members
	groupRoutes.Patch("/member/{groupID}", app.updateGroupUserRoleHandler)
	groupRoutes.Delete("/member/{groupID}/{memberID}", app.adminDeleteGroupMemberHandler) // admin deletion {},{}
	groupRoutes.Delete("/member/{groupID}", app.userLeaveGroupHandler)                    // user leaving {}

	// get for creators
	groupRoutes.Get("/created", app.getAllGroupsCreatedByUserHandler)

	// group invitations
	groupRoutes.Post("/invite", app.createNewGroupInvitation)
	groupRoutes.Patch("/invite/{groupID}", app.updateGroupInvitationStatusHandler)

	// group goals
	groupRoutes.Post("/goal", app.createNewGroupGoalHandler)
	groupRoutes.Patch("/goal/{groupGoalID}", app.updateGroupGoalHandler)

	// group Transactions
	groupRoutes.Get("/transactions/{groupID}", app.getGroupTransactionsByGroupIdHandler)
	groupRoutes.Post("/transactions", app.createNewGroupTransactionHandler)
	groupRoutes.Delete("/transactions/{groupTransactionID}", app.deleteGroupTransactionHandler)

	// group Expenses
	groupRoutes.Get("/expenses/{groupID}", app.getGroupExpensesByGroupIdHandler)
	groupRoutes.Post("/expenses", app.createNewGroupExpenseHandler)
	groupRoutes.Delete("/expenses/{groupExpenseID}", app.deleteGroupExpenseHandler)

	// Public groups
	groupRoutes.Get("/public", app.getAllPublicGroupsHandler)
	groupRoutes.Post("/public", app.createNewPublicMembershipHandler)
	return groupRoutes
}

// expenseRoutes() is a method that returns a chi.Router that contains all the routes for the expenses
func (app *application) expenseRoutes() chi.Router {
	expenseRoutes := chi.NewRouter()
	expenseRoutes.Get("/", app.getAllExpensesByUserIDHandler)
	expenseRoutes.Post("/", app.createNewExpenseHandler)
	expenseRoutes.Patch("/{expenseID}", app.updateExpenseByIDHandler)
	expenseRoutes.Post("/recurring", app.createNewRecurringExpenseHandler)
	expenseRoutes.Get("/recurring", app.getAllRecurringExpensesByUserIDHandler)
	expenseRoutes.Patch("/recurring/{expenseID}", app.updateRecurringExpenseByIDHandler)

	expenseRoutes.Post("/receipts", app.getOCRDRecieptDataAnalysisHandler)

	return expenseRoutes
}

func (app *application) incomeRouter() chi.Router {
	incomeRoutes := chi.NewRouter()
	incomeRoutes.Get("/", app.getAllIncomesByUserIDHandler)
	incomeRoutes.Post("/", app.createNewIncomeHandler)
	incomeRoutes.Patch("/{incomeID}", app.updateIncomeHandler)
	return incomeRoutes
}

func (app *application) debtRoutes() chi.Router {
	debtRoutes := chi.NewRouter()
	debtRoutes.Get("/", app.getAllDebtsByUserIDHandler)
	debtRoutes.Post("/", app.createNewDebtHandler)
	debtRoutes.Patch("/{debtID}", app.updateDebtHandler)

	//installment
	debtRoutes.Get("/installment/{debtID}", app.getDebtPaymentsByDebtUserIDHandler)
	debtRoutes.Patch("/installment/{debtID}", app.makeDebtPaymentHandler)

	return debtRoutes
}

func (app *application) investmentPortfolioRoutes() chi.Router {
	investmentPortfolioRoutes := chi.NewRouter()
	// stocks
	investmentPortfolioRoutes.Get("/stocks", app.getAllStockInvestmentByUserIDHandler)
	investmentPortfolioRoutes.Post("/stocks", app.createNewStockInvestmentHandler)
	investmentPortfolioRoutes.Patch("/stocks/{stockID}", app.updateStockInvestmentHandler)
	investmentPortfolioRoutes.Delete("/stocks/{stockID}", app.deleteStockInvestmentByIDHandler)
	// bonds
	investmentPortfolioRoutes.Get("/bonds", app.getAllBondInvestmentByUserIDHandler)
	investmentPortfolioRoutes.Post("/bonds", app.createNewBondInvestmentHandler)
	investmentPortfolioRoutes.Patch("/bonds/{bondID}", app.updateBondInvestmentHandler)
	investmentPortfolioRoutes.Delete("/bonds/{bondID}", app.deleteBondInvestmentByIDHandler)
	// alternative investments
	investmentPortfolioRoutes.Get("/alternative", app.getAllAlternativeInvestmentByUserIDHandler)
	investmentPortfolioRoutes.Post("/alternative", app.createNewAlternativeInvestmentHandler)
	investmentPortfolioRoutes.Patch("/alternative/{alternativeID}", app.updateAlternativeInvestmentHandler)
	investmentPortfolioRoutes.Delete("/alternative/{alternativeID}", app.deleteAlternativeInvestmentByIDHandler)
	// investment transactiona
	investmentPortfolioRoutes.Post("/transactions", app.createNewInvestmentTransactionHandler)
	investmentPortfolioRoutes.Delete("/transactions/{transactionID}", app.deleteInvestmentTransactionByIDHandler)

	// Analysis
	investmentPortfolioRoutes.Get("/analysis", app.investmentPrtfolioAnalysisHandler)
	investmentPortfolioRoutes.Get("/analysis/summary", app.getLatestLLMAnalysisResponseByUserIDHandler)
	return investmentPortfolioRoutes
}

// feedRoutes() is a method that returns a chi.Router that contains all the routes for the feeds
func (app *application) feedRoutes() chi.Router {
	feedRoutes := chi.NewRouter()
	feedRoutes.Get("/", app.getAllRSSPostWithFavoriteTagsHandler)
	feedRoutes.Post("/", app.createNewFeedHandler)

	feedRoutes.Get("/{postID}", app.getRssFeedPostByIDHandler)
	feedRoutes.Patch("/{feedID}", app.updateFeedHandler)
	feedRoutes.Delete("/{feedID}", app.deleteFeedByIDHandler)

	// favorites
	feedRoutes.Post("/favorites", app.createNewFavoriteOnPostHandler)
	feedRoutes.Delete("/favorites/{postID}", app.deleteFavoriteOnPostHandler)

	return feedRoutes
}

// personalFinanceRoutes() is a method that returns a chi.Router that contains all the routes for the personal finance
func (app *application) personalFinanceRoutes() chi.Router {
	personalFinanceRoutes := chi.NewRouter()
	personalFinanceRoutes.Get("/analysis", app.getAllFinanceDetailsForAnalysisByUserIDHandler)
	personalFinanceRoutes.Get("/summary", app.getAllInvestmentInfoByUserIDHandler)
	personalFinanceRoutes.Get("/prediction", app.getPersonalFinancePrediction)
	personalFinanceRoutes.Get("/expense-income/summary", app.getExpenseIncomeSummaryReportHandler)
	return personalFinanceRoutes
}

// awardRoutes() is a method that returns a chi.Router that contains all the routes for the awards
func (app *application) awardRoutes() chi.Router {
	awardRoutes := chi.NewRouter()
	awardRoutes.Get("/", app.getAllAwardsForUserByIDHandler)
	return awardRoutes
}

// searchOptionRoutes() is a method that returns a chi.Router that contains all the routes for the search options
func (app *application) searchOptionRoutes() chi.Router {
	searchOptionRoutes := chi.NewRouter()
	searchOptionRoutes.Get("/budget-categories", app.getDistinctBudgetCategoryHandler)
	searchOptionRoutes.Get("/currencies", app.getAllCurrencyHandler)
	searchOptionRoutes.Get("/budget-id-names", app.getDistinctBudgetIdBudgetNameHandler)
	return searchOptionRoutes
}

// notifications() is a method that returns a chi.Router that contains all the routes for the notifications
func (app *application) notifications() chi.Router {
	notificationRoutes := chi.NewRouter()
	notificationRoutes.Get("/", app.getAllNotificationsByUserIdHandler)
	notificationRoutes.Patch("/{notificationID}", app.updatedNotificationHandler)

	notificationRoutes.Delete("/{notificationID}", app.deleteNotificationByIdHandler)
	notificationRoutes.Delete("/", app.deleteAllNotificationsByUserIdHandler)
	return notificationRoutes
}

// comments() is a method that returns a chi.Router that contains all the routes for the comments
func (app *application) comments() chi.Router {
	commentRoutes := chi.NewRouter()
	commentRoutes.Get("/", app.getCommentsWithReactionsByAssociatedIdHandler)
	commentRoutes.Post("/", app.createNewCommentHandler)
	commentRoutes.Patch("/{commentID}", app.updateCommentHandler)
	commentRoutes.Delete("/{commentID}", app.deleteCommentHandler)

	// Reaction
	commentRoutes.Post("/reaction", app.createNewReactionHandler)
	commentRoutes.Delete("/reaction/{commentID}", app.deleteReactionHandler)
	return commentRoutes
}
