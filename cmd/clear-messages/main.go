package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	_ "github.com/lib/pq"

	"github.com/go-message-dispatcher/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	db, err := sql.Open("postgres", cfg.DatabaseDSN())
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	_, err = db.ExecContext(context.Background(), "TRUNCATE TABLE messages RESTART IDENTITY")
	if err != nil {
		_ = db.Close()
		log.Fatalf("Failed to clear messages: %v", err)
	}

	_ = db.Close()
	fmt.Println("All messages cleared successfully")
}
