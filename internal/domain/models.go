// Package domain defines the core business entities and interfaces for the message dispatcher.
// These models represent the fundamental concepts in our messaging domain.
package domain

import (
	"context"
	"time"
)

// Message represents a text message to be sent to a phone number
type Message struct {
	ID          int       `json:"id" db:"id"`
	PhoneNumber string    `json:"phone_number" db:"phone_number"`
	Content     string    `json:"content" db:"content"`
	Sent        bool      `json:"sent" db:"sent"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

// SentMessageResponse represents the API response format for sent messages
// including cached delivery metadata from Redis
type SentMessageResponse struct {
	Message
	MessageID *string    `json:"message_id,omitempty"`
	CachedAt  *time.Time `json:"cached_at,omitempty"`
}

// SMSDeliveryResponse represents the response from the SMS provider API
type SMSDeliveryResponse struct {
	Message   string `json:"message"`
	MessageID string `json:"messageId"`
}

// CachedDelivery stores delivery metadata in Redis for enhanced API responses
type CachedDelivery struct {
	MessageID string    `json:"message_id"`
	Timestamp time.Time `json:"timestamp"`
}

// ProcessingStatus represents the current state of the message processing system
type ProcessingStatus string

const (
	StatusStopped ProcessingStatus = "stopped"
	StatusRunning ProcessingStatus = "running"
)

// MessageRepository defines the interface for message data access operations
type MessageRepository interface {
	// GetUnsentMessages retrieves a specified number of unsent messages in FIFO order
	GetUnsentMessages(ctx context.Context, limit int) ([]*Message, error)

	// MarkAsSent updates the sent status of a message to true
	MarkAsSent(ctx context.Context, messageID int) error

	// GetSentMessages retrieves all messages that have been sent
	GetSentMessages(ctx context.Context) ([]*Message, error)

	// CreateMessage adds a new message to the queue (primarily for testing)
	CreateMessage(ctx context.Context, phoneNumber, content string) (*Message, error)
}

// CacheRepository defines the interface for caching delivery metadata
type CacheRepository interface {
	// SetDeliveryCache stores delivery metadata in cache with expiration
	SetDeliveryCache(ctx context.Context, messageID int, delivery *CachedDelivery) error

	// GetDeliveryCache retrieves cached delivery metadata
	GetDeliveryCache(ctx context.Context, messageID int) (*CachedDelivery, error)

	// GetMultipleDeliveryCache retrieves cached delivery metadata for multiple messages
	GetMultipleDeliveryCache(ctx context.Context, messageIDs []int) (map[int]*CachedDelivery, error)
}

// SMSProvider defines the interface for sending SMS messages through external providers
type SMSProvider interface {
	// SendMessage sends an SMS message and returns the provider's response
	SendMessage(ctx context.Context, phoneNumber, content string) (*SMSDeliveryResponse, error)
}

// MessageService defines the business logic interface for message processing
type MessageService interface {
	// ProcessMessages retrieves and sends a batch of unsent messages
	ProcessMessages(ctx context.Context) error

	// GetSentMessagesWithCache retrieves sent messages enhanced with cached delivery data
	GetSentMessagesWithCache(ctx context.Context) ([]*SentMessageResponse, error)
}

// ProcessingController defines the interface for controlling the message processing scheduler
type ProcessingController interface {
	// Start begins the automatic message processing
	Start() error

	// Stop halts the automatic message processing
	Stop() error

	// IsRunning returns the current processing status
	IsRunning() bool
}
