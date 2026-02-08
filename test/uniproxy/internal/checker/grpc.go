package checker

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/BigKAA/uniproxy/internal/config"
)

// GRPCChecker performs TCP dial connectivity checks for gRPC endpoints.
type GRPCChecker struct {
	conn config.Connection
	addr string
}

// NewGRPCChecker creates a gRPC (TCP dial) checker for the given connection.
func NewGRPCChecker(conn config.Connection) *GRPCChecker {
	return &GRPCChecker{
		conn: conn,
		addr: fmt.Sprintf("%s:%s", conn.Host, conn.Port),
	}
}

// Check performs a TCP dial and returns the result.
func (c *GRPCChecker) Check(ctx context.Context) Result {
	start := time.Now()

	d := net.Dialer{Timeout: 5 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", c.addr)
	elapsed := time.Since(start)

	if conn != nil {
		conn.Close()
	}

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
