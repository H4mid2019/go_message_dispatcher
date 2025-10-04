package scheduler

import (
	"context"
	"errors"
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

	s.logger.Info("Scheduler started", zap.Duration("interval", s.interval))
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
		s.logger.Info("Waiting for batch to complete")
	}
	s.processingMux.RUnlock()

	if s.lockEnabled && s.distributedLock != nil && s.distributedLock.IsHeld() {
		if err := s.distributedLock.Release(context.Background()); err != nil {
			s.logger.Error("Failed to release lock on shutdown", zap.Error(err))
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

	if s.lockEnabled && s.distributedLock != nil {
		s.lockExtendTicker = time.NewTicker(s.interval / 2)
		defer s.lockExtendTicker.Stop()
	}

	s.logger.Info("Processing started", zap.Bool("distributed_locking", s.lockEnabled))

	s.processBatch()

	for {
		select {
		case <-s.ctx.Done():
			s.logger.Info("Processing stopped")
			return
		case <-ticker.C:
			s.processBatch()
		case <-s.getExtendChannel():
			if s.lockEnabled && s.distributedLock != nil && s.distributedLock.IsHeld() {
				if err := s.distributedLock.Extend(context.Background()); err != nil {
					s.logger.Warn("Failed to extend lock", zap.Error(err))
				}
			}
		}
	}
}

func (s *MessageScheduler) getExtendChannel() <-chan time.Time {
	if s.lockExtendTicker != nil {
		return s.lockExtendTicker.C
	}
	return nil
}

func (s *MessageScheduler) processBatch() {
	if s.lockEnabled && s.distributedLock != nil {
		lockCtx, lockCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer lockCancel()

		if err := s.distributedLock.Acquire(lockCtx); err != nil {
			if errors.Is(err, lock.ErrLockNotAcquired) {
				s.logger.Debug("Another instance processing, skipping")
			} else {
				s.logger.Warn("Failed to acquire lock", zap.Error(err))
			}
			return
		}
		defer func() {
			if err := s.distributedLock.Release(context.Background()); err != nil {
				s.logger.Error("Failed to release lock", zap.Error(err))
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
		s.logger.Error("Batch processing failed", zap.Error(err), zap.Duration("duration", duration))
		return
	}

	s.logger.Debug("Batch processed", zap.Duration("duration", duration))
}
