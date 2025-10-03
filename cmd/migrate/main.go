package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/go-message-dispatcher/internal/config"
	_ "github.com/lib/pq"
)

var (
	version   = "dev"
	buildTime = "unknown"
	gitCommit = "unknown"
)

func main() {
	showVersion := flag.Bool("version", false, "Show version information")
	flag.Parse()

	if *showVersion {
		fmt.Printf("Message Dispatcher Migration Tool\n")
		fmt.Printf("Version: %s\n", version)
		fmt.Printf("Build Time: %s\n", buildTime)
		fmt.Printf("Git Commit: %s\n", gitCommit)
		return
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	db, err := sql.Open("postgres", cfg.DatabaseDSN())
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	err = db.Ping()
	if err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	err = runMigrations(db)
	if err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	fmt.Println("Database migrations completed successfully")
}

func runMigrations(db *sql.DB) error {
	migrationFiles := []string{
		"migrations/001_initial_schema.sql",
	}

	for _, migrationFile := range migrationFiles {
		err := runMigrationFile(db, migrationFile)
		if err != nil {
			return fmt.Errorf("failed to run migration %s: %w", migrationFile, err)
		}
	}

	return nil
}

func runMigrationFile(db *sql.DB, filename string) error {
	content, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read migration file: %w", err)
	}

	_, err = db.Exec(string(content))
	if err != nil {
		return fmt.Errorf("failed to execute migration SQL: %w", err)
	}

	return nil
}
