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
	defer db.Close()

	_, err = db.ExecContext(context.Background(),
		"INSERT INTO messages (phone_number, content) VALUES ($1, $2)",
		"+905554444444", "Test message 5 - retry test")
	if err != nil {
		log.Printf("Failed to insert message: %v", err)
	} else {
		fmt.Println("Added test message for retry testing")
	}

	var unsentCount int
	err = db.QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM messages WHERE sent = FALSE").Scan(&unsentCount)
	if err != nil {
		log.Printf("Failed to count unsent messages: %v", err)
	} else {
		fmt.Printf("Unsent messages in database: %d\n", unsentCount)
	}
}
