package ratelimiter

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RateLimiter struct {
	client *redis.Client
}

func New(client *redis.Client) *RateLimiter {
	return &RateLimiter{client: client}
}

func (rl *RateLimiter) Key(userID int64) string {
	today := time.Now().UTC().Format("2006-01-02")
	return fmt.Sprintf("usage:%d:%s", userID, today)
}

func (rl *RateLimiter) Increment(ctx context.Context, userID int64) (int64, error) {
	key := rl.Key(userID)

	count, err := rl.client.Incr(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to increment usage counter: %w", err)
	}

	if count == 1 {
		now := time.Now().UTC()
		midnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
		ttl := midnight.Sub(now)
		rl.client.Expire(ctx, key, ttl)
	}

	return count, nil
}

func (rl *RateLimiter) GetUsage(ctx context.Context, userID int64) (int64, error) {
	key := rl.Key(userID)

	count, err := rl.client.Get(ctx, key).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("failed to get usage counter: %w", err)
	}

	return count, nil
}
