// Package handler provides HTTP request handlers for the REST API endpoints.
// It implements the API layer that exposes message processing controls and monitoring.
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-message-dispatcher/internal/domain"
	"go.uber.org/zap"
)

// MessageHandler handles HTTP requests for message-related operations
type MessageHandler struct {
	messageService       domain.MessageService
	processingController domain.ProcessingController
	logger               *zap.Logger
	version              VersionInfo
}

// VersionInfo holds build and version information
type VersionInfo struct {
	Version   string `json:"version"`
	BuildTime string `json:"build_time"`
	GitCommit string `json:"git_commit"`
}

// NewMessageHandler creates a new message handler with all dependencies
func NewMessageHandler(messageService domain.MessageService, processingController domain.ProcessingController, logger *zap.Logger, versionInfo VersionInfo) *MessageHandler {
	return &MessageHandler{
		messageService:       messageService,
		processingController: processingController,
		logger:               logger,
		version:              versionInfo,
	}
}

// ControlResponse represents API responses for start/stop operations
type ControlResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// SentMessagesResponse represents the API response for listing sent messages
type SentMessagesResponse struct {
	Messages []*domain.SentMessageResponse `json:"messages"`
	Total    int                           `json:"total"`
}

// ErrorResponse represents API error responses
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

// StartProcessing starts the automatic message processing
// @Summary Start message processing
// @Description Starts the automatic message processing system
// @Tags messaging
// @Accept json
// @Produce json
// @Success 200 {object} ControlResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/messaging/start [post]
func (h *MessageHandler) StartProcessing(c *gin.Context) {
	err := h.processingController.Start()
	if err != nil {
		h.logger.Error("Failed to start message processing", zap.Error(err))
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "start_failed",
			Message: err.Error(),
		})
		return
	}

	h.logger.Info("Message processing started via API")
	c.JSON(http.StatusOK, ControlResponse{
		Status:  "started",
		Message: "Message processing started successfully",
	})
}

// StopProcessing stops the automatic message processing
// @Summary Stop message processing
// @Description Stops the automatic message processing system
// @Tags messaging
// @Accept json
// @Produce json
// @Success 200 {object} ControlResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/messaging/stop [post]
func (h *MessageHandler) StopProcessing(c *gin.Context) {
	err := h.processingController.Stop()
	if err != nil {
		h.logger.Error("Failed to stop message processing", zap.Error(err))
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "stop_failed",
			Message: err.Error(),
		})
		return
	}

	h.logger.Info("Message processing stopped via API")
	c.JSON(http.StatusOK, ControlResponse{
		Status:  "stopped",
		Message: "Message processing stopped successfully",
	})
}

// GetSentMessages retrieves all sent messages with cached delivery data
// @Summary List sent messages
// @Description Retrieves all messages that have been sent, including delivery metadata from cache
// @Tags messages
// @Accept json
// @Produce json
// @Success 200 {object} SentMessagesResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/messages/sent [get]
func (h *MessageHandler) GetSentMessages(c *gin.Context) {
	messages, err := h.messageService.GetSentMessagesWithCache(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to retrieve sent messages", zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "retrieval_failed",
			Message: "Failed to retrieve sent messages",
		})
		return
	}

	response := SentMessagesResponse{
		Messages: messages,
		Total:    len(messages),
	}

	c.JSON(http.StatusOK, response)
}

// HealthCheck provides a health check endpoint for monitoring
// @Summary Health check
// @Description Returns the health status of the service
// @Tags health
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /health [get]
func (h *MessageHandler) HealthCheck(c *gin.Context) {
	status := "stopped"
	if h.processingController.IsRunning() {
		status = "running"
	}

	c.JSON(http.StatusOK, gin.H{
		"status":            "healthy",
		"processing_status": status,
		"version":           h.version,
		"timestamp":         gin.H{"time": "now"},
	})
}

// Version provides version and build information
// @Summary Version information
// @Description Returns version, build time, and git commit information
// @Tags info
// @Accept json
// @Produce json
// @Success 200 {object} VersionInfo
// @Router /version [get]
func (h *MessageHandler) Version(c *gin.Context) {
	c.JSON(http.StatusOK, h.version)
}
