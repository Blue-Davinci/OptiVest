package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/justinas/alice"
)

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
	v1Router.Mount("/budgets", app.budgetRoutes())

	// Moount the v1Router to the main base router
	router.Mount("/v1", v1Router)
	return router
}

func (app *application) userRoutes(dynamicMiddleware *alice.Chain) chi.Router {
	userRoutes := chi.NewRouter()
	userRoutes.Post("/", app.registerUserHandler)
	// /activation : for activating accounts
	userRoutes.Put("/activated", app.activateUserHandler)
	userRoutes.With(dynamicMiddleware.Then).Patch("/mfa", app.setupMFAHandler)
	userRoutes.With(dynamicMiddleware.Then).Patch("/mfa/verify", app.verifiy2FASetupHandler)
	return userRoutes
}

func (app *application) apiKeyRoutes() chi.Router {
	apiKeyRoutes := chi.NewRouter()
	// initial request for token
	apiKeyRoutes.Post("/authentication", app.createAuthenticationApiKeyHandler)
	apiKeyRoutes.Patch("/authentication/verify", app.validateMFALoginAttemptHandler)
	// /password-reset : for sending keys for resetting passwords
	apiKeyRoutes.Post("/password-reset", app.createPasswordResetTokenHandler)
	return apiKeyRoutes
}

func (app *application) budgetRoutes() chi.Router {
	budgetRoutes := chi.NewRouter()
	budgetRoutes.Post("/", app.createNewBudgetdHandler)
	return budgetRoutes
}
