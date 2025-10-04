package service

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-message-dispatcher/internal/domain"
)

type HTTPSMSProvider struct {
	client  *http.Client
	baseURL string
	token   string
}

func NewHTTPSMSProvider(baseURL, token string) *HTTPSMSProvider {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, // #nosec G402
		},
	}

	return &HTTPSMSProvider{
		client: &http.Client{
			Timeout:   6 * time.Second,
			Transport: transport,
		},
		baseURL: baseURL,
		token:   token,
	}
}

type SMSRequest struct {
	PhoneNumber string `json:"phone_number"`
	Content     string `json:"content"`
}

func (p *HTTPSMSProvider) SendMessage(ctx context.Context, phoneNumber, content string) (*domain.SMSDeliveryResponse, error) {
	request := SMSRequest{
		PhoneNumber: phoneNumber,
		Content:     content,
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal SMS request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.token))

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send SMS request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("SMS provider returned status %d", resp.StatusCode)
	}

	var smsResponse domain.SMSDeliveryResponse
	err = json.NewDecoder(resp.Body).Decode(&smsResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to decode SMS response: %w", err)
	}

	return &smsResponse, nil
}

type MessageService struct {
	messageRepo domain.MessageRepository
	cacheRepo   domain.CacheRepository
	smsProvider domain.SMSProvider
}

func NewMessageService(
	messageRepo domain.MessageRepository,
	cacheRepo domain.CacheRepository,
	smsProvider domain.SMSProvider,
) *MessageService {
	return &MessageService{
		messageRepo: messageRepo,
		cacheRepo:   cacheRepo,
		smsProvider: smsProvider,
	}
}

func (s *MessageService) ProcessMessages(ctx context.Context) error {
	const batchSize = 2
	messages, err := s.messageRepo.GetUnsentMessages(ctx, batchSize)
	if err != nil {
		return fmt.Errorf("failed to retrieve unsent messages: %w", err)
	}

	if len(messages) == 0 {
		return nil
	}

	anyFailure := false
	for _, message := range messages {
		err := s.processSingleMessage(ctx, message)
		if err != nil {
			anyFailure = true
		}
	}

	if anyFailure {
		return fmt.Errorf("one or more messages failed to process")
	}

	return nil
}

func (s *MessageService) processSingleMessage(ctx context.Context, message *domain.Message) error {
	response, err := s.smsProvider.SendMessage(ctx, message.PhoneNumber, message.Content)
	if err != nil {
		return fmt.Errorf("failed to send SMS for message %d: %w", message.ID, err)
	}

	err = s.messageRepo.MarkAsSent(ctx, message.ID)
	if err != nil {
		return fmt.Errorf("failed to mark message %d as sent: %w", message.ID, err)
	}

	cachedDelivery := &domain.CachedDelivery{
		MessageID: response.MessageID,
		Timestamp: time.Now(),
	}

	_ = s.cacheRepo.SetDeliveryCache(ctx, message.ID, cachedDelivery)

	return nil
}

func (s *MessageService) GetSentMessagesWithCache(ctx context.Context) ([]*domain.SentMessageResponse, error) {
	messages, err := s.messageRepo.GetSentMessages(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve sent messages: %w", err)
	}

	if len(messages) == 0 {
		return []*domain.SentMessageResponse{}, nil
	}

	messageIDs := make([]int, len(messages))
	for i, msg := range messages {
		messageIDs[i] = msg.ID
	}

	cachedData, err := s.cacheRepo.GetMultipleDeliveryCache(ctx, messageIDs)
	if err != nil {
		cachedData = make(map[int]*domain.CachedDelivery)
	}

	responses := make([]*domain.SentMessageResponse, len(messages))
	for i, msg := range messages {
		response := &domain.SentMessageResponse{
			Message: *msg,
		}

		if cached, exists := cachedData[msg.ID]; exists {
			response.MessageID = &cached.MessageID
			response.CachedAt = &cached.Timestamp
		}

		responses[i] = response
	}

	return responses, nil
}
