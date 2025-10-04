package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"

	"github.com/go-message-dispatcher/internal/domain"
)

type MockMessageService struct {
	mock.Mock
}

func (m *MockMessageService) ProcessMessages(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockMessageService) GetSentMessagesWithCache(ctx context.Context) ([]*domain.SentMessageResponse, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*domain.SentMessageResponse), args.Error(1)
}

func TestMessageScheduler_StartAndStop(t *testing.T) {
	mockService := new(MockMessageService)
	logger, _ := zap.NewDevelopment()

	mockService.On("ProcessMessages", mock.Anything).Return(nil)

	scheduler := NewMessageScheduler(mockService, logger, 100*time.Millisecond)

	assert.False(t, scheduler.IsRunning())

	err := scheduler.Start()
	assert.NoError(t, err)
	assert.True(t, scheduler.IsRunning())

	err = scheduler.Start()
	assert.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	err = scheduler.Stop()
	assert.NoError(t, err)
	assert.False(t, scheduler.IsRunning())
}

func TestMessageScheduler_ProcessesImmediatelyOnStart(t *testing.T) {
	mockService := new(MockMessageService)
	logger, _ := zap.NewDevelopment()

	mockService.On("ProcessMessages", mock.Anything).Return(nil)

	scheduler := NewMessageScheduler(mockService, logger, 1*time.Second)
	err := scheduler.Start()
	assert.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	mockService.AssertCalled(t, "ProcessMessages", mock.Anything)

	_ = scheduler.Stop()
}

func TestMessageScheduler_GracefulShutdownWaitsForBatch(t *testing.T) {
	mockService := new(MockMessageService)
	logger, _ := zap.NewDevelopment()

	processedCount := 0
	mockService.On("ProcessMessages", mock.Anything).Run(func(args mock.Arguments) {
		processedCount++
		time.Sleep(150 * time.Millisecond)
	}).Return(nil)

	scheduler := NewMessageScheduler(mockService, logger, 500*time.Millisecond)
	_ = scheduler.Start()

	time.Sleep(100 * time.Millisecond)

	err := scheduler.Stop()
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, processedCount, 1)
	assert.False(t, scheduler.IsRunning())
}

func TestMessageScheduler_ProcessesAtInterval(t *testing.T) {
	mockService := new(MockMessageService)
	logger, _ := zap.NewDevelopment()

	callCount := 0
	mockService.On("ProcessMessages", mock.Anything).Run(func(args mock.Arguments) {
		callCount++
	}).Return(nil)

	scheduler := NewMessageScheduler(mockService, logger, 100*time.Millisecond)
	_ = scheduler.Start()

	time.Sleep(350 * time.Millisecond)
	_ = scheduler.Stop()

	assert.GreaterOrEqual(t, callCount, 3)
}
