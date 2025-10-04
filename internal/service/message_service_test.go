package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/go-message-dispatcher/internal/domain"
)

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
	mockMessageRepo := new(MockMessageRepository)
	mockCacheRepo := new(MockCacheRepository)
	mockSMSProvider := new(MockSMSProvider)

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

	mockMessageRepo.On("GetUnsentMessages", mock.Anything, 2).Return(testMessages, nil)
	mockSMSProvider.On("SendMessage", mock.Anything, "+1234567890", "Test message 1").
		Return(&domain.SMSDeliveryResponse{Message: "Accepted", MessageID: "msg_123"}, nil)
	mockSMSProvider.On("SendMessage", mock.Anything, "+1234567891", "Test message 2").
		Return(&domain.SMSDeliveryResponse{Message: "Accepted", MessageID: "msg_456"}, nil)
	mockMessageRepo.On("MarkAsSent", mock.Anything, 1).Return(nil)
	mockMessageRepo.On("MarkAsSent", mock.Anything, 2).Return(nil)
	mockCacheRepo.On("SetDeliveryCache", mock.Anything, 1, mock.AnythingOfType("*domain.CachedDelivery")).Return(nil)
	mockCacheRepo.On("SetDeliveryCache", mock.Anything, 2, mock.AnythingOfType("*domain.CachedDelivery")).Return(nil)

	service := NewMessageService(mockMessageRepo, mockCacheRepo, mockSMSProvider)
	err := service.ProcessMessages(context.Background())
	assert.NoError(t, err)
	mockMessageRepo.AssertExpectations(t)
	mockSMSProvider.AssertExpectations(t)
	mockCacheRepo.AssertExpectations(t)
}

func TestMessageService_ProcessMessages_NoMessages(t *testing.T) {
	mockMessageRepo := new(MockMessageRepository)
	mockCacheRepo := new(MockCacheRepository)
	mockSMSProvider := new(MockSMSProvider)

	mockMessageRepo.On("GetUnsentMessages", mock.Anything, 2).Return([]*domain.Message{}, nil)

	service := NewMessageService(mockMessageRepo, mockCacheRepo, mockSMSProvider)
	err := service.ProcessMessages(context.Background())

	assert.NoError(t, err)
	mockMessageRepo.AssertExpectations(t)
	mockSMSProvider.AssertNotCalled(t, "SendMessage")
	mockCacheRepo.AssertNotCalled(t, "SetDeliveryCache")
}

func TestMessageService_GetSentMessagesWithCache_Success(t *testing.T) {
	mockMessageRepo := new(MockMessageRepository)
	mockCacheRepo := new(MockCacheRepository)
	mockSMSProvider := new(MockSMSProvider)

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

	mockMessageRepo.On("GetSentMessages", mock.Anything).Return(sentMessages, nil)
	mockCacheRepo.On("GetMultipleDeliveryCache", mock.Anything, []int{1}).Return(cachedData, nil)

	service := NewMessageService(mockMessageRepo, mockCacheRepo, mockSMSProvider)
	result, err := service.GetSentMessagesWithCache(context.Background())
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, 1, result[0].ID)
	assert.NotNil(t, result[0].MessageID)
	assert.Equal(t, "msg_123", *result[0].MessageID)
	assert.NotNil(t, result[0].CachedAt)

	mockMessageRepo.AssertExpectations(t)
	mockCacheRepo.AssertExpectations(t)
}

func TestMessageService_ProcessMessages_FirstSucceedsSecondFails(t *testing.T) {
	mockMessageRepo := new(MockMessageRepository)
	mockCacheRepo := new(MockCacheRepository)
	mockSMSProvider := new(MockSMSProvider)

	testMessages := []*domain.Message{
		{ID: 1, PhoneNumber: "+1234567890", Content: "Message 1", Sent: false},
		{ID: 2, PhoneNumber: "+1234567891", Content: "Message 2", Sent: false},
	}

	mockMessageRepo.On("GetUnsentMessages", mock.Anything, 2).Return(testMessages, nil)
	mockSMSProvider.On("SendMessage", mock.Anything, "+1234567890", "Message 1").
		Return(&domain.SMSDeliveryResponse{Message: "Accepted", MessageID: "msg_123"}, nil)
	mockSMSProvider.On("SendMessage", mock.Anything, "+1234567891", "Message 2").
		Return(nil, assert.AnError)
	mockMessageRepo.On("MarkAsSent", mock.Anything, 1).Return(nil)
	mockCacheRepo.On("SetDeliveryCache", mock.Anything, 1, mock.AnythingOfType("*domain.CachedDelivery")).Return(nil)

	service := NewMessageService(mockMessageRepo, mockCacheRepo, mockSMSProvider)
	err := service.ProcessMessages(context.Background())

	assert.Error(t, err)
	mockMessageRepo.AssertCalled(t, "MarkAsSent", mock.Anything, 1)
	mockMessageRepo.AssertNotCalled(t, "MarkAsSent", mock.Anything, 2)
}

func TestMessageService_ProcessMessages_SingleMessage(t *testing.T) {
	mockMessageRepo := new(MockMessageRepository)
	mockCacheRepo := new(MockCacheRepository)
	mockSMSProvider := new(MockSMSProvider)

	testMessages := []*domain.Message{
		{ID: 1, PhoneNumber: "+905551111111", Content: "Single message", Sent: false},
	}

	mockMessageRepo.On("GetUnsentMessages", mock.Anything, 2).Return(testMessages, nil)
	mockSMSProvider.On("SendMessage", mock.Anything, "+905551111111", "Single message").
		Return(&domain.SMSDeliveryResponse{Message: "Accepted", MessageID: "msg_789"}, nil)
	mockMessageRepo.On("MarkAsSent", mock.Anything, 1).Return(nil)
	mockCacheRepo.On("SetDeliveryCache", mock.Anything, 1, mock.AnythingOfType("*domain.CachedDelivery")).Return(nil)

	service := NewMessageService(mockMessageRepo, mockCacheRepo, mockSMSProvider)
	err := service.ProcessMessages(context.Background())

	assert.NoError(t, err)
	mockSMSProvider.AssertNumberOfCalls(t, "SendMessage", 1)
}

func TestMessageService_ProcessMessages_RedisFailureDoesNotBlockSending(t *testing.T) {
	mockMessageRepo := new(MockMessageRepository)
	mockCacheRepo := new(MockCacheRepository)
	mockSMSProvider := new(MockSMSProvider)

	testMessages := []*domain.Message{
		{ID: 1, PhoneNumber: "+1234567890", Content: "Test", Sent: false},
	}

	mockMessageRepo.On("GetUnsentMessages", mock.Anything, 2).Return(testMessages, nil)
	mockSMSProvider.On("SendMessage", mock.Anything, "+1234567890", "Test").
		Return(&domain.SMSDeliveryResponse{Message: "Accepted", MessageID: "msg_111"}, nil)
	mockMessageRepo.On("MarkAsSent", mock.Anything, 1).Return(nil)
	mockCacheRepo.On("SetDeliveryCache", mock.Anything, 1, mock.AnythingOfType("*domain.CachedDelivery")).
		Return(assert.AnError)

	service := NewMessageService(mockMessageRepo, mockCacheRepo, mockSMSProvider)
	err := service.ProcessMessages(context.Background())

	assert.NoError(t, err)
	mockMessageRepo.AssertCalled(t, "MarkAsSent", mock.Anything, 1)
}

func TestMessageService_GetSentMessagesWithCache_RedisFailureFallsBack(t *testing.T) {
	mockMessageRepo := new(MockMessageRepository)
	mockCacheRepo := new(MockCacheRepository)
	mockSMSProvider := new(MockSMSProvider)

	sentMessages := []*domain.Message{
		{ID: 1, PhoneNumber: "+1234567890", Content: "Test", Sent: true},
	}

	mockMessageRepo.On("GetSentMessages", mock.Anything).Return(sentMessages, nil)
	mockCacheRepo.On("GetMultipleDeliveryCache", mock.Anything, []int{1}).
		Return(map[int]*domain.CachedDelivery{}, assert.AnError)

	service := NewMessageService(mockMessageRepo, mockCacheRepo, mockSMSProvider)
	result, err := service.GetSentMessagesWithCache(context.Background())

	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Nil(t, result[0].MessageID)
}
