package service

import (
	"context"
	"testing"
	"time"

	"github.com/go-message-dispatcher/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockMessageRepository is a mock implementation for testing
type MockMessageRepository struct {
	mock.Mock
}

func (m *MockMessageRepository) GetUnsentMessages(ctx context.Context, limit int) ([]*domain.Message, error) {
	args := m.Called(ctx, limit)
	return args.Get(0).([]*domain.Message), args.Error(1)
}

func (m *MockMessageRepository) MarkAsSent(ctx context.Context, messageID int) error {
	args := m.Called(ctx, messageID)
	return args.Error(0)
}

func (m *MockMessageRepository) GetSentMessages(ctx context.Context) ([]*domain.Message, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*domain.Message), args.Error(1)
}

func (m *MockMessageRepository) CreateMessage(ctx context.Context, phoneNumber, content string) (*domain.Message, error) {
	args := m.Called(ctx, phoneNumber, content)
	return args.Get(0).(*domain.Message), args.Error(1)
}

// MockCacheRepository is a mock implementation for testing
type MockCacheRepository struct {
	mock.Mock
}

func (m *MockCacheRepository) SetDeliveryCache(ctx context.Context, messageID int, delivery *domain.CachedDelivery) error {
	args := m.Called(ctx, messageID, delivery)
	return args.Error(0)
}

func (m *MockCacheRepository) GetDeliveryCache(ctx context.Context, messageID int) (*domain.CachedDelivery, error) {
	args := m.Called(ctx, messageID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.CachedDelivery), args.Error(1)
}

func (m *MockCacheRepository) GetMultipleDeliveryCache(ctx context.Context, messageIDs []int) (map[int]*domain.CachedDelivery, error) {
	args := m.Called(ctx, messageIDs)
	return args.Get(0).(map[int]*domain.CachedDelivery), args.Error(1)
}

// MockSMSProvider is a mock implementation for testing
type MockSMSProvider struct {
	mock.Mock
}

func (m *MockSMSProvider) SendMessage(ctx context.Context, phoneNumber, content string) (*domain.SMSDeliveryResponse, error) {
	args := m.Called(ctx, phoneNumber, content)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.SMSDeliveryResponse), args.Error(1)
}

func TestMessageService_ProcessMessages_Success(t *testing.T) {
	// Setup mocks
	mockMessageRepo := new(MockMessageRepository)
	mockCacheRepo := new(MockCacheRepository)
	mockSMSProvider := new(MockSMSProvider)

	// Create test messages
	testMessages := []*domain.Message{
		{
			ID:          1,
			PhoneNumber: "+1234567890",
			Content:     "Test message 1",
			Sent:        false,
			CreatedAt:   time.Now(),
		},
		{
			ID:          2,
			PhoneNumber: "+1234567891",
			Content:     "Test message 2",
			Sent:        false,
			CreatedAt:   time.Now(),
		},
	}

	// Setup mock expectations
	mockMessageRepo.On("GetUnsentMessages", mock.Anything, 2).Return(testMessages, nil)

	// SMS provider expectations
	mockSMSProvider.On("SendMessage", mock.Anything, "+1234567890", "Test message 1").
		Return(&domain.SMSDeliveryResponse{Message: "Accepted", MessageID: "msg_123"}, nil)
	mockSMSProvider.On("SendMessage", mock.Anything, "+1234567891", "Test message 2").
		Return(&domain.SMSDeliveryResponse{Message: "Accepted", MessageID: "msg_456"}, nil)

	// Repository update expectations
	mockMessageRepo.On("MarkAsSent", mock.Anything, 1).Return(nil)
	mockMessageRepo.On("MarkAsSent", mock.Anything, 2).Return(nil)

	// Cache expectations
	mockCacheRepo.On("SetDeliveryCache", mock.Anything, 1, mock.AnythingOfType("*domain.CachedDelivery")).Return(nil)
	mockCacheRepo.On("SetDeliveryCache", mock.Anything, 2, mock.AnythingOfType("*domain.CachedDelivery")).Return(nil)

	// Create service
	service := NewMessageService(mockMessageRepo, mockCacheRepo, mockSMSProvider)

	// Execute test
	err := service.ProcessMessages(context.Background())

	// Assertions
	assert.NoError(t, err)
	mockMessageRepo.AssertExpectations(t)
	mockSMSProvider.AssertExpectations(t)
	mockCacheRepo.AssertExpectations(t)
}

func TestMessageService_ProcessMessages_NoMessages(t *testing.T) {
	// Setup mocks
	mockMessageRepo := new(MockMessageRepository)
	mockCacheRepo := new(MockCacheRepository)
	mockSMSProvider := new(MockSMSProvider)

	// Setup expectation for no messages
	mockMessageRepo.On("GetUnsentMessages", mock.Anything, 2).Return([]*domain.Message{}, nil)

	// Create service
	service := NewMessageService(mockMessageRepo, mockCacheRepo, mockSMSProvider)

	// Execute test
	err := service.ProcessMessages(context.Background())

	// Assertions
	assert.NoError(t, err)
	mockMessageRepo.AssertExpectations(t)
	// SMS provider and cache should not be called
	mockSMSProvider.AssertNotCalled(t, "SendMessage")
	mockCacheRepo.AssertNotCalled(t, "SetDeliveryCache")
}

func TestMessageService_GetSentMessagesWithCache_Success(t *testing.T) {
	// Setup mocks
	mockMessageRepo := new(MockMessageRepository)
	mockCacheRepo := new(MockCacheRepository)
	mockSMSProvider := new(MockSMSProvider)

	// Create test data
	sentMessages := []*domain.Message{
		{
			ID:          1,
			PhoneNumber: "+1234567890",
			Content:     "Test message 1",
			Sent:        true,
			CreatedAt:   time.Now(),
		},
	}

	cachedData := map[int]*domain.CachedDelivery{
		1: {
			MessageID: "msg_123",
			Timestamp: time.Now(),
		},
	}

	// Setup expectations
	mockMessageRepo.On("GetSentMessages", mock.Anything).Return(sentMessages, nil)
	mockCacheRepo.On("GetMultipleDeliveryCache", mock.Anything, []int{1}).Return(cachedData, nil)

	// Create service
	service := NewMessageService(mockMessageRepo, mockCacheRepo, mockSMSProvider)

	// Execute test
	result, err := service.GetSentMessagesWithCache(context.Background())

	// Assertions
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, 1, result[0].ID)
	assert.NotNil(t, result[0].MessageID)
	assert.Equal(t, "msg_123", *result[0].MessageID)
	assert.NotNil(t, result[0].CachedAt)

	mockMessageRepo.AssertExpectations(t)
	mockCacheRepo.AssertExpectations(t)
}
