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
	ErrLockNotAcquired = errors.New("lock not acquired")
	ErrLockNotHeld     = errors.New("lock not held")
)

type DistributedLock interface {
	Acquire(ctx context.Context) error
	Release(ctx context.Context) error
	Extend(ctx context.Context) error
	IsHeld() bool
}

type RedisLock struct {
	client   *redis.Client
	key      string
	value    string
	ttl      time.Duration
	logger   *zap.Logger
	acquired bool
}

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

func (l *RedisLock) Acquire(ctx context.Context) error {
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

func (l *RedisLock) Release(ctx context.Context) error {
	if !l.acquired {
		return ErrLockNotHeld
	}

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

func (l *RedisLock) Extend(ctx context.Context) error {
	if !l.acquired {
		return ErrLockNotHeld
	}

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

func (l *RedisLock) IsHeld() bool {
	return l.acquired
}

func generateLockValue() string {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return time.Now().Format(time.RFC3339Nano)
	}
	return base64.URLEncoding.EncodeToString(b)
}
