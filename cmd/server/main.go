// Package main is the entry point for the message dispatcher service.
// It orchestrates all components and provides graceful shutdown handling.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-message-dispatcher/internal/config"
	"github.com/go-message-dispatcher/internal/domain"
	"github.com/go-message-dispatcher/internal/handler"
	"github.com/go-message-dispatcher/internal/repository"
	"github.com/go-message-dispatcher/internal/scheduler"
	"github.com/go-message-dispatcher/internal/service"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	_ "github.com/lib/pq"
)

// Build information set via ldflags during build
var (
	version   = "dev"
	buildTime = "unknown"
	gitCommit = "unknown"
)

// Application holds all the components of our service
type Application struct {
	config               *config.Config
	logger               *zap.Logger
	db                   *sql.DB
	redisClient          *redis.Client
	messageService       domain.MessageService
	processingController domain.ProcessingController
	httpServer           *http.Server
}

func main() {
	// Handle version flag
	showVersion := flag.Bool("version", false, "Show version information")
	flag.Parse()

	if *showVersion {
		fmt.Printf("Message Dispatcher Server\n")
		fmt.Printf("Version: %s\n", version)
		fmt.Printf("Build Time: %s\n", buildTime)
		fmt.Printf("Git Commit: %s\n", gitCommit)
		return
	}

	// Initialize application
	app, err := NewApplication()
	if err != nil {
		fmt.Printf("Failed to initialize application: %v\n", err)
		os.Exit(1)
	}

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the HTTP server
	go func() {
		app.logger.Info("Starting HTTP server", zap.Int("port", app.config.Server.Port))
		if err := app.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			app.logger.Fatal("Failed to start HTTP server", zap.Error(err))
		}
	}()

	// Wait for shutdown signal
	app.waitForShutdown(ctx)
}

// NewApplication creates and configures the entire application
func NewApplication() (*Application, error) {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	// Initialize logger
	logger, err := initLogger(cfg.Server.LogLevel)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	// Initialize database connection
	db, err := initDatabase(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// Initialize Redis connection
	redisClient, err := initRedis(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Redis: %w", err)
	}

	// Initialize repositories
	messageRepo := repository.NewPostgreSQLMessageRepository(db)
	cacheRepo := repository.NewRedisCacheRepository(redisClient)

	// Initialize SMS provider
	smsProvider := service.NewHTTPSMSProvider(cfg.SMS.APIURL, cfg.SMS.Token)

	// Initialize services
	messageService := service.NewMessageService(messageRepo, cacheRepo, smsProvider)

	// Initialize scheduler (processing controller)
	messageScheduler := scheduler.NewMessageScheduler(messageService, logger, cfg.App.ProcessingInterval)

	// Initialize HTTP handlers with version information
	versionInfo := handler.VersionInfo{
		Version:   version,
		BuildTime: buildTime,
		GitCommit: gitCommit,
	}
	messageHandler := handler.NewMessageHandler(messageService, messageScheduler, logger, versionInfo)

	// Setup HTTP server
	httpServer := setupHTTPServer(cfg, messageHandler, logger)

	app := &Application{
		config:               cfg,
		logger:               logger,
		db:                   db,
		redisClient:          redisClient,
		messageService:       messageService,
		processingController: messageScheduler,
		httpServer:           httpServer,
	}

	logger.Info("Application initialized successfully")
	return app, nil
}

// initLogger creates a configured zap logger
func initLogger(logLevel string) (*zap.Logger, error) {
	var level zapcore.Level
	switch logLevel {
	case "debug":
		level = zapcore.DebugLevel
	case "info":
		level = zapcore.InfoLevel
	case "warn":
		level = zapcore.WarnLevel
	case "error":
		level = zapcore.ErrorLevel
	default:
		level = zapcore.InfoLevel
	}

	config := zap.Config{
		Level:       zap.NewAtomicLevelAt(level),
		Development: false,
		Sampling: &zap.SamplingConfig{
			Initial:    100,
			Thereafter: 100,
		},
		Encoding:         "json",
		EncoderConfig:    zap.NewProductionEncoderConfig(),
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	return config.Build()
}

// initDatabase creates and tests the database connection
func initDatabase(cfg *config.Config) (*sql.DB, error) {
	db, err := sql.Open("postgres", cfg.DatabaseDSN())
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	// Configure connection pool for production use
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Test the connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = db.PingContext(ctx)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

// initRedis creates and tests the Redis connection
func initRedis(cfg *config.Config) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr(),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	// Test the connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := client.Ping(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to ping Redis: %w", err)
	}

	return client, nil
}

// setupHTTPServer configures the Gin HTTP server with all routes
func setupHTTPServer(cfg *config.Config, messageHandler *handler.MessageHandler, logger *zap.Logger) *http.Server {
	// Set Gin mode based on log level
	if cfg.Server.LogLevel == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	// Add middleware
	router.Use(gin.Recovery())
	router.Use(ginLogger(logger))

	// Health check and version endpoints
	router.GET("/health", messageHandler.HealthCheck)
	router.GET("/version", messageHandler.Version)

	// API routes
	api := router.Group("/api")
	{
		// Messaging control endpoints
		messaging := api.Group("/messaging")
		{
			messaging.POST("/start", messageHandler.StartProcessing)
			messaging.POST("/stop", messageHandler.StopProcessing)
		}

		// Message monitoring endpoints
		messages := api.Group("/messages")
		{
			messages.GET("/sent", messageHandler.GetSentMessages)
		}
	}

	return &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: router,
	}
}

// ginLogger creates a Gin middleware that logs requests using zap
func ginLogger(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		clientIP := c.ClientIP()
		method := c.Request.Method
		statusCode := c.Writer.Status()

		if raw != "" {
			path = path + "?" + raw
		}

		logger.Info("HTTP Request",
			zap.Int("status", statusCode),
			zap.String("method", method),
			zap.String("path", path),
			zap.String("ip", clientIP),
			zap.Duration("latency", latency),
		)
	}
}

// waitForShutdown handles graceful shutdown on interrupt signals
func (app *Application) waitForShutdown(ctx context.Context) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	<-quit
	app.logger.Info("Shutdown signal received")

	// Create shutdown context with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), app.config.App.ShutdownTimeout)
	defer cancel()

	// Stop message processing first
	if app.processingController.IsRunning() {
		app.logger.Info("Stopping message processing")
		if err := app.processingController.Stop(); err != nil {
			app.logger.Error("Failed to stop message processing", zap.Error(err))
		}
	}

	// Shutdown HTTP server
	app.logger.Info("Shutting down HTTP server")
	if err := app.httpServer.Shutdown(shutdownCtx); err != nil {
		app.logger.Error("Failed to shutdown HTTP server gracefully", zap.Error(err))
	}

	// Close database connections
	if err := app.db.Close(); err != nil {
		app.logger.Error("Failed to close database connection", zap.Error(err))
	}

	// Close Redis connection
	if err := app.redisClient.Close(); err != nil {
		app.logger.Error("Failed to close Redis connection", zap.Error(err))
	}

	app.logger.Info("Application shutdown completed")
}
