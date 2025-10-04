// Package scheduler provides background task scheduling and message processing automation.
package scheduler

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/go-message-dispatcher/internal/domain"
	"github.com/go-message-dispatcher/internal/lock"
)

type MessageScheduler struct {
	messageService   domain.MessageService
	logger           *zap.Logger
	interval         time.Duration
	ctx              context.Context
	cancel           context.CancelFunc
	wg               sync.WaitGroup
	running          bool
	runningMux       sync.RWMutex
	processing       bool
	processingMux    sync.RWMutex
	distributedLock  lock.DistributedLock
	lockEnabled      bool
	lockExtendTicker *time.Ticker
}

func NewMessageScheduler(messageService domain.MessageService, logger *zap.Logger, interval time.Duration) *MessageScheduler {
	return &MessageScheduler{
		messageService: messageService,
		logger:         logger,
		interval:       interval,
		lockEnabled:    false,
	}
}

// NewMessageSchedulerWithLock creates a new message scheduler with distributed locking support
func NewMessageSchedulerWithLock(messageService domain.MessageService, logger *zap.Logger, interval time.Duration, distributedLock lock.DistributedLock) *MessageScheduler {
	return &MessageScheduler{
		messageService:  messageService,
		logger:          logger,
		interval:        interval,
		distributedLock: distributedLock,
		lockEnabled:     true,
	}
}

func (s *MessageScheduler) Start() error {
	s.runningMux.Lock()
	defer s.runningMux.Unlock()

	if s.running {
		return nil
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
	if !s.running {
		s.runningMux.Unlock()
		return nil
	}

	s.logger.Info("Stopping message scheduler")
	s.cancel()
	s.running = false
	s.runningMux.Unlock()

	// Stop lock extension ticker
	if s.lockExtendTicker != nil {
		s.lockExtendTicker.Stop()
	}

	s.wg.Wait()

	s.processingMux.RLock()
	if s.processing {
		s.logger.Info("Waiting for current batch to complete")
	}
	s.processingMux.RUnlock()

	// Release distributed lock if held
	if s.lockEnabled && s.distributedLock != nil && s.distributedLock.IsHeld() {
		if err := s.distributedLock.Release(context.Background()); err != nil {
			s.logger.Error("Failed to release distributed lock on shutdown", zap.Error(err))
		}
	}

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

	// Start lock extension ticker if distributed locking is enabled
	if s.lockEnabled && s.distributedLock != nil {
		// Extend lock every interval/2 to ensure it doesn't expire
		s.lockExtendTicker = time.NewTicker(s.interval / 2)
		defer s.lockExtendTicker.Stop()
	}

	s.logger.Info("Background message processing started", zap.Bool("distributed_locking", s.lockEnabled))

	s.processBatch()

	for {
		select {
		case <-s.ctx.Done():
			s.logger.Info("Background message processing stopped")
			return
		case <-ticker.C:
			s.processBatch()
		case <-s.getExtendChannel():
			// Extend lock TTL to prevent expiration during long processing
			if s.lockEnabled && s.distributedLock != nil && s.distributedLock.IsHeld() {
				if err := s.distributedLock.Extend(context.Background()); err != nil {
					s.logger.Warn("Failed to extend distributed lock", zap.Error(err))
				}
			}
		}
	}
}

// getExtendChannel returns the lock extension ticker channel or a nil channel if not enabled
func (s *MessageScheduler) getExtendChannel() <-chan time.Time {
	if s.lockExtendTicker != nil {
		return s.lockExtendTicker.C
	}
	// Return a channel that never sends to avoid busy-waiting
	return nil
}

func (s *MessageScheduler) processBatch() {
	// Try to acquire distributed lock if enabled
	if s.lockEnabled && s.distributedLock != nil {
		lockCtx, lockCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer lockCancel()

		if err := s.distributedLock.Acquire(lockCtx); err != nil {
			if err == lock.ErrLockNotAcquired {
				s.logger.Debug("Another instance is processing messages, skipping this cycle")
			} else {
				s.logger.Warn("Failed to acquire distributed lock", zap.Error(err))
			}
			return
		}
		defer func() {
			if err := s.distributedLock.Release(context.Background()); err != nil {
				s.logger.Error("Failed to release distributed lock", zap.Error(err))
			}
		}()
	}

	s.processingMux.Lock()
	s.processing = true
	s.processingMux.Unlock()

	defer func() {
		s.processingMux.Lock()
		s.processing = false
		s.processingMux.Unlock()
	}()

	const processingTimeout = 30 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), processingTimeout)
	defer cancel()

	start := time.Now()
	err := s.messageService.ProcessMessages(ctx)
	duration := time.Since(start)

	if err != nil {
		s.logger.Error("Failed to process message batch", zap.Error(err), zap.Duration("duration", duration))
		return
	}

	s.logger.Debug("Message batch processed", zap.Duration("duration", duration))
}
