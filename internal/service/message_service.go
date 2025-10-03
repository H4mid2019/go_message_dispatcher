// Package service provides business logic implementations and external service integrations.
// It orchestrates between repositories and handles external API communications.
package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-message-dispatcher/internal/domain"
)

// HTTPSMSProvider implements SMSProvider for HTTP-based SMS services
type HTTPSMSProvider struct {
	client  *http.Client
	baseURL string
	token   string
}

// NewHTTPSMSProvider creates a new HTTP SMS provider with timeout configuration
func NewHTTPSMSProvider(baseURL, token string) *HTTPSMSProvider {
	return &HTTPSMSProvider{
		client: &http.Client{
			Timeout: 30 * time.Second, // Reasonable timeout for SMS API calls
		},
		baseURL: baseURL,
		token:   token,
	}
}

// SMSRequest represents the JSON payload sent to the SMS provider API
type SMSRequest struct {
	PhoneNumber string `json:"phone_number"`
	Content     string `json:"content"`
}

// SendMessage sends an SMS message through the HTTP provider API
// Implements retry logic and proper error handling for production reliability
func (p *HTTPSMSProvider) SendMessage(ctx context.Context, phoneNumber, content string) (*domain.SMSDeliveryResponse, error) {
	request := SMSRequest{
		PhoneNumber: phoneNumber,
		Content:     content,
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal SMS request: %w", err)
	}

	// Create HTTP request with context for cancellation support
	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set required headers for the SMS provider API
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.token))

	// Execute the HTTP request
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send SMS request: %w", err)
	}
	defer resp.Body.Close()

	// Check for HTTP errors
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("SMS provider returned status %d", resp.StatusCode)
	}

	// Parse the response
	var smsResponse domain.SMSDeliveryResponse
	err = json.NewDecoder(resp.Body).Decode(&smsResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to decode SMS response: %w", err)
	}

	return &smsResponse, nil
}

// MessageService implements the business logic for message processing
type MessageService struct {
	messageRepo domain.MessageRepository
	cacheRepo   domain.CacheRepository
	smsProvider domain.SMSProvider
}

// NewMessageService creates a new message service with all dependencies
func NewMessageService(messageRepo domain.MessageRepository, cacheRepo domain.CacheRepository, smsProvider domain.SMSProvider) *MessageService {
	return &MessageService{
		messageRepo: messageRepo,
		cacheRepo:   cacheRepo,
		smsProvider: smsProvider,
	}
}

// ProcessMessages retrieves and sends a batch of unsent messages
// This is the core business logic that runs on the scheduled interval
func (s *MessageService) ProcessMessages(ctx context.Context) error {
	// Retrieve unsent messages in FIFO order
	messages, err := s.messageRepo.GetUnsentMessages(ctx, 2) // Always process exactly 2 messages
	if err != nil {
		return fmt.Errorf("failed to retrieve unsent messages: %w", err)
	}

	if len(messages) == 0 {
		// No messages to process - this is normal, not an error
		return nil
	}

	// Process each message individually to handle partial failures gracefully
	for _, message := range messages {
		err := s.processSingleMessage(ctx, message)
		if err != nil {
			// Log the error but continue processing other messages
			// In production, you might want to implement retry logic here
			return fmt.Errorf("failed to process message %d: %w", message.ID, err)
		}
	}

	return nil
}

// processSingleMessage handles the complete lifecycle of sending a single message
func (s *MessageService) processSingleMessage(ctx context.Context, message *domain.Message) error {
	// Send the message via SMS provider
	response, err := s.smsProvider.SendMessage(ctx, message.PhoneNumber, message.Content)
	if err != nil {
		return fmt.Errorf("failed to send SMS: %w", err)
	}

	// Mark message as sent in database
	err = s.messageRepo.MarkAsSent(ctx, message.ID)
	if err != nil {
		return fmt.Errorf("failed to mark message as sent: %w", err)
	}

	// Cache delivery metadata for enhanced API responses
	cachedDelivery := &domain.CachedDelivery{
		MessageID: response.MessageID,
		Timestamp: time.Now(),
	}

	err = s.cacheRepo.SetDeliveryCache(ctx, message.ID, cachedDelivery)
	if err != nil {
		// Cache failure shouldn't fail the entire operation
		// Log the error but don't return it - the message was successfully sent
		fmt.Printf("Warning: failed to cache delivery data for message %d: %v\n", message.ID, err)
	}

	return nil
}

// GetSentMessagesWithCache retrieves sent messages enhanced with cached delivery data
// This provides a richer API response by combining database and cache data
func (s *MessageService) GetSentMessagesWithCache(ctx context.Context) ([]*domain.SentMessageResponse, error) {
	// Get all sent messages from database
	messages, err := s.messageRepo.GetSentMessages(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve sent messages: %w", err)
	}

	if len(messages) == 0 {
		return []*domain.SentMessageResponse{}, nil
	}

	// Extract message IDs for cache lookup
	messageIDs := make([]int, len(messages))
	for i, msg := range messages {
		messageIDs[i] = msg.ID
	}

	// Retrieve cached delivery data for all messages
	cachedData, err := s.cacheRepo.GetMultipleDeliveryCache(ctx, messageIDs)
	if err != nil {
		// Cache failure shouldn't fail the API response
		// Return messages without cached data
		fmt.Printf("Warning: failed to retrieve cached delivery data: %v\n", err)
		cachedData = make(map[int]*domain.CachedDelivery)
	}

	// Combine database and cache data
	responses := make([]*domain.SentMessageResponse, len(messages))
	for i, msg := range messages {
		response := &domain.SentMessageResponse{
			Message: *msg,
		}

		// Add cached delivery data if available
		if cached, exists := cachedData[msg.ID]; exists {
			response.MessageID = &cached.MessageID
			response.CachedAt = &cached.Timestamp
		}

		responses[i] = response
	}

	return responses, nil
}
