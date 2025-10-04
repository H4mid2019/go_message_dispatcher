// Package lock provides distributed locking mechanisms for multi-instance coordination.
package lock

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

var (
	// ErrLockNotAcquired is returned when a lock cannot be acquired
	ErrLockNotAcquired = errors.New("lock not acquired")
	// ErrLockNotHeld is returned when trying to release a lock that is not held
	ErrLockNotHeld = errors.New("lock not held")
)

// DistributedLock defines the interface for distributed locking
type DistributedLock interface {
	// Acquire attempts to acquire the lock
	Acquire(ctx context.Context) error
	// Release releases the lock
	Release(ctx context.Context) error
	// Extend extends the lock TTL
	Extend(ctx context.Context) error
	// IsHeld checks if the lock is currently held by this instance
	IsHeld() bool
}

// RedisLock implements distributed locking using Redis
type RedisLock struct {
	client   *redis.Client
	key      string
	value    string
	ttl      time.Duration
	logger   *zap.Logger
	acquired bool
}

// NewRedisLock creates a new Redis-based distributed lock
func NewRedisLock(client *redis.Client, key string, ttl time.Duration, logger *zap.Logger) *RedisLock {
	return &RedisLock{
		client:   client,
		key:      key,
		value:    generateLockValue(),
		ttl:      ttl,
		logger:   logger,
		acquired: false,
	}
}

// Acquire attempts to acquire the lock using SET NX (set if not exists)
func (l *RedisLock) Acquire(ctx context.Context) error {
	// Try to set the key with NX (only if not exists) and EX (expiration)
	success, err := l.client.SetNX(ctx, l.key, l.value, l.ttl).Result()
	if err != nil {
		l.logger.Error("Failed to acquire lock", zap.String("key", l.key), zap.Error(err))
		return err
	}

	if !success {
		l.logger.Debug("Lock already held by another instance", zap.String("key", l.key))
		return ErrLockNotAcquired
	}

	l.acquired = true
	l.logger.Info("Lock acquired", zap.String("key", l.key), zap.Duration("ttl", l.ttl))
	return nil
}

// Release releases the lock using Lua script to ensure atomicity
func (l *RedisLock) Release(ctx context.Context) error {
	if !l.acquired {
		return ErrLockNotHeld
	}

	// Use Lua script to atomically check and delete
	// This ensures we only delete the lock if we own it
	script := `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("del", KEYS[1])
		else
			return 0
		end
	`

	result, err := l.client.Eval(ctx, script, []string{l.key}, l.value).Result()
	if err != nil {
		l.logger.Error("Failed to release lock", zap.String("key", l.key), zap.Error(err))
		return err
	}

	if result == int64(0) {
		l.logger.Warn("Lock was not held by this instance", zap.String("key", l.key))
		l.acquired = false
		return ErrLockNotHeld
	}

	l.acquired = false
	l.logger.Info("Lock released", zap.String("key", l.key))
	return nil
}

// Extend extends the lock TTL if we still hold it
func (l *RedisLock) Extend(ctx context.Context) error {
	if !l.acquired {
		return ErrLockNotHeld
	}

	// Use Lua script to atomically check owner and extend TTL
	script := `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("expire", KEYS[1], ARGV[2])
		else
			return 0
		end
	`

	result, err := l.client.Eval(ctx, script, []string{l.key}, l.value, int(l.ttl.Seconds())).Result()
	if err != nil {
		l.logger.Error("Failed to extend lock", zap.String("key", l.key), zap.Error(err))
		return err
	}

	if result == int64(0) {
		l.logger.Warn("Lock extension failed - lock no longer held", zap.String("key", l.key))
		l.acquired = false
		return ErrLockNotHeld
	}

	l.logger.Debug("Lock extended", zap.String("key", l.key), zap.Duration("ttl", l.ttl))
	return nil
}

// IsHeld checks if the lock is currently held by this instance
func (l *RedisLock) IsHeld() bool {
	return l.acquired
}

// generateLockValue generates a unique random value for the lock
// This ensures only the instance that acquired the lock can release it
func generateLockValue() string {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		// Fallback to timestamp if crypto/rand fails
		return time.Now().Format(time.RFC3339Nano)
	}
	return base64.URLEncoding.EncodeToString(b)
}
