package checker

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/BigKAA/uniproxy/internal/config"
)

// RedisChecker performs Redis PING health checks.
type RedisChecker struct {
	conn   config.Connection
	client *redis.Client
}

// NewRedisChecker creates a Redis checker for the given connection.
func NewRedisChecker(conn config.Connection) *RedisChecker {
	client := redis.NewClient(&redis.Options{
		Addr:        fmt.Sprintf("%s:%s", conn.Host, conn.Port),
		DialTimeout: 5 * time.Second,
		ReadTimeout: 5 * time.Second,
	})
	return &RedisChecker{
		conn:   conn,
		client: client,
	}
}

// Check performs a Redis PING and returns the result.
func (c *RedisChecker) Check(ctx context.Context) Result {
	start := time.Now()
	err := c.client.Ping(ctx).Err()
	elapsed := time.Since(start)

	return Result{
		Name:      c.conn.Name,
		Type:      c.conn.Type,
		Host:      c.conn.Host,
		Port:      c.conn.Port,
		Healthy:   err == nil,
		LastCheck: time.Now(),
		LatencyMs: float64(elapsed.Microseconds()) / 1000.0,
	}
}
