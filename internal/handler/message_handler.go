// Package handler provides HTTP request handlers for the REST API endpoints.
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/go-message-dispatcher/internal/domain"
)

type MessageHandler struct {
	messageService       domain.MessageService
	processingController domain.ProcessingController
	logger               *zap.Logger
	version              VersionInfo
	dbHealthChecker      domain.HealthChecker
	redisHealthChecker   domain.HealthChecker
}

type VersionInfo struct {
	Version   string `json:"version"`
	BuildTime string `json:"build_time"`
	GitCommit string `json:"git_commit"`
}

func NewMessageHandler(
	messageService domain.MessageService,
	processingController domain.ProcessingController,
	logger *zap.Logger,
	versionInfo VersionInfo,
	dbHealthChecker domain.HealthChecker,
	redisHealthChecker domain.HealthChecker,
) *MessageHandler {
	return &MessageHandler{
		messageService:       messageService,
		processingController: processingController,
		logger:               logger,
		version:              versionInfo,
		dbHealthChecker:      dbHealthChecker,
		redisHealthChecker:   redisHealthChecker,
	}
}

type ControlResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

type SentMessagesResponse struct {
	Messages []*domain.SentMessageResponse `json:"messages"`
	Total    int                           `json:"total"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

func (h *MessageHandler) StartProcessing(c *gin.Context) {
	isRunning := h.processingController.IsRunning()
	if isRunning {
		c.JSON(http.StatusOK, ControlResponse{
			Status:  "running",
			Message: "Message processing is already running",
		})
		return
	}

	err := h.processingController.Start()
	if err != nil {
		h.logger.Error("Failed to start message processing", zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "start_failed",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, ControlResponse{
		Status:  "started",
		Message: "Message processing started successfully",
	})
}

func (h *MessageHandler) StopProcessing(c *gin.Context) {
	isRunning := h.processingController.IsRunning()
	if !isRunning {
		c.JSON(http.StatusOK, ControlResponse{
			Status:  "stopped",
			Message: "Message processing is not running",
		})
		return
	}

	err := h.processingController.Stop()
	if err != nil {
		h.logger.Error("Failed to stop message processing", zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "stop_failed",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, ControlResponse{
		Status:  "stopped",
		Message: "Message processing stopped successfully",
	})
}

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

func (h *MessageHandler) HealthCheck(c *gin.Context) {
	ctx := c.Request.Context()

	status := "stopped"
	if h.processingController.IsRunning() {
		status = "running"
	}

	health := gin.H{
		"status":            "healthy",
		"processing_status": status,
		"version":           h.version,
		"dependencies":      gin.H{},
	}

	overallHealthy := true

	if err := h.dbHealthChecker.CheckHealth(ctx); err != nil {
		h.logger.Warn("Database health check failed", zap.Error(err))
		health["dependencies"].(gin.H)["database"] = "unhealthy"
		health["status"] = "degraded"
		overallHealthy = false
	} else {
		health["dependencies"].(gin.H)["database"] = "healthy"
	}

	if err := h.redisHealthChecker.CheckHealth(ctx); err != nil {
		h.logger.Warn("Redis health check failed", zap.Error(err))
		health["dependencies"].(gin.H)["redis"] = "unhealthy"
		health["status"] = "degraded"
	} else {
		health["dependencies"].(gin.H)["redis"] = "healthy"
	}

	if !overallHealthy {
		c.JSON(http.StatusServiceUnavailable, health)
		return
	}

	c.JSON(http.StatusOK, health)
}

func (h *MessageHandler) Version(c *gin.Context) {
	c.JSON(http.StatusOK, h.version)
}
