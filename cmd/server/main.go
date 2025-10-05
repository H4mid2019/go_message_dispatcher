package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	_ "github.com/go-message-dispatcher/cmd/server/docs"
	"github.com/go-message-dispatcher/internal/config"
	"github.com/go-message-dispatcher/internal/domain"
	"github.com/go-message-dispatcher/internal/handler"
	"github.com/go-message-dispatcher/internal/lock"
	"github.com/go-message-dispatcher/internal/repository"
	"github.com/go-message-dispatcher/internal/scheduler"
	"github.com/go-message-dispatcher/internal/service"
)

var (
	version   = "dev"
	buildTime = "unknown"
	gitCommit = "unknown"
)

// @title Message Dispatcher API
// @version 1.0
// @description API for managing and dispatching SMS messages
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.email support@example.com

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:8080
// @BasePath /api
// @schemes http https

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
	showVersion := flag.Bool("version", false, "Show version information")
	flag.Parse()

	if *showVersion {
		fmt.Printf("Message Dispatcher Server\n")
		fmt.Printf("Version: %s\n", version)
		fmt.Printf("Build Time: %s\n", buildTime)
		fmt.Printf("Git Commit: %s\n", gitCommit)
		return
	}

	app, err := NewApplication()
	if err != nil {
		fmt.Printf("Failed to initialize application: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app.logger.Info("Starting HTTP server", zap.Int("port", app.config.Server.Port))

	go func() {
		if err := app.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			app.logger.Fatal("Failed to start HTTP server", zap.Error(err))
		}
	}()

	app.waitForShutdown(ctx)
}

func NewApplication() (*Application, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	logger, err := initLogger(cfg.Server.LogLevel)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	logger.Info("Initializing application")

	db, err := initDatabase(cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	redisClient, err := initRedis(cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Redis: %w", err)
	}

	messageRepo := repository.NewPostgreSQLMessageRepository(db)
	cacheRepo := repository.NewRedisCacheRepository(redisClient)
	smsProvider := service.NewHTTPSMSProvider(cfg.SMS.APIURL, cfg.SMS.Token)
	messageService := service.NewMessageService(messageRepo, cacheRepo, smsProvider, logger)

	// Create scheduler with distributed locking if enabled
	var messageScheduler domain.ProcessingController
	if cfg.App.DistributedLockEnabled {
		distributedLock := lock.NewRedisLock(redisClient, cfg.App.DistributedLockKey, cfg.App.DistributedLockTTL, logger)
		messageScheduler = scheduler.NewMessageSchedulerWithLock(messageService, logger, cfg.App.ProcessingInterval, distributedLock)
		logger.Info("Distributed locking enabled",
			zap.String("lock_key", cfg.App.DistributedLockKey),
			zap.Duration("lock_ttl", cfg.App.DistributedLockTTL))
	} else {
		messageScheduler = scheduler.NewMessageScheduler(messageService, logger, cfg.App.ProcessingInterval)
		logger.Info("Distributed locking disabled - single instance mode")
	}

	versionInfo := handler.VersionInfo{
		Version:   version,
		BuildTime: buildTime,
		GitCommit: gitCommit,
	}
	messageHandler := handler.NewMessageHandler(messageService, messageScheduler, logger, versionInfo, messageRepo, cacheRepo)
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

	if err := messageScheduler.Start(); err != nil {
		return nil, fmt.Errorf("failed to start message scheduler: %w", err)
	}

	logger.Info("Application initialized",
		zap.String("version", version),
		zap.Int("server_port", cfg.Server.Port))

	return app, nil
}

const debugLevel = "debug"

func initLogger(logLevel string) (*zap.Logger, error) {
	var level zapcore.Level
	switch logLevel {
	case debugLevel:
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

	const samplingInitial = 100
	const samplingTherafter = 100

	zapLogConfig := zap.Config{
		Level:       zap.NewAtomicLevelAt(level),
		Development: false,
		Sampling: &zap.SamplingConfig{
			Initial:    samplingInitial,
			Thereafter: samplingTherafter,
		},
		Encoding:         "json",
		EncoderConfig:    zap.NewProductionEncoderConfig(),
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	return zapLogConfig.Build()
}

func initDatabase(cfg *config.Config, logger *zap.Logger) (*sql.DB, error) {
	const maxOpenConns = 25
	const maxIdleConns = 25
	const connMaxLifetime = 5 * time.Minute
	const dbTimeout = 5 * time.Second
	const maxRetries = 5

	db, err := sql.Open("postgres", cfg.DatabaseDSN())
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(maxIdleConns)
	db.SetConnMaxLifetime(connMaxLifetime)

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
		err = db.PingContext(ctx)
		cancel()

		if err == nil {
			logger.Info("Database connection established",
				zap.String("host", cfg.Database.Host),
				zap.Int("port", cfg.Database.Port),
				zap.Int("attempt", attempt))
			return db, nil
		}

		lastErr = err
		if attempt < maxRetries {
			backoff := time.Duration(attempt*attempt) * time.Second
			logger.Warn("Database connection failed, retrying",
				zap.Error(err),
				zap.Int("attempt", attempt),
				zap.Duration("backoff", backoff))
			time.Sleep(backoff)
		}
	}

	_ = db.Close()
	return nil, fmt.Errorf("failed to connect to database after %d attempts: %w", maxRetries, lastErr)
}

func initRedis(cfg *config.Config, logger *zap.Logger) (*redis.Client, error) {
	const redisTimeout = 5 * time.Second
	const maxRetries = 5

	client := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr(),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
		_, err := client.Ping(ctx).Result()
		cancel()

		if err == nil {
			logger.Info("Redis connection established",
				zap.String("host", cfg.Redis.Host),
				zap.Int("port", cfg.Redis.Port),
				zap.Int("attempt", attempt))
			return client, nil
		}

		lastErr = err
		if attempt < maxRetries {
			backoff := time.Duration(attempt*attempt) * time.Second
			logger.Warn("Redis connection failed, retrying",
				zap.Error(err),
				zap.Int("attempt", attempt),
				zap.Duration("backoff", backoff))
			time.Sleep(backoff)
		}
	}

	return nil, fmt.Errorf("failed to connect to Redis after %d attempts: %w", maxRetries, lastErr)
}

func setupHTTPServer(cfg *config.Config, messageHandler *handler.MessageHandler, logger *zap.Logger) *http.Server {
	if cfg.Server.LogLevel == debugLevel {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(ginLogger(logger))

	router.GET("/health", messageHandler.HealthCheck)
	router.GET("/version", messageHandler.Version)

	api := router.Group("/api")
	messaging := api.Group("/messaging")
	messaging.POST("/start", messageHandler.StartProcessing)
	messaging.POST("/stop", messageHandler.StopProcessing)

	messages := api.Group("/messages")
	messages.GET("/sent", messageHandler.GetSentMessages)

	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	const readHeaderTimeout = 10 * time.Second

	return &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:           router,
		ReadHeaderTimeout: readHeaderTimeout,
	}
}

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

func (app *Application) waitForShutdown(_ context.Context) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	<-quit
	app.logger.Info("Shutdown signal received, initiating graceful shutdown")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), app.config.App.ShutdownTimeout)
	defer cancel()

	if app.processingController.IsRunning() {
		app.logger.Info("Stopping message processing")
		if err := app.processingController.Stop(); err != nil {
			app.logger.Error("Failed to stop message processing", zap.Error(err))
		}
	}

	app.logger.Info("Shutting down HTTP server")
	if err := app.httpServer.Shutdown(shutdownCtx); err != nil {
		app.logger.Error("Failed to shutdown HTTP server gracefully", zap.Error(err))
	}

	if err := app.db.Close(); err != nil {
		app.logger.Error("Failed to close database connection", zap.Error(err))
	}

	if err := app.redisClient.Close(); err != nil {
		app.logger.Error("Failed to close Redis connection", zap.Error(err))
	}

	app.logger.Info("Application shutdown complete")
}
