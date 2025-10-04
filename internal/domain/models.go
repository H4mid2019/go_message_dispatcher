package domain

import (
	"context"
	"fmt"
	"time"
)

type Message struct {
	ID          int       `json:"id" db:"id"`
	PhoneNumber string    `json:"phone_number" db:"phone_number"`
	Content     string    `json:"content" db:"content"`
	Sent        bool      `json:"sent" db:"sent"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

func (m *Message) IsValid() error {
	if m.PhoneNumber == "" {
		return fmt.Errorf("phone number is required")
	}
	if len(m.PhoneNumber) < 10 || len(m.PhoneNumber) > 20 {
		return fmt.Errorf("phone number must be between 10 and 20 characters")
	}
	if m.Content == "" {
		return fmt.Errorf("message content is required")
	}
	return nil
}

func (m *Message) ValidatePhoneNumber() bool {
	if m.PhoneNumber == "" {
		return false
	}
	if len(m.PhoneNumber) < 10 || len(m.PhoneNumber) > 20 {
		return false
	}
	return true
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

type HealthChecker interface {
	CheckHealth(ctx context.Context) error
}
