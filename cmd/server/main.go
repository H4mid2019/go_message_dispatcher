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

var (
	version   = "dev"
	buildTime = "unknown"
	gitCommit = "unknown"
)

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

	go func() {
		if err := app.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
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

	db, err := initDatabase(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	redisClient, err := initRedis(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Redis: %w", err)
	}

	messageRepo := repository.NewPostgreSQLMessageRepository(db)
	cacheRepo := repository.NewRedisCacheRepository(redisClient)
	smsProvider := service.NewHTTPSMSProvider(cfg.SMS.APIURL, cfg.SMS.Token)
	messageService := service.NewMessageService(messageRepo, cacheRepo, smsProvider)
	messageScheduler := scheduler.NewMessageScheduler(messageService, logger, cfg.App.ProcessingInterval)

	versionInfo := handler.VersionInfo{
		Version:   version,
		BuildTime: buildTime,
		GitCommit: gitCommit,
	}
	messageHandler := handler.NewMessageHandler(messageService, messageScheduler, logger, versionInfo)
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

	return app, nil
}
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

func initDatabase(cfg *config.Config) (*sql.DB, error) {
	db, err := sql.Open("postgres", cfg.DatabaseDSN())
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = db.PingContext(ctx)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

func initRedis(cfg *config.Config) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr(),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := client.Ping(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to ping Redis: %w", err)
	}

	return client, nil
}

func setupHTTPServer(cfg *config.Config, messageHandler *handler.MessageHandler, logger *zap.Logger) *http.Server {
	if cfg.Server.LogLevel == "debug" {
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
	{
		messaging := api.Group("/messaging")
		{
			messaging.POST("/start", messageHandler.StartProcessing)
			messaging.POST("/stop", messageHandler.StopProcessing)
		}

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

func (app *Application) waitForShutdown(ctx context.Context) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	<-quit

	shutdownCtx, cancel := context.WithTimeout(context.Background(), app.config.App.ShutdownTimeout)
	defer cancel()

	if app.processingController.IsRunning() {
		if err := app.processingController.Stop(); err != nil {
			app.logger.Error("Failed to stop message processing", zap.Error(err))
		}
	}

	if err := app.httpServer.Shutdown(shutdownCtx); err != nil {
		app.logger.Error("Failed to shutdown HTTP server gracefully", zap.Error(err))
	}

	if err := app.db.Close(); err != nil {
		app.logger.Error("Failed to close database connection", zap.Error(err))
	}

	if err := app.redisClient.Close(); err != nil {
		app.logger.Error("Failed to close Redis connection", zap.Error(err))
	}
}
