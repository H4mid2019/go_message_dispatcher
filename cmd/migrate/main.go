// Package main provides a simple database migration tool for setting up the initial schema.
// This tool runs the SQL migration files against the configured PostgreSQL database.
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/go-message-dispatcher/internal/config"
	_ "github.com/lib/pq"
)

// Build information set via ldflags during build
var (
	version   = "dev"
	buildTime = "unknown"
	gitCommit = "unknown"
)

func main() {
	// Handle version flag
	showVersion := flag.Bool("version", false, "Show version information")
	flag.Parse()

	if *showVersion {
		fmt.Printf("Message Dispatcher Migration Tool\n")
		fmt.Printf("Version: %s\n", version)
		fmt.Printf("Build Time: %s\n", buildTime)
		fmt.Printf("Git Commit: %s\n", gitCommit)
		return
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Connect to database
	db, err := sql.Open("postgres", cfg.DatabaseDSN())
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Test connection
	err = db.Ping()
	if err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	fmt.Println("Connected to database successfully")

	// Run migrations
	err = runMigrations(db)
	if err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	fmt.Println("Database migrations completed successfully")
}

// runMigrations executes all SQL migration files in the migrations directory
func runMigrations(db *sql.DB) error {
	migrationFiles := []string{
		"migrations/001_initial_schema.sql",
	}

	for _, migrationFile := range migrationFiles {
		fmt.Printf("Running migration: %s\n", migrationFile)

		err := runMigrationFile(db, migrationFile)
		if err != nil {
			return fmt.Errorf("failed to run migration %s: %w", migrationFile, err)
		}
	}

	return nil
}

// runMigrationFile executes a single SQL migration file
func runMigrationFile(db *sql.DB, filename string) error {
	// Read the migration file
	content, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read migration file: %w", err)
	}

	// Execute the SQL
	_, err = db.Exec(string(content))
	if err != nil {
		return fmt.Errorf("failed to execute migration SQL: %w", err)
	}

	fmt.Printf("✓ Migration %s completed\n", filepath.Base(filename))
	return nil
}
