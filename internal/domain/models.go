// Package domain defines core business entities, interfaces, and domain logic.
package domain

import (
	"context"
	"time"
)

type Message struct {
	ID          int       `json:"id" db:"id"`
	PhoneNumber string    `json:"phone_number" db:"phone_number"`
	Content     string    `json:"content" db:"content"`
	Sent        bool      `json:"sent" db:"sent"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

type SentMessageResponse struct {
	Message
	MessageID *string    `json:"message_id,omitempty"`
	CachedAt  *time.Time `json:"cached_at,omitempty"`
}

type SMSDeliveryResponse struct {
	Message   string `json:"message"`
	MessageID string `json:"messageId"`
}

type CachedDelivery struct {
	MessageID string    `json:"message_id"`
	Timestamp time.Time `json:"timestamp"`
}

type ProcessingStatus string

const (
	StatusStopped ProcessingStatus = "stopped"
	StatusRunning ProcessingStatus = "running"
)

type MessageRepository interface {
	GetUnsentMessages(ctx context.Context, limit int) ([]*Message, error)
	MarkAsSent(ctx context.Context, messageID int) error
	GetSentMessages(ctx context.Context) ([]*Message, error)
	CreateMessage(ctx context.Context, phoneNumber, content string) (*Message, error)
}

type CacheRepository interface {
	SetDeliveryCache(ctx context.Context, messageID int, delivery *CachedDelivery) error
	GetDeliveryCache(ctx context.Context, messageID int) (*CachedDelivery, error)
	GetMultipleDeliveryCache(ctx context.Context, messageIDs []int) (map[int]*CachedDelivery, error)
}

type SMSProvider interface {
	SendMessage(ctx context.Context, phoneNumber, content string) (*SMSDeliveryResponse, error)
}

type MessageService interface {
	ProcessMessages(ctx context.Context) error
	GetSentMessagesWithCache(ctx context.Context) ([]*SentMessageResponse, error)
}

type ProcessingController interface {
	Start() error
	Stop() error
	IsRunning() bool
}
