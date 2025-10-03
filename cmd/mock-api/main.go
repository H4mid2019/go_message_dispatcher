// Package main provides a mock SMS API server for testing the message dispatcher.
// This simulates an external SMS provider API for development and testing.
package main

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// SMSRequest represents the expected SMS request payload
type SMSRequest struct {
	PhoneNumber string `json:"phone_number" binding:"required"`
	Content     string `json:"content" binding:"required"`
}

// SMSResponse represents the SMS API response
type SMSResponse struct {
	Message   string `json:"message"`
	MessageID string `json:"messageId"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Status    string `json:"status"`
	Service   string `json:"service"`
	Timestamp string `json:"timestamp"`
}

// generateMessageID creates a random message ID similar to the JS version
func generateMessageID() string {
	// Generate random string similar to Math.random().toString(36).substr(2, 9)
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 9)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return "msg_" + string(b)
}

// loggerMiddleware logs incoming requests
func loggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		fmt.Printf("%s - %s %s - %d - %v\n",
			start.Format(time.RFC3339),
			c.Request.Method,
			c.Request.URL.Path,
			c.Writer.Status(),
			time.Since(start),
		)
	}
}

// sendSMSHandler handles the POST /send endpoint
func sendSMSHandler(c *gin.Context) {
	var req SMSRequest

	// Bind and validate JSON request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Missing required fields",
			Message: "phone_number and content are required",
		})
		return
	}

	// Simulate processing delay (100ms like the JS version)
	time.Sleep(100 * time.Millisecond)

	// Generate a mock message ID
	messageID := generateMessageID()

	// Log the mock SMS (truncate content like JS version)
	contentPreview := req.Content
	if len(contentPreview) > 50 {
		contentPreview = contentPreview[:50] + "..."
	}
	fmt.Printf("📱 Mock SMS sent to %s: \"%s\"\n", req.PhoneNumber, contentPreview)

	// Return success response matching the expected format
	c.JSON(http.StatusOK, SMSResponse{
		Message:   "Accepted",
		MessageID: messageID,
	})
}

// healthHandler handles the GET /health endpoint
func healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, HealthResponse{
		Status:    "healthy",
		Service:   "mock-sms-api",
		Timestamp: time.Now().Format(time.RFC3339),
	})
}

func main() {
	// Initialize random seed
	rand.Seed(time.Now().UnixNano())

	// Set Gin to release mode for cleaner output
	gin.SetMode(gin.ReleaseMode)

	// Create Gin router
	r := gin.New()

	// Add custom logger middleware
	r.Use(loggerMiddleware())
	r.Use(gin.Recovery())

	// Routes
	r.POST("/send", sendSMSHandler)
	r.GET("/health", healthHandler)

	// Server configuration
	port := "3001"

	// Startup messages
	fmt.Println("🚀 Mock SMS API server starting...")
	fmt.Printf("📡 SMS endpoint: http://localhost:%s/send\n", port)
	fmt.Printf("❤️ Health check: http://localhost:%s/health\n", port)
	fmt.Printf("🚀 Mock SMS API server running on port %s\n", port)

	// Start server
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
