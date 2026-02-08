package checker

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/BigKAA/uniproxy/internal/config"
)

// PostgresChecker performs PostgreSQL connectivity checks.
type PostgresChecker struct {
	conn config.Connection
	dsn  string
}

// NewPostgresChecker creates a PostgreSQL checker for the given connection.
func NewPostgresChecker(conn config.Connection) *PostgresChecker {
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		conn.Username, conn.Password, conn.Host, conn.Port, conn.Database)
	return &PostgresChecker{
		conn: conn,
		dsn:  dsn,
	}
}

// Check connects to PostgreSQL, runs SELECT 1, and returns the result.
func (c *PostgresChecker) Check(ctx context.Context) Result {
	start := time.Now()

	db, err := pgx.Connect(ctx, c.dsn)
	if err != nil {
		return c.result(false, time.Since(start))
	}
	defer db.Close(ctx)

	var n int
	err = db.QueryRow(ctx, "SELECT 1").Scan(&n)
	elapsed := time.Since(start)

	return c.result(err == nil, elapsed)
}

func (c *PostgresChecker) result(healthy bool, elapsed time.Duration) Result {
	return Result{
		Name:      c.conn.Name,
		Type:      c.conn.Type,
		Host:      c.conn.Host,
		Port:      c.conn.Port,
		Healthy:   healthy,
		LastCheck: time.Now(),
		LatencyMs: float64(elapsed.Microseconds()) / 1000.0,
	}
}
