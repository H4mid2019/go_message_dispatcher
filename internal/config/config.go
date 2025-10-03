// Package config provides centralized configuration management for the message dispatcher service.
// It loads configuration from environment variables with sensible defaults and validation.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all configuration values for the application
type Config struct {
	Database DatabaseConfig
	Redis    RedisConfig
	Server   ServerConfig
	SMS      SMSConfig
	App      AppConfig
}

// DatabaseConfig contains PostgreSQL connection parameters
type DatabaseConfig struct {
	Host     string
	Port     int
	Name     string
	User     string
	Password string
	SSLMode  string
}

// RedisConfig contains Redis connection parameters
type RedisConfig struct {
	Host     string
	Port     int
	Password string
	DB       int
}

// ServerConfig contains HTTP server configuration
type ServerConfig struct {
	Port     int
	LogLevel string
}

// SMSConfig contains SMS provider API configuration
type SMSConfig struct {
	APIURL string
	Token  string
}

// AppConfig contains application-specific settings
type AppConfig struct {
	BatchSize          int
	ProcessingInterval time.Duration
	ShutdownTimeout    time.Duration
}

// Load reads configuration from environment variables and returns a Config struct.
// It provides sensible defaults for development and validates required values.
func Load() (*Config, error) {
	config := &Config{
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnvInt("DB_PORT", 5432),
			Name:     getEnv("DB_NAME", "messages_db"),
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", "password"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		Redis: RedisConfig{
			Host:     getEnv("REDIS_HOST", "localhost"),
			Port:     getEnvInt("REDIS_PORT", 6379),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvInt("REDIS_DB", 0),
		},
		Server: ServerConfig{
			Port:     getEnvInt("SERVER_PORT", 8080),
			LogLevel: getEnv("LOG_LEVEL", "info"),
		},
		SMS: SMSConfig{
			APIURL: getEnv("SMS_API_URL", "http://localhost:3001/send"),
			Token:  getEnv("SMS_API_TOKEN", "mock-token"),
		},
		App: AppConfig{
			BatchSize:          getEnvInt("BATCH_SIZE", 2),
			ProcessingInterval: getEnvDuration("PROCESSING_INTERVAL", 2*time.Minute),
			ShutdownTimeout:    getEnvDuration("SHUTDOWN_TIMEOUT", 30*time.Second),
		},
	}

	// Validate critical configuration
	if err := config.validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return config, nil
}

// validate ensures all required configuration values are present and valid
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
	return nil
}

// DatabaseDSN returns the PostgreSQL connection string
func (c *Config) DatabaseDSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Database.Host, c.Database.Port, c.Database.User,
		c.Database.Password, c.Database.Name, c.Database.SSLMode)
}

// RedisAddr returns the Redis connection address
func (c *Config) RedisAddr() string {
	return fmt.Sprintf("%s:%d", c.Redis.Host, c.Redis.Port)
}

// getEnv retrieves an environment variable or returns the default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt retrieves an environment variable as integer or returns the default value
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

// getEnvDuration retrieves an environment variable as duration or returns the default value
func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
