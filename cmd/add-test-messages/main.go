package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"math/rand"

	_ "github.com/lib/pq"

	"github.com/go-message-dispatcher/internal/config"
)

func randomPhone() string {
	// Common country codes: 1 (US/CA), 44 (UK), 49 (DE), 90 (TR), 359 (BG), 86 (CN)
	countryCodes := []string{"1", "44", "49", "90", "359", "86", "91", "33", "39", "81"}
	countryCode := countryCodes[rand.Intn(len(countryCodes))] // #nosec G404 -- Using math/rand is acceptable for test data generation

	// Generates 9-10 digit, cause the total length is 10-20 chars
	subscriberDigits := 10
	if len(countryCode) >= 3 {
		subscriberDigits = 9 // Shorter number for longer country codes
	}

	var subscriberNumber int64
	if subscriberDigits == 10 {
		subscriberNumber = rand.Int63n(9000000000) + 1000000000 // #nosec G404 -- Using math/rand is acceptable for test data generation
	} else {
		subscriberNumber = rand.Int63n(900000000) + 100000000 // #nosec G404 -- Using math/rand is acceptable for test data generation
	}

	return fmt.Sprintf("+%s%d", countryCode, subscriberNumber)
}

func main() {
	msgCounts := flag.Int("msg_count", 3, "How many entries it should put into db.")
	startNumber := flag.Int("start_number", 0, "default start number to start, simple way to track the messages")
	flag.Parse()

	if *msgCounts == 0 {
		log.Println("You can define the count of messages by using msg_count flag")
		log.Println("By default creates only 3 new messages.")
	} else {
		log.Printf("Will create %d messages.", *msgCounts)
	}

	if *startNumber == 0 {
		log.Println("You can set the start number at the end of message content, for tracking. using start_number.")
	} else {
		log.Printf("Extension number for contents will start from %d", *startNumber)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	db, err := sql.Open("postgres", cfg.DatabaseDSN())
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer func() { _ = db.Close() }()

	for counter := range *msgCounts {
		msgContent := fmt.Sprintf("Test message %d", *startNumber+counter+1)
		phoneNumber := randomPhone()
		_, err := db.Exec(`
			INSERT INTO messages (phone_number, content, scheduled_at, created_at)
			VALUES ($1, $2, NOW() + INTERVAL '1 minute', NOW())
		`, phoneNumber, msgContent)
		if err != nil {
			_ = db.Close()                                  // Close connection before exiting
			log.Fatalf("Failed to insert message: %v", err) //nolint:gocritic // exitAfterDefer is acceptable here as we manually close db
		}
	}

	fmt.Printf("%d test messages added successfully\n", *msgCounts)
}
