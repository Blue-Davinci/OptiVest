package main

import (
	"expvar"
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
	// SSE middleware chain. Deliberately omits logRequests: SSE
	// connections are long-lived (minutes to hours) and a single
	// "request completed" line at disconnect time is more misleading than
	// useful. requestID still runs so that anything the SSE handler
	// itself logs is correlatable with the original HTTP request.
	sseMiddleware := alice.New(app.requestID, app.recoverPanic, app.authenticate, app.requireAuthenticatedUser, app.requireActivatedUser).Then
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
	// Global middleware chain, outer to inner:
	//   metrics      - expvar counters and httpsnoop wrapper for the whole
	//                  pipeline. Kept outermost so every request is at
	//                  least counted.
	//   requestID    - stamps X-Request-ID (passthrough or generated).
	//                  Must run before logRequests so the log line carries
	//                  the ID, and before authenticate so any authentication
	//                  log lines also carry it. Trivially side-effect-free,
	//                  safe to sit outside recoverPanic.
	//   logRequests  - one structured zap line per request. Critically must
	//                  sit OUTSIDE recoverPanic: httpsnoop.CaptureMetrics
	//                  re-propagates inner panics, so if recoverPanic were
	//                  outside, a panicking handler would never produce a
	//                  log line. With this ordering recoverPanic catches
	//                  the panic, writes a 500, and CaptureMetrics returns
	//                  cleanly with Code=500 so logRequests still emits
	//                  the (Error-level) line ops are paged on.
	//   recoverPanic - catches panics, returns 500.
	//   rateLimit    - per-IP GCRA via Redis. 429s still get one log line
	//                  because logRequests is outside.
	//   authenticate - reads bearer token, populates user context and the
	//                  requestLog holder for logRequests.
	globalMiddleware := alice.New(app.metrics, app.requestID, app.logRequests, app.recoverPanic, app.rateLimit, app.authenticate).Then

	// dynamic protected middleware
	dynamicMiddleware := alice.New(app.requireAuthenticatedUser, app.requireActivatedUser)

	// Operational endpoints. Deliberately attached to the base router with
	// only CORS in front of them, so scrape traffic bypasses the global
	// middleware chain that wraps /v1. That avoids three issues a Prometheus
	// scraper would otherwise cause:
	//   1. circular counting — every scrape would increment the very metrics
	//      it is trying to read (request_log_total, total_requests_received).
	//   2. log noise — at a 15s scrape interval logRequests would emit
	//      thousands of lines per scrape source per day.
	//   3. self-rate-limit — a fast scraper or a misconfigured cluster could
	//      exhaust the per-IP token bucket and start receiving 429s on the
	//      very endpoint operators rely on for diagnosis.
	// The endpoints are intentionally unauthenticated; deployments are
	// expected to scope reachability to the internal scrape network. See
	// SECURITY.md for the reasoning.
	router.Get("/healthcheck", app.healthcheckHandler)
	router.Get("/readyz", app.readyzHandler)
	router.Get("/metrics", app.prometheusMetricsHandler)
	router.Handle("/debug/vars", expvar.Handler())

	// Make our categorized routes. globalMiddleware is attached to v1Router,
	// not the base router, so /v1/* keeps its full chain (metrics, requestID,
	// logRequests, recoverPanic, rateLimit, authenticate) while /metrics and
	// /debug/vars stay clean.
	v1Router := chi.NewRouter()
	v1Router.Use(globalMiddleware)

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
	userRoutes.Post("/recovery", app.validateRecoveryCodeHandler)
	userRoutes.With(dynamicMiddleware.Then).Post("/mfa", app.setupMFAHandler)
	userRoutes.With(dynamicMiddleware.Then).Post("/mfa/verify", app.verifiy2FASetupHandler)
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
	apiKeyRoutes.Post("/authentication/verify", app.validateMFALoginAttemptHandler)
	// /password-reset : for sending keys for resetting passwords
	apiKeyRoutes.Post("/password-reset", app.createPasswordResetTokenHandler)
	apiKeyRoutes.Post("/recovery", app.initializeRecoveryByRecoveryCodes)
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
