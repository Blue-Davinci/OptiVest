package main

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"expvar"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/microcosm-cc/bluemonday"
	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"

	"github.com/Blue-Davinci/OptiVest/internal/data"
	"github.com/Blue-Davinci/OptiVest/internal/database"
	"github.com/Blue-Davinci/OptiVest/internal/logger"
	"github.com/Blue-Davinci/OptiVest/internal/mailer"
	"github.com/Blue-Davinci/OptiVest/internal/vcs"
)

// a quick variable to hold our version. ToDo: Change this.
var (
	version = vcs.Version()
)

type apikey_details struct {
	key string
	url string
}

type config struct {
	port int
	env  string
	api  struct {
		name            string
		author          string
		defaultcurrency string
		apikeys         struct { // api keys
			alphavantage         apikey_details
			exchangerates        apikey_details
			fred                 apikey_details
			fmp                  apikey_details
			sambanova            apikey_details
			optivestmicroservice apikey_details
			ocrspace             apikey_details
		}
	}
	ws struct {
		port                     int
		MaxConcurrentConnections int
	}
	db struct {
		dsn          string
		maxOpenConns int
		maxIdleConns int
		maxIdleTime  string
	}
	redis struct {
		addr     string
		password string
		db       int
	}
	http_client struct {
		timeout  time.Duration
		retrymax int
	}
	// llm holds runtime knobs for the streaming SambaNova client. The
	// LLM path is special-cased away from the generic http_client tunables
	// because chat-completions calls keep a connection slot open for
	// 10-30s while the model emits SSE chunks; the generic 10s client
	// timeout would just kill them mid-stream, and the generic 3-retry
	// policy would replay a 30s prompt on a transient 5xx and double
	// user-visible latency.
	llm struct {
		// totalBudget is a wallclock cap that supersedes any per-attempt
		// timeout. It governs the entire call including pre-first-byte
		// retries plus the streaming read. A value of 0 falls back to a
		// safe internal default (see llmStreamingDefaults).
		totalBudget time.Duration
		// idleTimeout aborts the stream when no chunk has arrived within
		// the window. Protects connection slots from a stalled upstream
		// that has sent headers but no body, and also acts as a coarse
		// DoS mitigation.
		idleTimeout time.Duration
		// maxRetriesBeforeFirstByte caps how many times the retryablehttp
		// layer may replay the request on dial / TLS / 5xx errors *before*
		// the first SSE chunk arrives. Mid-stream errors are never
		// retried regardless of this value (see CheckRetry override in
		// LLMStream).
		maxRetriesBeforeFirstByte int
	}
	sanitization struct {
		sanitizer *bluemonday.Policy
		usestrict bool
	}
	scraper struct {
		nooffeedstofetch int
		fetchinterval    int
		scraperclient    struct {
			retrymax int
			timeout  int
		}
	}
	smtp struct {
		host     string
		port     int
		username string
		password string
		sender   string
	}
	limiter struct {
		rps     float64
		burst   int
		enabled bool
	}
	cors struct {
		trustedOrigins []string
	}
	encryption struct {
		key string
	}
	frontend struct {
		baseurl            string
		activationurl      string
		accountsettings    string
		loginurl           string
		passwordreseturl   string
		callback_url       string
		awardurl           string
		groupurl           string
		groupinvitationurl string
		applogourl         string
		profileurl         string
		recoveryurl        string
	}
	scheduler struct {
		trackMonthlyGoalsCron        *cron.Cron
		trackGoalProgressStatus      *cron.Cron
		trackExpiredGroupInvitations *cron.Cron
		trackRecurringExpenses       *cron.Cron
		trackOverdueDebts            *cron.Cron
		trackExpiredNotifications    *cron.Cron
		rssFeedScraper               *cron.Cron
	}
	limit struct {
		monthlyGoalProcessingBatchLimit      int
		recurringExpenseTrackerBurstLimit    int
		overdueDebtTrackerBurstLimit         int
		expiredNotificationTrackerBurstLimit int
	}
	// portfolio holds runtime knobs for the investment-portfolio analysis
	// pipeline. Concurrency is bounded so we never burst more parallel
	// upstream API calls (Alpha Vantage / FRED / FMP) than the configured
	// vendor plan allows.
	portfolio struct {
		// workerLimit caps the number of in-flight per-asset analyses
		// (stocks + bonds) inside performInvestmentPortfolioAnalysis.
		// Each in-flight worker may issue 2-3 upstream HTTP calls plus
		// one DB INSERT. Set to 1 to reproduce the legacy serial behavior.
		workerLimit int
	}
}

type application struct {
	config      config
	logger      *zap.Logger
	models      data.Models
	http_client *Optivet_Client
	mailer      mailer.Mailer
	wg          sync.WaitGroup
	RedisDB     *redis.Client
	// db is the raw connection pool. Held alongside models (sqlc-generated
	// Queries) so operational endpoints — specifically /readyz — can call
	// PingContext on it without going through the typed query layer, which
	// hides *sql.DB behind an unexported DBTX field.
	db *sql.DB
	// ctx is the application's lifecycle context. It is canceled when the
	// process receives SIGINT/SIGTERM (see main()). HTTP servers wire it into
	// BaseContext so in-flight requests get cancellation propagation, and
	// background goroutines should select on app.ctx.Done() to exit cleanly.
	ctx context.Context
	// Mutex protects Clients, ListeningUsers, and ClientCancelFuncs. It is an
	// RWMutex so reads (e.g. snapshotting per-user channels for fan-out) do not
	// serialize with one another.
	Mutex             sync.RWMutex
	WebSocketUpgrader websocket.Upgrader
	Clients           map[int64]chan string
	ListeningUsers    map[int64]bool // Track active listeners for each user
	ClientCancelFuncs map[int64]context.CancelFunc
	// sf is a process-wide singleflight registry that collapses concurrent
	// duplicate upstream API calls into a single in-flight fetch. When the
	// portfolio analysis fans out N goroutines that all miss the Redis cache
	// for the same key (e.g. the FMP global sector snapshot, or two stock
	// lots of the same symbol), only one upstream HTTP request is issued and
	// every other waiter receives the leader's result. This keeps us under
	// vendor rate limits even when worker concurrency rises. Counters
	// `portfolio_singleflight_*` track how often dedup actually fires.
	sf singleflight.Group
}

func main() {
	logger, err := logger.InitJSONLogger()
	if err != nil {
		fmt.Println("Error initializing logger")
		return
	}
	// Load the environment variables from the .env file
	getCurrentPath(logger)
	// config
	var cfg config

	// Load our configurations from the Flags
	// Port & env
	flag.IntVar(&cfg.port, "port", 4000, "API server port")
	flag.StringVar(&cfg.env, "env", "development", "Environment (development|staging|production)")
	// Websocket
	flag.IntVar(&cfg.ws.port, "ws-port", 4001, "Websocket server port")
	flag.IntVar(&cfg.ws.MaxConcurrentConnections, "ws-max-concurrent-connections", 100, "Websocket server max concurrent connections")
	// API configuration
	flag.StringVar(&cfg.api.name, "api-name", "OptiVest", "API name")
	flag.StringVar(&cfg.api.author, "api-author", "Blue_Davinci", "API author")
	flag.StringVar(&cfg.api.defaultcurrency, "api-default-currency", "USD", "Default currency")
	// API keys.
	// All keys default to the corresponding OPTIVEST_* env var; never embed real
	// keys here. URLs default to the upstream public endpoints. Missing keys are
	// validated by validateConfig() below for non-development environments.
	flag.StringVar(&cfg.api.apikeys.alphavantage.key, "api-key-alphavantage", os.Getenv("OPTIVEST_ALPHAVANTAGE_API_KEY"), "Alpha Vantage API key (env OPTIVEST_ALPHAVANTAGE_API_KEY)")
	flag.StringVar(&cfg.api.apikeys.alphavantage.url, "api-url-alphavantage", "https://www.alphavantage.co/query?", "Alpha Vantage API URL")
	flag.StringVar(&cfg.api.apikeys.exchangerates.key, "api-key-exchangerates", os.Getenv("OPTIVEST_EXCHANGERATE_API_KEY"), "Exchange-Rate API key (env OPTIVEST_EXCHANGERATE_API_KEY)")
	flag.StringVar(&cfg.api.apikeys.exchangerates.url, "api-url-exchangerates", "https://v6.exchangerate-api.com/v6", "Exchange-Rate API URL")
	flag.StringVar(&cfg.api.apikeys.fred.key, "api-key-fred", os.Getenv("OPTIVEST_FRED_API_KEY"), "FRED API key (env OPTIVEST_FRED_API_KEY)")
	flag.StringVar(&cfg.api.apikeys.fred.url, "api-url-fred", "https://api.stlouisfed.org/fred/series/observations?", "FRED API URL")
	flag.StringVar(&cfg.api.apikeys.fmp.key, "api-key-fmp", os.Getenv("OPTIVEST_FINANCIALMODELINGPREP_API_KEY"), "FMP API key (env OPTIVEST_FINANCIALMODELINGPREP_API_KEY)")
	flag.StringVar(&cfg.api.apikeys.fmp.url, "api-url-fmp", "https://financialmodelingprep.com/api/v3", "FMP API URL")
	flag.StringVar(&cfg.api.apikeys.sambanova.key, "api-key-sambanova", os.Getenv("OPTIVEST_SAMBA_NOVA_LLM_API_KEY"), "SambaNova API key (env OPTIVEST_SAMBA_NOVA_LLM_API_KEY)")
	flag.StringVar(&cfg.api.apikeys.sambanova.url, "api-url-sambanova", "https://fast-api.snova.ai/v1/chat/completions", "SambaNova API URL")
	flag.StringVar(&cfg.api.apikeys.optivestmicroservice.key, "api-key-optivestmicroservice", os.Getenv("OPTIVEST_PREDICTOR_API_KEY"), "OptiVest predictor microservice API key (env OPTIVEST_PREDICTOR_API_KEY)")
	flag.StringVar(&cfg.api.apikeys.optivestmicroservice.url, "api-url-optivestmicroservice", "http://127.0.0.1:8000/v1/predict", "OptiVest predictor microservice URL")
	flag.StringVar(&cfg.api.apikeys.ocrspace.key, "api-key-ocrspace", os.Getenv("OPTIVEST_OCRSPACE_API_KEY"), "OCR.Space API key (env OPTIVEST_OCRSPACE_API_KEY)")
	flag.StringVar(&cfg.api.apikeys.ocrspace.url, "api-url-ocrspace", "https://api.ocr.space/parse/image", "OCR.Space API URL")
	// Rate limiter flags
	flag.Float64Var(&cfg.limiter.rps, "limiter-rps", 5, "Rate limiter maximum requests per second")
	flag.IntVar(&cfg.limiter.burst, "limiter-burst", 10, "Rate limiter maximum burst")
	flag.BoolVar(&cfg.limiter.enabled, "limiter-enabled", true, "Enable rate limiter")
	// Database configuration
	flag.StringVar(&cfg.db.dsn, "db-dsn", os.Getenv("OPTIVEST_DB_DSN"), "PostgreSQL DSN")
	flag.IntVar(&cfg.db.maxOpenConns, "db-max-open-conns", 25, "PostgreSQL max open connections")
	flag.IntVar(&cfg.db.maxIdleConns, "db-max-idle-conns", 25, "PostgreSQL max idle connections")
	flag.StringVar(&cfg.db.maxIdleTime, "db-max-idle-time", "15m", "PostgreSQL max connection idle time")
	// Redis configuration
	flag.StringVar(&cfg.redis.addr, "redis-addr", "localhost:6379", "Redis address")
	flag.StringVar(&cfg.redis.password, "redis-password", os.Getenv("OPTIVEST_REDIS_PASSWORD"), "Redis password")
	flag.IntVar(&cfg.redis.db, "redis-db", 0, "Redis database")
	// HTTP client configuration
	flag.DurationVar(&cfg.http_client.timeout, "http-client-timeout", 10*time.Second, "HTTP client timeout")
	flag.IntVar(&cfg.http_client.retrymax, "http-client-retrymax", 3, "HTTP client maximum retries")
	// LLM streaming client tunables (SambaNova chat-completions). See the
	// type definition in the config struct for the rationale; defaults are
	// chosen to match the observed p95 latency of the 405B model plus a
	// margin, with retry budget tight enough to keep tail latency below
	// 2x of a healthy single-attempt call.
	flag.DurationVar(&cfg.llm.totalBudget, "llm-total-budget", 90*time.Second, "Wallclock cap for an LLM streaming call across retries")
	flag.DurationVar(&cfg.llm.idleTimeout, "llm-idle-timeout", 15*time.Second, "Abort the LLM stream if no chunk arrives within this window")
	flag.IntVar(&cfg.llm.maxRetriesBeforeFirstByte, "llm-max-retries", 2, "Max retries for the LLM call before the first SSE chunk; mid-stream errors never retry")
	// Sanitization
	flag.BoolVar(&cfg.sanitization.usestrict, "sanitization-strict", false, "Use strict sanitization")
	// Encryption key
	flag.StringVar(&cfg.encryption.key, "encryption-key", os.Getenv("OPTIVEST_DATA_ENCRYPTION_KEY"), "Encryption key")
	// CORS configuration
	flag.Func("cors-trusted-origins", "Trusted CORS origins (space separated)", func(val string) error {
		cfg.cors.trustedOrigins = strings.Fields(val)
		return nil

	})
	// Scraper settings
	flag.IntVar(&cfg.scraper.nooffeedstofetch, "scraper-routines", 5, "Number of feeds to fetch concurrently")
	flag.IntVar(&cfg.scraper.fetchinterval, "scraper-interval", 40, "Interval in seconds before the next bunch of feeds are fetched")
	flag.IntVar(&cfg.scraper.scraperclient.retrymax, "scraper-retry-max", 3, "Maximum number of retries for HTTP requests")
	flag.IntVar(&cfg.scraper.scraperclient.timeout, "scraper-timeout", 15, "HTTP client timeout in seconds")
	// SMTP configuration
	flag.StringVar(&cfg.smtp.host, "smtp-host", os.Getenv("OPTIVEST_SMTP_HOST"), "SMTP server hostname")
	flag.IntVar(&cfg.smtp.port, "smtp-port", 587, "SMTP server port")
	flag.StringVar(&cfg.smtp.username, "smtp-username", os.Getenv("OPTIVEST_SMTP_USERNAME"), "SMTP server username")
	flag.StringVar(&cfg.smtp.password, "smtp-password", os.Getenv("OPTIVEST_SMTP_PASSWORD"), "SMTP server password")
	flag.StringVar(&cfg.smtp.sender, "smtp-sender", os.Getenv("OPTIVEST_SMTP_SENDER"), "SMTP sender email address")
	// Frontend configuration
	flag.StringVar(&cfg.frontend.baseurl, "frontend-url", "http://localhost:5173", "Frontend URL")
	flag.StringVar(&cfg.frontend.loginurl, "frontend-login-url", "http://localhost:5173/login", "Frontend Login URL")
	flag.StringVar(&cfg.frontend.activationurl, "frontend-activation-url", "http://localhost:5173/verify?token=", "Frontend Activation URL")
	flag.StringVar(&cfg.frontend.passwordreseturl, "frontend-password-reset-url", "http://localhost:5173/passwordreset/password?token=", "Frontend Password Reset URL")
	flag.StringVar(&cfg.frontend.callback_url, "frontend-callback-url", "https://adapted-healthy-monitor.ngrok-free.app/v1", "Frontend Callback URL")
	flag.StringVar(&cfg.frontend.awardurl, "frontend-award-url", "http://localhost:5173/awards", "Frontend Award URL")
	flag.StringVar(&cfg.frontend.groupurl, "frontend-group-url", "http://localhost:5173/dashboard/groups", "Frontend Group URL")
	flag.StringVar(&cfg.frontend.groupinvitationurl, "frontend-group-invite-url", "http://localhost:5173/dashboard/groups/invitation", "Frontend Group invitation URL")
	flag.StringVar(&cfg.frontend.applogourl, "frontend-app-logo-url", "https://i.ibb.co/hZdMWvh/optivest-cropped.png", "Frontend App Logo URL")
	flag.StringVar(&cfg.frontend.accountsettings, "frontend-account-settings", "http://localhost:5173/dashboard/account", "Frontend Account Settings URL")
	flag.StringVar(&cfg.frontend.profileurl, "frontend-profile-url", "http://localhost:5173/dashboard/account", "Frontend Profile URL")
	flag.StringVar(&cfg.frontend.recoveryurl, "frontend-recovery-url", "http://localhost:5173/passwordreset/recovery/validate", "Frontend Recovery URL")
	// Limit configuration
	flag.IntVar(&cfg.limit.monthlyGoalProcessingBatchLimit, "monthly-goal-batch-limit", 100, "Batching Limit for Monthly Goal Processing")
	flag.IntVar(&cfg.limit.recurringExpenseTrackerBurstLimit, "recurring-expense-burst-limit", 100, "Batch Limit for Recurring Expense Tracker")
	flag.IntVar(&cfg.limit.overdueDebtTrackerBurstLimit, "overdue-debt-burst-limit", 100, "Batch Limit for Overdue Debt Tracker")
	flag.IntVar(&cfg.limit.expiredNotificationTrackerBurstLimit, "expired-notification-burst-limit", 100, "Batch Limit for Expired Notification Tracker")
	// Investment portfolio analysis concurrency.
	// Default 6 chosen to balance per-user latency (sub-5s for typical
	// 10-15 asset portfolios) against upstream Alpha Vantage rate limits
	// (Premium tier = 75 req/min; each worker issues ~2 AV calls). On
	// Free tier (5 req/min) consider lowering to 1; on Premium Plus
	// (600 req/min) it can safely go to 16+. Set to 1 to disable
	// concurrency entirely and reproduce pre-P3 serial behavior.
	flag.IntVar(&cfg.portfolio.workerLimit, "portfolio-worker-limit", 6, "Max concurrent per-asset workers in portfolio analysis (1 = serial; tune to upstream API rate limits)")

	// Refuse to start if a secret-bearing flag was passed on the command
	// line. The flag definitions above remain so the StringVars still
	// default-from-env, but operators who type the value into the shell
	// (instead of exporting the env var) leak it to /proc/<pid>/cmdline,
	// `ps -ef`, shell history, and any process supervisor that records
	// argv. We do this BEFORE flag.Parse so the value is never decoded
	// into a typed config field that might subsequently appear in a log
	// line or in /debug/vars. Runs before the -version declaration too
	// because rejectSecretFlags only looks at os.Args literally; it
	// doesn't depend on any flag being registered yet.
	if raw, name := rejectSecretFlags(os.Args[1:]); raw != "" {
		fmt.Fprintf(os.Stderr,
			"refusing to start: %q is a secret-bearing flag (matched %q); set the corresponding env var instead\n",
			raw, name,
		)
		os.Exit(2)
	}

	// -version must be declared BEFORE flag.Parse(); declaring it after
	// (the prior layout) meant `optivest -version` failed at parse time
	// with "flag provided but not defined" and the conditional below
	// always saw the zero value. Image-publishing scripts that grep
	// stdout for the build version saw nothing.
	displayVersion := flag.Bool("version", false, "Display version and exit")

	flag.Parse()

	if *displayVersion {
		fmt.Printf("Version:\t%s\n", version)
		os.Exit(0)
	}

	// Fail fast if required secrets are missing in non-development environments.
	// In development we tolerate empty secrets so the server can boot for partial
	// integration work, but warn loudly so misconfiguration is visible.
	if errs := validateConfig(cfg); len(errs) > 0 {
		if cfg.env == "development" {
			for _, e := range errs {
				logger.Warn("missing configuration (allowed in development)", zap.String("error", e))
			}
		} else {
			for _, e := range errs {
				logger.Error("missing required configuration", zap.String("error", e))
			}
			logger.Fatal("refusing to start: required configuration missing", zap.String("env", cfg.env))
		}
	}

	// Initialize our cronJobs
	cfg.scheduler.trackMonthlyGoalsCron = cron.New()
	cfg.scheduler.trackGoalProgressStatus = cron.New()
	cfg.scheduler.trackExpiredGroupInvitations = cron.New()
	cfg.scheduler.trackRecurringExpenses = cron.New()
	cfg.scheduler.trackOverdueDebts = cron.New()
	cfg.scheduler.trackExpiredNotifications = cron.New()
	cfg.scheduler.rssFeedScraper = cron.New()
	// if the usestrict flag is set to true, then use the StrictPolicy() method to create a new Policy object.
	// Otherwise, use the UGCPolicy() method to create a new Policy object.
	if cfg.sanitization.usestrict {
		cfg.sanitization.sanitizer = bluemonday.StrictPolicy()
	} else {
		cfg.sanitization.sanitizer = bluemonday.UGCPolicy()
	}

	// Initialize Redis connection
	rdb, err := openRedis(cfg)
	if err != nil {
		logger.Fatal("Error while connecting to REDIS.", zap.String("error", err.Error()))
	}
	logger.Info("Redis connection established", zap.String("addr", cfg.redis.addr))
	// create our connection pull. openDB returns both the raw *sql.DB pool
	// (used by /readyz for liveness pings) and the sqlc-generated Queries
	// wrapper (used by the data layer).
	rawDB, queries, err := openDB(cfg)
	if err != nil {
		logger.Fatal(err.Error(), zap.String("dsn", cfg.db.dsn))
	}
	// create out http client. The shared logger flows in so every outbound
	// call participates in the inbound request-correlation pipeline (see
	// cmd/api/http_clients.go for the schema).
	httpClient := NewClient(cfg.http_client.timeout, cfg.http_client.retrymax, logger)
	// log our connection pool
	logger.Info("database connection pool established", zap.String("dsn", cfg.db.dsn))
	// Init our exp metrics variables for server metrics.
	publishMetrics()

	// Application lifecycle context. signal.NotifyContext cancels the context
	// when the process receives SIGINT or SIGTERM, replacing the per-server
	// signal.Notify dance the previous version did. Both HTTP servers and
	// (in P1+) background goroutines watch this context to exit cleanly.
	appCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	app := &application{
		config:      cfg,
		logger:      logger,
		models:      data.NewModels(queries),
		http_client: httpClient,
		mailer:      mailer.New(cfg.smtp.host, cfg.smtp.port, cfg.smtp.username, cfg.smtp.password, cfg.smtp.sender),
		RedisDB:     rdb,
		db:          rawDB,
		ctx:         appCtx,
		WebSocketUpgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
		Clients:           make(map[int64]chan string),
		ListeningUsers:    make(map[int64]bool),
		ClientCancelFuncs: make(map[int64]context.CancelFunc),
	}
	if err := app.startupFunction(); err != nil {
		// startupFunction's main job is seeding the default-currency cache
		// from the Exchange Rate API. In a fresh environment the upstream
		// call can legitimately fail (no API key in development, vendor
		// outage, network egress restricted, etc.). validateConfig already
		// treats missing required secrets as warn-in-dev / fatal-in-prod;
		// the startup hook should follow the same policy so a developer
		// without every vendor key can still bring the binary up.
		// Currency-aware code paths will fail at first use with a clear
		// per-request error, which is the right tradeoff vs. crash-looping
		// at boot.
		if cfg.env == "development" {
			logger.Warn("startup: currency seeding failed (allowed in development)", zap.String("error", err.Error()))
		} else {
			logger.Fatal("Error while starting up application", zap.String("error", err.Error()))
			return
		}
	}
	// start schedulers
	app.startSchedulers()
	logger.Info("Loaded Cors Origins", zap.Strings("origins", cfg.cors.trustedOrigins))
	err = app.server()
	if err != nil {
		logger.Fatal("Error while starting server.", zap.String("error", err.Error()))
	}

}

func (app *application) startupFunction() error {
	//fmt.Println("Received Bond Data: ", dataaa)
	// first we need to check if the currency is in REDIS, if it is
	// we skip requesting the data from the API
	// if it is not we request the data from the API and save it to REDIS
	// If the currency cannot be found it will return ErrFailedToGetCurrency
	err := app.verifyCurrencyInRedis(app.config.api.defaultcurrency)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrFailedToGetCurrency):
			// log the error and continue to fetch the data from the API
			app.logger.Error("Failed to get currency from Redis", zap.String("currency", app.config.api.defaultcurrency))
			// read and load currencies
			err = app.getAndSaveAvailableCurrencies(app.ctx)
			if err != nil {
				return err
			}
		default:
			app.logger.Error("Error verifying currency in Redis", zap.String("error", err.Error()))
			return err
		}
	}
	return nil
}

// startSchedulers starts the cronjobs for the application
func (app *application) startSchedulers() {
	app.logger.Info("Starting Schedulers")
	// we use our app.background function to run the cronjobs in the background
	// and also will be responsible for managing them especially when we need to stop or end the application
	app.background(func() {
		app.trackMonthlyGoalsScheduleHandler()        // trackMonthlyGoals
		app.updateGoalProgressOnExpiredGoalsHandler() // updateGoalProgressOnExpiredGoals
		app.trackExpiredGroupInvitationsHandler()     // trackExpiredGroupInvitations
		app.trackRecurringExpensesHandler()           // trackRecurringExpenses
		app.trackOverdueDebtsHandler()                // trackOverdueDebts
		app.trackExpiredNotificationsHandler()        // trackExpiredNotification
		app.startRssFeedScraperHandler()              // rssFeedScraper
		app.listenToAwardNotifications()              // listenToAwardNotifications
	})

}

// publishMetrics sets up the expvar variables for the application
// It sets the version, the number of active goroutines, and the current Unix timestamp.
func publishMetrics() {
	expvar.NewString("version").Set(version)
	// Publish the number of active goroutines.
	expvar.Publish("goroutines", expvar.Func(func() any {
		return runtime.NumGoroutine()
	}))
	// Publish the current Unix timestamp.
	expvar.Publish("timestamp", expvar.Func(func() any {
		return time.Now().Unix()
	}))
}

// getCurrentPath invokes getEnvPath to get the path to the .env file based on
// the current working directory and asks godotenv to load it into the
// process environment.
//
// Failure to load the file is intentionally a WARN, not a FATAL: when the
// binary runs inside a container, k8s pod, or CI job, configuration is
// usually injected via real environment variables and a .env file is
// neither expected nor present. The downstream validateConfig() call is the
// single source of truth for "do we have what we need to start"; it will
// fatal cleanly with a list of every missing variable in non-development
// environments and warn-only in development. Forcing a fatal here on top of
// that just made the binary unrunnable in containers without a sentinel
// file mounted at the right path.
func getCurrentPath(logger *zap.Logger) string {
	currentpath := getEnvPath(logger)
	if currentpath == "" {
		logger.Warn("env loader: could not determine .env path; proceeding with process environment only")
		return currentpath
	}
	if err := godotenv.Load(currentpath); err != nil {
		logger.Warn("env loader: could not load .env; proceeding with process environment only",
			zap.String("path", currentpath),
			zap.String("error", err.Error()),
		)
		return currentpath
	}
	logger.Info("Loading Environment Variables", zap.String("path", currentpath))
	return currentpath
}

// getEnvPath returns the path to the .env file based on the current working directory.
func getEnvPath(logger *zap.Logger) string {
	dir, err := os.Getwd()
	if err != nil {
		logger.Fatal(err.Error(), zap.String("path", dir))
		return ""
	}
	if strings.Contains(dir, "cmd/api") || strings.Contains(dir, "cmd") {
		return ".env"
	}
	return filepath.Join("cmd", "api", ".env")
}

// openDB opens the Postgres connection pool and wraps it with the sqlc
// Queries layer. Both handles are returned: the raw *sql.DB so operational
// endpoints (e.g. /readyz) can ping it directly, and *database.Queries so
// the data layer can issue typed queries.
func openDB(cfg config) (*sql.DB, *database.Queries, error) {
	db, err := sql.Open("postgres", cfg.db.dsn)
	if err != nil {
		return nil, nil, err
	}
	db.SetMaxOpenConns(cfg.db.maxOpenConns)
	db.SetMaxIdleConns(cfg.db.maxIdleConns)
	duration, err := time.ParseDuration(cfg.db.maxIdleTime)
	if err != nil {
		return nil, nil, err
	}
	db.SetConnMaxIdleTime(duration)
	// Use ping to establish new conncetions
	err = db.Ping()
	if err != nil {
		return nil, nil, err
	}
	queries := database.New(db)
	return db, queries, nil
}

// secretFlagPrefixes is the set of CLI flag name prefixes whose values are
// sensitive credentials. The flag definitions in main() default-from-env for
// each of these, so an operator who exports the env var still gets the
// expected behavior. What's banned here is passing the value LITERALLY on
// the command line, which leaks it to:
//
//   - /proc/<pid>/cmdline (world-readable on most Linux distros)
//   - ps -ef and any tool that reads /proc/<pid>/cmdline
//   - shell history (.bash_history / .zsh_history)
//   - any process supervisor that records argv (systemd journal, k8s
//     describe, docker inspect, supervisord logs)
//
// Prefix matching is intentional for "-api-key-" so we cover every API key
// without having to list each one - new vendor integrations get the
// protection automatically.
var secretFlagPrefixes = []string{
	"encryption-key",
	"redis-password",
	"smtp-password",
	"db-dsn",
	"api-key-",
}

// rejectSecretFlags scans args (typically os.Args[1:]) for any token
// matching a secretFlagPrefixes entry. Returns the raw arg as the user
// typed it (so error messages echo back exactly what was rejected) and
// the normalised flag name; both empty if args are clean.
//
// Both single-dash (-encryption-key=...) and double-dash forms
// (--encryption-key=...) are handled by trimming leading dashes before
// the prefix match. Values are only extracted to find the '=' boundary;
// they are never returned and never logged.
func rejectSecretFlags(args []string) (raw, name string) {
	for _, arg := range args {
		trimmed := strings.TrimLeft(arg, "-")
		if eq := strings.IndexByte(trimmed, '='); eq != -1 {
			trimmed = trimmed[:eq]
		}
		for _, prefix := range secretFlagPrefixes {
			if strings.HasPrefix(trimmed, prefix) {
				return arg, trimmed
			}
		}
	}
	return "", ""
}

// validateConfig returns a list of human-readable errors describing required
// configuration that is missing. It is intentionally cheap to call: it does not
// perform any I/O. Callers decide whether missing values are fatal based on the
// runtime environment (see main()).
//
// The list of required secrets reflects the surface that the API actively uses
// in production paths. Add to it whenever a new upstream integration becomes a
// hard dependency.
func validateConfig(cfg config) []string {
	required := []struct {
		name  string
		value string
	}{
		{"OPTIVEST_DB_DSN (-db-dsn)", cfg.db.dsn},
		{"OPTIVEST_DATA_ENCRYPTION_KEY (-encryption-key)", cfg.encryption.key},
		{"OPTIVEST_ALPHAVANTAGE_API_KEY (-api-key-alphavantage)", cfg.api.apikeys.alphavantage.key},
		{"OPTIVEST_EXCHANGERATE_API_KEY (-api-key-exchangerates)", cfg.api.apikeys.exchangerates.key},
		{"OPTIVEST_FRED_API_KEY (-api-key-fred)", cfg.api.apikeys.fred.key},
		{"OPTIVEST_FINANCIALMODELINGPREP_API_KEY (-api-key-fmp)", cfg.api.apikeys.fmp.key},
		{"OPTIVEST_SAMBA_NOVA_LLM_API_KEY (-api-key-sambanova)", cfg.api.apikeys.sambanova.key},
		{"OPTIVEST_PREDICTOR_API_KEY (-api-key-optivestmicroservice)", cfg.api.apikeys.optivestmicroservice.key},
		{"OPTIVEST_OCRSPACE_API_KEY (-api-key-ocrspace)", cfg.api.apikeys.ocrspace.key},
		{"OPTIVEST_SMTP_HOST (-smtp-host)", cfg.smtp.host},
		{"OPTIVEST_SMTP_USERNAME (-smtp-username)", cfg.smtp.username},
		{"OPTIVEST_SMTP_PASSWORD (-smtp-password)", cfg.smtp.password},
		{"OPTIVEST_SMTP_SENDER (-smtp-sender)", cfg.smtp.sender},
	}
	var errs []string
	for _, r := range required {
		if strings.TrimSpace(r.value) == "" {
			errs = append(errs, r.name+" is empty")
		}
	}
	// AES-GCM key must be hex-decoded to 16/24/32 bytes; reject bad lengths early
	// so the first request that needs to encrypt/decrypt does not get a runtime
	// panic in internal/data.
	if cfg.encryption.key != "" {
		decoded, err := hex.DecodeString(cfg.encryption.key)
		switch {
		case err != nil:
			errs = append(errs, "OPTIVEST_DATA_ENCRYPTION_KEY must be hex-encoded: "+err.Error())
		case len(decoded) != 16 && len(decoded) != 24 && len(decoded) != 32:
			errs = append(errs, fmt.Sprintf("OPTIVEST_DATA_ENCRYPTION_KEY must decode to 16/24/32 bytes, got %d", len(decoded)))
		}
	}
	// portfolio worker limit guardrails. Reject 0/negative outright (would
	// deadlock errgroup.SetLimit with 0). 64 is a soft ceiling: above that
	// you are almost certainly going to get rate-limited by upstream APIs
	// faster than you gain throughput, so force operators to make the
	// trade-off consciously.
	if cfg.portfolio.workerLimit < 1 {
		errs = append(errs, fmt.Sprintf("-portfolio-worker-limit must be >= 1, got %d", cfg.portfolio.workerLimit))
	}
	if cfg.portfolio.workerLimit > 64 {
		errs = append(errs, fmt.Sprintf("-portfolio-worker-limit must be <= 64, got %d (set explicitly via -portfolio-worker-limit if you really mean this)", cfg.portfolio.workerLimit))
	}
	return errs
}

// openRedis() opens a new Redis connection using the provided configuration.
// It returns a pointer to the Redis client and an error value.
func openRedis(cfg config) (*redis.Client, error) {
	// Initialize the Redis client with the provided config. The Password
	// field MUST be wired through here: cfg.redis.password is read from
	// -redis-password / OPTIVEST_REDIS_PASSWORD, validated, and surfaced
	// in the startup banner as "configured", but if the field below is
	// commented out the driver sends every command unauthenticated. A
	// Redis instance with `requirepass` rejects every call with NOAUTH,
	// and the failure mode looks like a generic transient error in the
	// caller stack rather than a configuration problem. Empty string
	// means "no auth", which is the existing zero-value semantics of
	// redis.Options - so leaving this enabled is safe even when the
	// operator hasn't set OPTIVEST_REDIS_PASSWORD.
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.redis.addr,
		Password: cfg.redis.password,
		DB:       cfg.redis.db,
	})

	// Ping the Redis server with a short bounded deadline so a misconfigured
	// Redis address fails fast at startup instead of hanging the process on
	// the default dial timeout.
	pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(pingCtx).Err(); err != nil {
		return nil, err
	}

	return rdb, nil
}
