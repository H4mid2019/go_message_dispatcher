// Package scheduler provides background task scheduling and message processing automation.
package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/go-message-dispatcher/internal/domain"
)

type MessageScheduler struct {
	messageService domain.MessageService
	logger         *zap.Logger
	interval       time.Duration
	ctx            context.Context
	cancel         context.CancelFunc
	wg             sync.WaitGroup
	running        bool
	runningMux     sync.RWMutex
}

func NewMessageScheduler(messageService domain.MessageService, logger *zap.Logger, interval time.Duration) *MessageScheduler {
	return &MessageScheduler{
		messageService: messageService,
		logger:         logger,
		interval:       interval,
	}
}

func (s *MessageScheduler) Start() error {
	s.runningMux.Lock()
	defer s.runningMux.Unlock()

	if s.running {
		return fmt.Errorf("scheduler is already running")
	}

	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.running = true

	s.wg.Add(1)
	go s.processMessages()

	s.logger.Info("Message scheduler started", zap.Duration("interval", s.interval))
	return nil
}

func (s *MessageScheduler) Stop() error {
	s.runningMux.Lock()
	defer s.runningMux.Unlock()

	if !s.running {
		return fmt.Errorf("scheduler is not running")
	}

	s.cancel()
	s.wg.Wait()
	s.running = false

	s.logger.Info("Message scheduler stopped")
	return nil
}

func (s *MessageScheduler) IsRunning() bool {
	s.runningMux.RLock()
	defer s.runningMux.RUnlock()
	return s.running
}

func (s *MessageScheduler) processMessages() {
	defer s.wg.Done()

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

func (s *MessageScheduler) processBatch() {
	const processingTimeout = 30 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), processingTimeout)
	defer cancel()

	start := time.Now()
	err := s.messageService.ProcessMessages(ctx)
	if err != nil {
		s.logger.Error("Failed to process message batch", zap.Error(err), zap.Duration("duration", time.Since(start)))
		return
	}

	s.logger.Debug("Successfully processed message batch", zap.Duration("duration", time.Since(start)))
}
