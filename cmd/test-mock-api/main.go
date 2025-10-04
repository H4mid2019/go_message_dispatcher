package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type SMSRequest struct {
	PhoneNumber string `json:"phone_number"`
	Content     string `json:"content"`
}

func main() {
	smsReq := SMSRequest{
		PhoneNumber: "+1234567890",
		Content:     "Test message",
	}

	jsonData, err := json.Marshal(smsReq)
	if err != nil {
		fmt.Printf("Error marshaling JSON: %v\n", err)
		return
	}

	const requestTimeout = 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "http://localhost:3001/send", bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Printf("Error creating request: %v\n", err)
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		fmt.Printf("Error making request: %v\n", err)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading response: %v\n", err)
		return
	}

	fmt.Printf("Status: %s\n", resp.Status)
	fmt.Printf("Response: %s\n", string(body))

	healthReq, err := http.NewRequestWithContext(ctx, "GET", "http://localhost:3001/health", http.NoBody)
	if err != nil {
		fmt.Printf("Error creating health request: %v\n", err)
		return
	}

	healthResp, err := client.Do(healthReq)
	if err != nil {
		fmt.Printf("Error getting health: %v\n", err)
		return
	}
	defer func() { _ = healthResp.Body.Close() }()

	healthBody, err := io.ReadAll(healthResp.Body)
	if err != nil {
		fmt.Printf("Error reading health response: %v\n", err)
		return
	}

	fmt.Printf("Health Status: %s\n", healthResp.Status)
	fmt.Printf("Health Response: %s\n", string(healthBody))
}
