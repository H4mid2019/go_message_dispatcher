package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-message-dispatcher/internal/domain"
	"go.uber.org/zap"
)

type MessageHandler struct {
	messageService       domain.MessageService
	processingController domain.ProcessingController
	logger               *zap.Logger
	version              VersionInfo
}

type VersionInfo struct {
	Version   string `json:"version"`
	BuildTime string `json:"build_time"`
	GitCommit string `json:"git_commit"`
}

func NewMessageHandler(messageService domain.MessageService, processingController domain.ProcessingController, logger *zap.Logger, versionInfo VersionInfo) *MessageHandler {
	return &MessageHandler{
		messageService:       messageService,
		processingController: processingController,
		logger:               logger,
		version:              versionInfo,
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
	err := h.processingController.Start()
	if err != nil {
		h.logger.Error("Failed to start message processing", zap.Error(err))
		c.JSON(http.StatusBadRequest, ErrorResponse{
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
	err := h.processingController.Stop()
	if err != nil {
		h.logger.Error("Failed to stop message processing", zap.Error(err))
		c.JSON(http.StatusBadRequest, ErrorResponse{
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
	status := "stopped"
	if h.processingController.IsRunning() {
		status = "running"
	}

	c.JSON(http.StatusOK, gin.H{
		"status":            "healthy",
		"processing_status": status,
		"version":           h.version,
	})
}

func (h *MessageHandler) Version(c *gin.Context) {
	c.JSON(http.StatusOK, h.version)
}
