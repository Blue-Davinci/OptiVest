package main

import (
	"context"
	"database/sql"
	"errors"
	"expvar"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/Blue-Davinci/OptiVest/internal/data"
	"github.com/Blue-Davinci/OptiVest/internal/database"
	"github.com/Blue-Davinci/OptiVest/internal/logger"
	"github.com/Blue-Davinci/OptiVest/internal/mailer"
	"github.com/Blue-Davinci/OptiVest/internal/vcs"
	"github.com/go-redis/redis/v8"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/microcosm-cc/bluemonday"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
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
		baseurl          string
		activationurl    string
		loginurl         string
		passwordreseturl string
		callback_url     string
		awardurl         string
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
}

type application struct {
	config            config
	logger            *zap.Logger
	models            data.Models
	http_client       *Optivet_Client
	mailer            mailer.Mailer
	wg                sync.WaitGroup
	RedisDB           *redis.Client
	Mutex             sync.Mutex
	WebSocketUpgrader websocket.Upgrader
	Clients           map[int64]chan string
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
	// API keys
	// alpha vantage
	flag.StringVar(&cfg.api.apikeys.alphavantage.key, "api-key-alphavantage", os.Getenv("OPTIVEST_ALPHAVANTAGE_API_KEY"), "Alpha Vantage API key")
	flag.StringVar(&cfg.api.apikeys.alphavantage.url, "api-url-alphavantage", "https://www.alphavantage.co/query?", "Alpha Vantage API URL")
	// exchange rates
	flag.StringVar(&cfg.api.apikeys.exchangerates.key, "api-key-exchangerates", os.Getenv("OPTIVEST_EXCHANGERATE_API_KEY"), "Exchange-Rate API Key")
	flag.StringVar(&cfg.api.apikeys.exchangerates.url, "api-url-exchangerates", "https://v6.exchangerate-api.com/v6", "Exchange-Rate API URL")
	// fred
	flag.StringVar(&cfg.api.apikeys.fred.key, "api-key-fred", os.Getenv("OPTIVEST_FRED_API_KEY"), "FRED API Key")
	flag.StringVar(&cfg.api.apikeys.fred.url, "api-url-fred", "https://api.stlouisfed.org/fred/series/observations?", "FRED API URL")
	// fmp
	flag.StringVar(&cfg.api.apikeys.fmp.key, "api-key-fmp", os.Getenv("OPTIVEST_FINANCIALMODELINGPREP_API_KEY"), "FMP API Key")
	flag.StringVar(&cfg.api.apikeys.fmp.url, "api-url-fmp", "https://financialmodelingprep.com/api/v3", "FMP API URL")
	// sambanova
	flag.StringVar(&cfg.api.apikeys.sambanova.key, "api-key-sambanova", os.Getenv("OPTIVEST_SAMBA_NOVA_LLM_API_KEY"), "Sambanova API Key")
	flag.StringVar(&cfg.api.apikeys.sambanova.url, "api-url-sambanova", "https://fast-api.snova.ai/v1/chat/completions", "Sambanova API URL")
	// optivest microservice
	flag.StringVar(&cfg.api.apikeys.optivestmicroservice.key, "api-key-optivestmicroservice", os.Getenv("OPTIVEST_PREDICTOR_API_KEY"), "OptiVest Microservice API Key")
	flag.StringVar(&cfg.api.apikeys.optivestmicroservice.url, "api-url-optivestmicroservice", "http://127.0.0.1:8000/v1/predict", "OptiVest Microservice API URL")
	// ocrspace
	flag.StringVar(&cfg.api.apikeys.ocrspace.key, "api-key-ocrspace", os.Getenv("OPTIVEST_OCRSPACE_API_KEY"), "OCR.Space API Key")
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
	// Limit configuration
	flag.IntVar(&cfg.limit.monthlyGoalProcessingBatchLimit, "monthly-goal-batch-limit", 100, "Batching Limit for Monthly Goal Processing")
	flag.IntVar(&cfg.limit.recurringExpenseTrackerBurstLimit, "recurring-expense-burst-limit", 100, "Batch Limit for Recurring Expense Tracker")
	flag.IntVar(&cfg.limit.overdueDebtTrackerBurstLimit, "overdue-debt-burst-limit", 100, "Batch Limit for Overdue Debt Tracker")
	flag.IntVar(&cfg.limit.expiredNotificationTrackerBurstLimit, "expired-notification-burst-limit", 100, "Batch Limit for Expired Notification Tracker")
	// Parse the flags
	flag.Parse()
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

	// Create a new version boolean flag with the default value of false.
	displayVersion := flag.Bool("version", false, "Display version and exit")
	// If the version flag value is true, then print out the version number and
	// immediately exit.
	if *displayVersion {
		fmt.Printf("Version:\t%s\n", version)
		os.Exit(0)
	}

	// Initialize Redis connection
	rdb, err := openRedis(cfg)
	if err != nil {
		logger.Fatal("Error while connecting to Redis.", zap.String("error", err.Error()))
	}
	logger.Info("Redis connection established", zap.String("addr", cfg.redis.addr))
	// create our connection pull
	db, err := openDB(cfg)
	if err != nil {
		logger.Fatal(err.Error(), zap.String("dsn", cfg.db.dsn))
	}
	// create out http client
	httpClient := NewClient(cfg.http_client.timeout, cfg.http_client.retrymax)
	// log our connection pool
	logger.Info("database connection pool established", zap.String("dsn", cfg.db.dsn))
	// Init our exp metrics variables for server metrics.
	publishMetrics()

	app := &application{
		config:      cfg,
		logger:      logger,
		models:      data.NewModels(db),
		http_client: httpClient,
		mailer:      mailer.New(cfg.smtp.host, cfg.smtp.port, cfg.smtp.username, cfg.smtp.password, cfg.smtp.sender),
		RedisDB:     rdb,
		Mutex:       sync.Mutex{},
		WebSocketUpgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
		Clients: make(map[int64]chan string),
	}
	err = app.startupFunction()
	if err != nil {
		logger.Fatal("Error while starting up application", zap.String("error", err.Error()))
		return
	}
	// start schedulers
	app.startSchedulers()

	err = app.server()
	if err != nil {
		logger.Fatal("Error while starting server.", zap.String("error", err.Error()))
	}

}

func (app *application) startupFunction() error {
	//fmt.Println("Recieved Bond Data: ", dataaa)
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
			err = app.getAndSaveAvailableCurrencies()
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
	go app.trackMonthlyGoalsScheduleHandler()        // trackMonthlyGoals
	go app.updateGoalProgressOnExpiredGoalsHandler() // updateGoalProgressOnExpiredGoals
	go app.trackExpiredGroupInvitationsHandler()     // trackExpiredGroupInvitations
	go app.trackRecurringExpensesHandler()           // trackRecurringExpenses
	go app.trackOverdueDebtsHandler()                // trackOverdueDebts
	go app.trackExpiredNotificationsHandler()        // trackExpiredNotification
	go app.startRssFeedScraperHandler()              // rssFeedScraper
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

// getCurrentPath invokes getEnvPath to get the path to the .env file based on the current working directory.
// After that it loads the .env file using godotenv.Load to be used by the initFlags() function
func getCurrentPath(logger *zap.Logger) string {
	currentpath := getEnvPath(logger)
	if currentpath != "" {
		err := godotenv.Load(currentpath)
		if err != nil {
			logger.Fatal(err.Error(), zap.String("path", currentpath))
		}
	} else {

		logger.Error("Path Error", zap.String("path", currentpath), zap.String("error", "unable to load .env file"))
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

// openDB() opens a new database connection using the provided configuration.
// It returns a pointer to the sql.DB connection pool and an error value.
func openDB(cfg config) (*database.Queries, error) {
	db, err := sql.Open("postgres", cfg.db.dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(cfg.db.maxOpenConns)
	db.SetMaxIdleConns(cfg.db.maxIdleConns)
	duration, err := time.ParseDuration(cfg.db.maxIdleTime)
	if err != nil {
		return nil, err
	}
	db.SetConnMaxIdleTime(duration)
	// Use ping to establish new conncetions
	err = db.Ping()
	if err != nil {
		return nil, err
	}
	queries := database.New(db)
	return queries, nil
}

// openRedis() opens a new Redis connection using the provided configuration.
// It returns a pointer to the Redis client and an error value.
func openRedis(cfg config) (*redis.Client, error) {
	// Initialize the Redis client with the provided config
	rdb := redis.NewClient(&redis.Options{
		Addr: cfg.redis.addr, // Redis address
		//Password: cfg.redis.password, // No password set if empty
		DB: cfg.redis.db, // Use default DB if not set
	})

	// Ping the Redis server to check if the connection is successful
	err := rdb.Ping(context.Background()).Err()
	if err != nil {
		return nil, err
	}

	return rdb, nil
}
