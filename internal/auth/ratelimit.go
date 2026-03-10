package auth

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// rateLimiter tracks login attempts per IP with a sliding window.
type rateLimiter struct {
	mu          sync.Mutex
	attempts    map[string][]time.Time
	window      time.Duration
	max         int
	lastCleanup time.Time
}

func newRateLimiter(window time.Duration, max int) *rateLimiter {
	return &rateLimiter{
		attempts:    make(map[string][]time.Time),
		window:      window,
		max:         max,
		lastCleanup: time.Now(),
	}
}

// Allow returns true if the IP is under the rate limit.
func (rl *rateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	// Throttled cleanup of stale entries.
	if now.Sub(rl.lastCleanup) >= rl.window {
		for k, times := range rl.attempts {
			filtered := filterAfter(times, cutoff)
			if len(filtered) == 0 {
				delete(rl.attempts, k)
			} else {
				rl.attempts[k] = filtered
			}
		}
		rl.lastCleanup = now
	}

	// Filter current IP's attempts within the window.
	recent := filterAfter(rl.attempts[ip], cutoff)

	if len(recent) >= rl.max {
		rl.attempts[ip] = recent
		return false
	}

	rl.attempts[ip] = append(recent, now)
	return true
}

func filterAfter(times []time.Time, cutoff time.Time) []time.Time {
	var result []time.Time
	for _, t := range times {
		if t.After(cutoff) {
			result = append(result, t)
		}
	}
	return result
}

// clientIP extracts the client IP from the request, supporting reverse proxies.
func clientIP(r *http.Request) string {
	// X-Forwarded-For: client, proxy1, proxy2 — take first (leftmost).
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.SplitN(xff, ",", 2)
		ip := strings.TrimSpace(parts[0])
		if ip != "" {
			return stripPort(ip)
		}
	}

	// X-Real-IP fallback.
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return stripPort(strings.TrimSpace(xri))
	}

	// RemoteAddr fallback.
	return stripPort(r.RemoteAddr)
}

func stripPort(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return host
}
