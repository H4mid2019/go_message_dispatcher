package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Database DatabaseConfig
	Redis    RedisConfig
	Server   ServerConfig
	SMS      SMSConfig
	App      AppConfig
}

type DatabaseConfig struct {
	Host     string
	Port     int
	Name     string
	User     string
	Password string
	SSLMode  string
}

type RedisConfig struct {
	Host     string
	Port     int
	Password string
	DB       int
}

type ServerConfig struct {
	Port     int
	LogLevel string
}

type SMSConfig struct {
	APIURL string
	Token  string
}

type AppConfig struct {
	BatchSize              int
	ProcessingInterval     time.Duration
	ShutdownTimeout        time.Duration
	DistributedLockEnabled bool
	DistributedLockTTL     time.Duration
	DistributedLockKey     string
	MaxContentLength       int
}

func Load() (*Config, error) {
	const (
		defaultDBPort     = 5432
		defaultRedisPort  = 6379
		defaultServerPort = 8080
		defaultBatchSize  = 2
	)

	config := &Config{
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnvInt("DB_PORT", defaultDBPort),
			Name:     getEnv("DB_NAME", "messages_db"),
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", "password"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		Redis: RedisConfig{
			Host:     getEnv("REDIS_HOST", "localhost"),
			Port:     getEnvInt("REDIS_PORT", defaultRedisPort),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvInt("REDIS_DB", 0),
		},
		Server: ServerConfig{
			Port:     getEnvInt("SERVER_PORT", defaultServerPort),
			LogLevel: getEnv("LOG_LEVEL", "info"),
		},
		SMS: SMSConfig{
			APIURL: getEnv("SMS_API_URL", "http://localhost:3001/send"),
			Token:  getEnv("SMS_API_TOKEN", "mock-token"),
		},
		App: AppConfig{
			BatchSize:              getEnvInt("BATCH_SIZE", defaultBatchSize),
			ProcessingInterval:     getEnvDuration("PROCESSING_INTERVAL", 2*time.Minute), //nolint:mnd
			ShutdownTimeout:        getEnvDuration("SHUTDOWN_TIMEOUT", 30*time.Second),   //nolint:mnd
			DistributedLockEnabled: getEnvBool("DISTRIBUTED_LOCK_ENABLED", false),
			DistributedLockTTL:     getEnvDuration("DISTRIBUTED_LOCK_TTL", 3*time.Minute),     //nolint:mnd
			DistributedLockKey:     getEnv("DISTRIBUTED_LOCK_KEY", "message-dispatcher:lock"), //nolint:mnd
			MaxContentLength:       getEnvInt("MAX_CONTENT_LENGTH", 160),                      //nolint:mnd
		},
	}

	if err := config.validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return config, nil
}

func (c *Config) validate() error {
	if c.Database.Name == "" {
		return fmt.Errorf("database name is required")
	}
	if c.Database.User == "" {
		return fmt.Errorf("database user is required")
	}
	if c.SMS.APIURL == "" {
		return fmt.Errorf("SMS API URL is required")
	}
	if c.App.BatchSize <= 0 {
		return fmt.Errorf("batch size must be positive")
	}
	if c.App.ProcessingInterval <= 0 {
		return fmt.Errorf("processing interval must be positive")
	}
	if c.App.MaxContentLength <= 0 {
		return fmt.Errorf("max content length must be positive")
	}
	return nil
}

func (c *Config) DatabaseDSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Database.Host, c.Database.Port, c.Database.User,
		c.Database.Password, c.Database.Name, c.Database.SSLMode)
}

func (c *Config) RedisAddr() string {
	return fmt.Sprintf("%s:%d", c.Redis.Host, c.Redis.Port)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolVal, err := strconv.ParseBool(value); err == nil {
			return boolVal
		}
	}
	return defaultValue
}
