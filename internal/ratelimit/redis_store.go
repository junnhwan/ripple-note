package ratelimit

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type redisCommander interface {
	Incr(ctx context.Context, key string) *redis.IntCmd
	Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd
}

type RedisStore struct {
	client redisCommander
}

func NewRedisStore(client redisCommander) *RedisStore {
	return &RedisStore{client: client}
}

func (s *RedisStore) Increment(ctx context.Context, key string, window time.Duration) (int64, error) {
	count, err := s.client.Incr(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	if count == 1 {
		if err := s.client.Expire(ctx, key, window).Err(); err != nil {
			return 0, err
		}
	}
	return count, nil
}
