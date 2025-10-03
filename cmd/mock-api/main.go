package main

import (
	"crypto/rand"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type SMSRequest struct {
	PhoneNumber string `json:"phone_number" binding:"required"`
	Content     string `json:"content" binding:"required"`
}

type SMSResponse struct {
	Message   string `json:"message"`
	MessageID string `json:"messageId"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

type HealthResponse struct {
	Status    string `json:"status"`
	Service   string `json:"service"`
	Timestamp string `json:"timestamp"`
}

func generateMessageID() string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	const idLength = 9
	b := make([]byte, idLength)

	randomBytes := make([]byte, idLength)
	_, err := rand.Read(randomBytes)
	if err != nil {
		// Fallback to timestamp-based ID if crypto/rand fails
		const fallbackMod = 1000000000
		return fmt.Sprintf("msg_%d", time.Now().UnixNano()%fallbackMod)
	}

	for i := range b {
		b[i] = charset[randomBytes[i]%byte(len(charset))]
	}
	return "msg_" + string(b)
}

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

func sendSMSHandler(c *gin.Context) {
	var req SMSRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Missing required fields",
			Message: "phone_number and content are required",
		})
		return
	}

	const responseDelay = 100 * time.Millisecond
	time.Sleep(responseDelay)
	messageID := generateMessageID()

	c.JSON(http.StatusOK, SMSResponse{
		Message:   "Accepted",
		MessageID: messageID,
	})
}

func healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, HealthResponse{
		Status:    "healthy",
		Service:   "mock-sms-api",
		Timestamp: time.Now().Format(time.RFC3339),
	})
}

func main() {
	gin.SetMode(gin.ReleaseMode)

	r := gin.New()
	r.Use(loggerMiddleware())
	r.Use(gin.Recovery())

	r.POST("/send", sendSMSHandler)
	r.GET("/health", healthHandler)

	port := "3001"
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
