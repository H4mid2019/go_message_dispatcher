package lock

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestRedisLock_AcquireAndRelease tests basic lock acquisition and release
func TestRedisLock_AcquireAndRelease(t *testing.T) {
	// Setup mock Redis client
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer func() { _ = client.Close() }()

	logger := zap.NewNop()
	ctx := context.Background()

	// Skip test if Redis is not available
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available, skipping test")
	}

	// Clean up any existing locks
	client.Del(ctx, "test:lock:acquire_release")

	lock := NewRedisLock(client, "test:lock:acquire_release", 5*time.Second, logger)

	// Test acquire
	err := lock.Acquire(ctx)
	require.NoError(t, err)
	assert.True(t, lock.IsHeld())

	// Test release
	err = lock.Release(ctx)
	require.NoError(t, err)
	assert.False(t, lock.IsHeld())

	// Clean up
	client.Del(ctx, "test:lock:acquire_release")
}

// TestRedisLock_ConcurrentAcquire tests that only one instance can acquire the lock
func TestRedisLock_ConcurrentAcquire(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer func() { _ = client.Close() }()

	logger := zap.NewNop()
	ctx := context.Background()

	if err := client.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available, skipping test")
	}

	client.Del(ctx, "test:lock:concurrent")

	lock1 := NewRedisLock(client, "test:lock:concurrent", 5*time.Second, logger)
	lock2 := NewRedisLock(client, "test:lock:concurrent", 5*time.Second, logger)

	// First lock should acquire
	err := lock1.Acquire(ctx)
	require.NoError(t, err)
	assert.True(t, lock1.IsHeld())

	// Second lock should fail to acquire
	err = lock2.Acquire(ctx)
	assert.ErrorIs(t, err, ErrLockNotAcquired)
	assert.False(t, lock2.IsHeld())

	// Release first lock
	err = lock1.Release(ctx)
	require.NoError(t, err)

	// Now second lock should be able to acquire
	err = lock2.Acquire(ctx)
	require.NoError(t, err)
	assert.True(t, lock2.IsHeld())

	_ = lock2.Release(ctx)
	client.Del(ctx, "test:lock:concurrent")
}

// TestRedisLock_Extend tests lock TTL extension
func TestRedisLock_Extend(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer func() { _ = client.Close() }()

	logger := zap.NewNop()
	ctx := context.Background()

	if err := client.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available, skipping test")
	}

	client.Del(ctx, "test:lock:extend")

	lock := NewRedisLock(client, "test:lock:extend", 2*time.Second, logger)

	// Acquire lock
	err := lock.Acquire(ctx)
	require.NoError(t, err)

	// Extend lock
	err = lock.Extend(ctx)
	require.NoError(t, err)

	// Check TTL is updated
	ttl, err := client.TTL(ctx, "test:lock:extend").Result()
	require.NoError(t, err)
	assert.True(t, ttl > time.Second, "TTL should be greater than 1 second after extension")

	_ = lock.Release(ctx)
	client.Del(ctx, "test:lock:extend")
}

// TestRedisLock_AutoExpiration tests that lock expires automatically
func TestRedisLock_AutoExpiration(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer func() { _ = client.Close() }()

	logger := zap.NewNop()
	ctx := context.Background()

	if err := client.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available, skipping test")
	}

	client.Del(ctx, "test:lock:expiration")

	lock1 := NewRedisLock(client, "test:lock:expiration", 1*time.Second, logger)
	lock2 := NewRedisLock(client, "test:lock:expiration", 1*time.Second, logger)

	// Acquire with lock1
	err := lock1.Acquire(ctx)
	require.NoError(t, err)

	// lock2 cannot acquire
	err = lock2.Acquire(ctx)
	assert.ErrorIs(t, err, ErrLockNotAcquired)

	// Wait for expiration
	time.Sleep(2 * time.Second)

	// Now lock2 should be able to acquire
	err = lock2.Acquire(ctx)
	require.NoError(t, err)
	assert.True(t, lock2.IsHeld())

	_ = lock2.Release(ctx)
	client.Del(ctx, "test:lock:expiration")
}

// TestRedisLock_ReleaseNotHeld tests releasing a lock not held
func TestRedisLock_ReleaseNotHeld(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer func() { _ = client.Close() }()

	logger := zap.NewNop()
	ctx := context.Background()

	if err := client.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available, skipping test")
	}

	client.Del(ctx, "test:lock:not_held")

	lock := NewRedisLock(client, "test:lock:not_held", 5*time.Second, logger)

	// Try to release without acquiring
	err := lock.Release(ctx)
	assert.ErrorIs(t, err, ErrLockNotHeld)

	// Clean up
	client.Del(ctx, "test:lock:not_held")
}
