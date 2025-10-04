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

	messages := []struct {
		phone   string
		content string
	}{
		{"+905551111111", "Test message 1"},
		{"+359888886645", "Test message 2"},
		{"+905552222222", "Test message 3"},
		{"+905553333333", "Test message 4"},
	}

	for _, msg := range messages {
		_, err := db.ExecContext(context.Background(),
			"INSERT INTO messages (phone_number, content) VALUES ($1, $2)",
			msg.phone, msg.content)
		if err != nil {
			log.Printf("Failed to insert message: %v", err)
		} else {
			fmt.Printf("Inserted message: %s - %s\n", msg.phone, msg.content)
		}
	}

	fmt.Println("Test messages added successfully")
}
