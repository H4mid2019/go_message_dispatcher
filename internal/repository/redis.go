// Package repository provides data access implementations.
package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/go-message-dispatcher/internal/domain"
)

type RedisCacheRepository struct {
	client *redis.Client
	ttl    time.Duration
}

func NewRedisCacheRepository(client *redis.Client) *RedisCacheRepository {
	const defaultTTL = 24 * time.Hour
	return &RedisCacheRepository{
		client: client,
		ttl:    defaultTTL,
	}
}

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

func (r *RedisCacheRepository) GetDeliveryCache(ctx context.Context, messageID int) (*domain.CachedDelivery, error) {
	key := r.buildCacheKey(messageID)

	data, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
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
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, fmt.Errorf("failed to execute cache pipeline: %w", err)
	}

	// Process results
	result := make(map[int]*domain.CachedDelivery)
	for messageID, cmd := range commands {
		data, err := cmd.Result()
		if err != nil {
			if errors.Is(err, redis.Nil) {
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
