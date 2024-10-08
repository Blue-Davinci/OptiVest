package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/justinas/alice"
)

func (app *application) wsRoutes() http.Handler {
	router := chi.NewRouter()
	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   app.config.cors.trustedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "PATCH"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"link"},
		AllowCredentials: false,
		MaxAge:           300, // Maximum value not ignored by any of major browsers
	}))
	router.Get("/ws", app.wsHandler)
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
	v1Router.With(dynamicMiddleware.Then).Mount("/investments", app.investmentPortfolioRoutes())
	//expenses
	v1Router.With(dynamicMiddleware.Then).Mount("/expenses", app.expenseRoutes())

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
	userRoutes.With(dynamicMiddleware.Then).Patch("/mfa", app.setupMFAHandler)
	userRoutes.With(dynamicMiddleware.Then).Patch("/mfa/verify", app.verifiy2FASetupHandler)
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
	return apiKeyRoutes
}

// budgetRoutes() is a method that returns a chi.Router that contains all the routes for the budgets
func (app *application) budgetRoutes() chi.Router {
	budgetRoutes := chi.NewRouter()
	budgetRoutes.Get("/", app.getBudgetsForUserHandler)
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
	// /plan : for creating a new plan for a goal
	goalRoutes.Post("/plan", app.createNewGoalPlanHandler)
	goalRoutes.Patch("/plan/{goalPlanID}", app.updatedGoalPlanHandler)
	goalRoutes.Get("/plan", app.getGoalPlansForUserHandler)
	return goalRoutes
}

// groupRoutes() is a method that returns a chi.Router that contains all the routes for the user groups
func (app *application) groupRoutes() chi.Router {
	groupRoutes := chi.NewRouter()
	groupRoutes.Post("/", app.createNewUserGroupHandler)
	groupRoutes.Patch("/{groupID}", app.updateUserGroupHandler)

	// group invitations
	groupRoutes.Post("/invite", app.createNewGroupInvitation)
	groupRoutes.Patch("/invite/{groupID}", app.updateGroupInvitationStatusHandler)

	// group goals
	groupRoutes.Post("/goal", app.createNewGroupGoalHandler)
	groupRoutes.Patch("/goal/{groupGoalID}", app.updateGroupGoalHandler)

	// group Transactions
	groupRoutes.Post("/transactions", app.createNewGroupTransactionHandler)
	groupRoutes.Delete("/transactions/{groupTransactionID}", app.deleteGroupTransactionHandler)

	// group Expenses
	groupRoutes.Post("/expenses", app.createNewGroupExpenseHandler)
	groupRoutes.Delete("/expenses/{groupExpenseID}", app.deleteGroupExpenseHandler)

	return groupRoutes
}

// expenseRoutes() is a method that returns a chi.Router that contains all the routes for the expenses
func (app *application) expenseRoutes() chi.Router {
	expenseRoutes := chi.NewRouter()
	expenseRoutes.Post("/", app.createNewExpenseHandler)
	expenseRoutes.Patch("/{expenseID}", app.updateExpenseByIDHandler)
	expenseRoutes.Post("/recurring", app.createNewRecurringExpenseHandler)
	expenseRoutes.Patch("/recurring/{expenseID}", app.updateRecurringExpenseByIDHandler)

	return expenseRoutes
}

func (app *application) incomeRouter() chi.Router {
	incomeRoutes := chi.NewRouter()
	incomeRoutes.Post("/", app.createNewIncomeHandler)
	incomeRoutes.Patch("/{incomeID}", app.updateIncomeHandler)
	return incomeRoutes
}

func (app *application) debtRoutes() chi.Router {
	debtRoutes := chi.NewRouter()
	debtRoutes.Post("/", app.createNewDebtHandler)
	debtRoutes.Patch("/{debtID}", app.updateDebtHandler)

	//installment
	debtRoutes.Patch("/installment/{debtID}", app.makeDebtPaymentHandler)
	return debtRoutes
}

func (app *application) investmentPortfolioRoutes() chi.Router {
	investmentPortfolioRoutes := chi.NewRouter()
	investmentPortfolioRoutes.Post("/stocks", app.createNewStockInvestmentHandler)
	investmentPortfolioRoutes.Patch("/stocks/{stockID}", app.updateStockInvestmentHandler)
	return investmentPortfolioRoutes
}
