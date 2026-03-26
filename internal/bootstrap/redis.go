package bootstrap

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// ConnectRedis parses the URL, creates a Redis client, and verifies connectivity.
func ConnectRedis(ctx context.Context, url string) (*redis.Client, error) {
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}

	client := redis.NewClient(opts)

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	return client, nil
}
