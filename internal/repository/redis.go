package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/go-message-dispatcher/internal/domain"
	"github.com/redis/go-redis/v9"
)

// RedisCacheRepository implements CacheRepository using Redis
type RedisCacheRepository struct {
	client *redis.Client
	ttl    time.Duration
}

// NewRedisCacheRepository creates a new Redis cache repository
func NewRedisCacheRepository(client *redis.Client) *RedisCacheRepository {
	return &RedisCacheRepository{
		client: client,
		ttl:    24 * time.Hour, // Cache delivery data for 24 hours
	}
}

// SetDeliveryCache stores delivery metadata in Redis with expiration
// Uses message ID as key for fast lookups during API responses
func (r *RedisCacheRepository) SetDeliveryCache(ctx context.Context, messageID int, delivery *domain.CachedDelivery) error {
	key := r.buildCacheKey(messageID)

	data, err := json.Marshal(delivery)
	if err != nil {
		return fmt.Errorf("failed to marshal delivery data: %w", err)
	}

	err = r.client.Set(ctx, key, data, r.ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to cache delivery data: %w", err)
	}

	return nil
}

// GetDeliveryCache retrieves cached delivery metadata for a single message
func (r *RedisCacheRepository) GetDeliveryCache(ctx context.Context, messageID int) (*domain.CachedDelivery, error) {
	key := r.buildCacheKey(messageID)

	data, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // Cache miss is not an error
		}
		return nil, fmt.Errorf("failed to get cached delivery data: %w", err)
	}

	var delivery domain.CachedDelivery
	err = json.Unmarshal([]byte(data), &delivery)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal delivery data: %w", err)
	}

	return &delivery, nil
}

// GetMultipleDeliveryCache efficiently retrieves cached delivery metadata for multiple messages
// Uses pipeline for better performance when loading many cached entries
func (r *RedisCacheRepository) GetMultipleDeliveryCache(ctx context.Context, messageIDs []int) (map[int]*domain.CachedDelivery, error) {
	if len(messageIDs) == 0 {
		return make(map[int]*domain.CachedDelivery), nil
	}

	// Build pipeline for efficient batch retrieval
	pipe := r.client.Pipeline()
	commands := make(map[int]*redis.StringCmd)

	for _, messageID := range messageIDs {
		key := r.buildCacheKey(messageID)
		commands[messageID] = pipe.Get(ctx, key)
	}

	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("failed to execute cache pipeline: %w", err)
	}

	// Process results
	result := make(map[int]*domain.CachedDelivery)
	for messageID, cmd := range commands {
		data, err := cmd.Result()
		if err != nil {
			if err == redis.Nil {
				continue // Skip cache misses
			}
			return nil, fmt.Errorf("failed to get cached data for message %d: %w", messageID, err)
		}

		var delivery domain.CachedDelivery
		err = json.Unmarshal([]byte(data), &delivery)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal delivery data for message %d: %w", messageID, err)
		}

		result[messageID] = &delivery
	}

	return result, nil
}

// buildCacheKey creates a consistent cache key for message delivery data
func (r *RedisCacheRepository) buildCacheKey(messageID int) string {
	return fmt.Sprintf("delivery:%s", strconv.Itoa(messageID))
}

// CheckConnection verifies the Redis connection is healthy
func (r *RedisCacheRepository) CheckConnection(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}
