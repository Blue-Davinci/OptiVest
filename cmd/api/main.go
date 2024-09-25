package main

import (
	"database/sql"
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
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"go.uber.org/zap"
)

// a quick variable to hold our version. ToDo: Change this.
var (
	version = vcs.Version()
)

type config struct {
	port int
	env  string
	db   struct {
		dsn          string
		maxOpenConns int
		maxIdleConns int
		maxIdleTime  string
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
	}
}

type application struct {
	config config
	logger *zap.Logger
	models data.Models
	mailer mailer.Mailer
	wg     sync.WaitGroup
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
	// Rate limiter flags
	flag.Float64Var(&cfg.limiter.rps, "limiter-rps", 5, "Rate limiter maximum requests per second")
	flag.IntVar(&cfg.limiter.burst, "limiter-burst", 10, "Rate limiter maximum burst")
	flag.BoolVar(&cfg.limiter.enabled, "limiter-enabled", true, "Enable rate limiter")
	// Database configuration
	flag.StringVar(&cfg.db.dsn, "db-dsn", os.Getenv("OPTIVEST_DB_DSN"), "PostgreSQL DSN")
	flag.IntVar(&cfg.db.maxOpenConns, "db-max-open-conns", 25, "PostgreSQL max open connections")
	flag.IntVar(&cfg.db.maxIdleConns, "db-max-idle-conns", 25, "PostgreSQL max idle connections")
	flag.StringVar(&cfg.db.maxIdleTime, "db-max-idle-time", "15m", "PostgreSQL max connection idle time")
	// Encryption key
	flag.StringVar(&cfg.encryption.key, "encryption-key", os.Getenv("OPTIVEST_DATA_ENCRYPTION_KEY"), "Encryption key")
	// CORS configuration
	flag.Func("cors-trusted-origins", "Trusted CORS origins (space separated)", func(val string) error {
		defaultCorsTrustedOrigins := "http://localhost:5173"
		if val == "" {
			val = defaultCorsTrustedOrigins
		}
		cfg.cors.trustedOrigins = strings.Fields(val)
		return nil
	})
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
	flag.StringVar(&cfg.frontend.passwordreseturl, "frontend-password-reset-url", "http://localhost:5173/reset/password?token=", "Frontend Password Reset URL")
	flag.StringVar(&cfg.frontend.callback_url, "frontend-callback-url", "https://adapted-healthy-monitor.ngrok-free.app/v1", "Frontend Callback URL")
	// Parse the flags
	flag.Parse()
	// Create a new version boolean flag with the default value of false.
	displayVersion := flag.Bool("version", false, "Display version and exit")
	// If the version flag value is true, then print out the version number and
	// immediately exit.
	if *displayVersion {
		fmt.Printf("Version:\t%s\n", version)
		os.Exit(0)
	}
	// create our connection pull
	db, err := openDB(cfg)
	if err != nil {
		logger.Fatal(err.Error(), zap.String("dsn", cfg.db.dsn))
	}
	logger.Info("database connection pool established", zap.String("dsn", cfg.db.dsn))
	logger.Info("Encryption Key", zap.String("key", cfg.encryption.key))
	// Init our exp metrics variables for server metrics.
	publishMetrics()

	app := &application{
		config: cfg,
		logger: logger,
		models: data.NewModels(db),
		mailer: mailer.New(cfg.smtp.host, cfg.smtp.port, cfg.smtp.username, cfg.smtp.password, cfg.smtp.sender),
	}
	err = app.server()
	if err != nil {
		logger.Fatal("Error while starting server.", zap.String("error", err.Error()))
	}
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
