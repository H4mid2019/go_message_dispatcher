package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type SMSRequest struct {
	PhoneNumber string `json:"phone_number"`
	Content     string `json:"content"`
}

type SMSResponse struct {
	Message   string `json:"message"`
	MessageID string `json:"messageId"`
}

func main() {
	// Test data
	req := SMSRequest{
		PhoneNumber: "+1234567890",
		Content:     "This is a test message from the Go mock API",
	}

	// Convert to JSON
	jsonData, err := json.Marshal(req)
	if err != nil {
		fmt.Printf("Error marshaling JSON: %v\n", err)
		return
	}

	// Make request
	resp, err := http.Post("http://localhost:3001/send", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Printf("Error making request: %v\n", err)
		return
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading response: %v\n", err)
		return
	}

	fmt.Printf("Status: %s\n", resp.Status)
	fmt.Printf("Response: %s\n", string(body))

	// Test health endpoint
	fmt.Println("\nTesting health endpoint...")
	healthResp, err := http.Get("http://localhost:3001/health")
	if err != nil {
		fmt.Printf("Error getting health: %v\n", err)
		return
	}
	defer healthResp.Body.Close()

	healthBody, err := io.ReadAll(healthResp.Body)
	if err != nil {
		fmt.Printf("Error reading health response: %v\n", err)
		return
	}

	fmt.Printf("Health Status: %s\n", healthResp.Status)
	fmt.Printf("Health Response: %s\n", string(healthBody))
}
