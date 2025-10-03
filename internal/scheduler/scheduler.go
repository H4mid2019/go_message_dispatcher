// Package scheduler provides background task scheduling using goroutines.
// It implements the automatic message processing system with start/stop control.
package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-message-dispatcher/internal/domain"
	"go.uber.org/zap"
)

// MessageScheduler manages the automatic processing of messages on a timer
type MessageScheduler struct {
	messageService domain.MessageService
	logger         *zap.Logger
	interval       time.Duration

	// Control mechanisms for graceful start/stop
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	running    bool
	runningMux sync.RWMutex
}

// NewMessageScheduler creates a new message scheduler with the specified processing interval
func NewMessageScheduler(messageService domain.MessageService, logger *zap.Logger, interval time.Duration) *MessageScheduler {
	return &MessageScheduler{
		messageService: messageService,
		logger:         logger,
		interval:       interval,
	}
}

// Start begins the automatic message processing
// Creates a background goroutine that processes messages at regular intervals
func (s *MessageScheduler) Start() error {
	s.runningMux.Lock()
	defer s.runningMux.Unlock()

	if s.running {
		return fmt.Errorf("scheduler is already running")
	}

	// Create cancellable context for clean shutdown
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.running = true

	// Start the background processing goroutine
	s.wg.Add(1)
	go s.processMessages()

	s.logger.Info("Message scheduler started",
		zap.Duration("interval", s.interval))

	return nil
}

// Stop halts the automatic message processing
// Ensures graceful shutdown by waiting for in-flight operations to complete
func (s *MessageScheduler) Stop() error {
	s.runningMux.Lock()
	defer s.runningMux.Unlock()

	if !s.running {
		return fmt.Errorf("scheduler is not running")
	}

	// Signal shutdown and wait for background goroutine to finish
	s.cancel()
	s.wg.Wait()
	s.running = false

	s.logger.Info("Message scheduler stopped")
	return nil
}

// IsRunning returns the current processing status
func (s *MessageScheduler) IsRunning() bool {
	s.runningMux.RLock()
	defer s.runningMux.RUnlock()
	return s.running
}

// processMessages is the main background processing loop
// Runs until cancelled and processes messages at regular intervals
func (s *MessageScheduler) processMessages() {
	defer s.wg.Done()

	// Create ticker for regular processing intervals
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	s.logger.Info("Background message processing started")

	for {
		select {
		case <-s.ctx.Done():
			s.logger.Info("Background message processing stopped")
			return
		case <-ticker.C:
			s.processBatch()
		}
	}
}

// processBatch handles a single batch of message processing
// Isolated from the main loop to enable better error handling and testing
func (s *MessageScheduler) processBatch() {
	processingCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	start := time.Now()

	err := s.messageService.ProcessMessages(processingCtx)
	if err != nil {
		s.logger.Error("Failed to process message batch",
			zap.Error(err),
			zap.Duration("duration", time.Since(start)))
		return
	}

	s.logger.Debug("Successfully processed message batch",
		zap.Duration("duration", time.Since(start)))
}
