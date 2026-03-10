package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimiter_AllowUnderLimit(t *testing.T) {
	rl := newRateLimiter(1*time.Minute, 5)
	for i := 0; i < 5; i++ {
		if !rl.Allow("10.0.0.1") {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}
}

func TestRateLimiter_BlockOverLimit(t *testing.T) {
	rl := newRateLimiter(1*time.Minute, 5)
	for i := 0; i < 5; i++ {
		rl.Allow("10.0.0.1")
	}
	if rl.Allow("10.0.0.1") {
		t.Fatal("6th request should be blocked")
	}
}

func TestRateLimiter_WindowExpiry(t *testing.T) {
	rl := newRateLimiter(50*time.Millisecond, 2)
	rl.Allow("10.0.0.1")
	rl.Allow("10.0.0.1")
	if rl.Allow("10.0.0.1") {
		t.Fatal("3rd request should be blocked")
	}

	time.Sleep(60 * time.Millisecond)

	if !rl.Allow("10.0.0.1") {
		t.Fatal("request after window expiry should be allowed")
	}
}

func TestRateLimiter_DifferentIPs(t *testing.T) {
	rl := newRateLimiter(1*time.Minute, 2)
	rl.Allow("10.0.0.1")
	rl.Allow("10.0.0.1")
	if rl.Allow("10.0.0.1") {
		t.Fatal("3rd request from same IP should be blocked")
	}
	if !rl.Allow("10.0.0.2") {
		t.Fatal("request from different IP should be allowed")
	}
}

func TestRateLimiter_CleanupThrottle(t *testing.T) {
	rl := newRateLimiter(50*time.Millisecond, 100)
	// Add entries for multiple IPs.
	for i := 0; i < 10; i++ {
		rl.Allow("10.0.0.1")
	}

	// Wait for window to expire.
	time.Sleep(60 * time.Millisecond)

	// Next Allow call should trigger cleanup.
	rl.Allow("10.0.0.2")

	rl.mu.Lock()
	_, exists := rl.attempts["10.0.0.1"]
	rl.mu.Unlock()

	if exists {
		t.Fatal("stale entry should have been cleaned up")
	}
}

func TestClientIP_XForwardedFor(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Forwarded-For", "203.0.113.50, 70.41.3.18, 150.172.238.178")
	got := clientIP(r)
	if got != "203.0.113.50" {
		t.Errorf("clientIP = %q, want %q", got, "203.0.113.50")
	}
}

func TestClientIP_XRealIP(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Real-IP", "203.0.113.50")
	got := clientIP(r)
	if got != "203.0.113.50" {
		t.Errorf("clientIP = %q, want %q", got, "203.0.113.50")
	}
}

func TestClientIP_RemoteAddr(t *testing.T) {
	r := &http.Request{RemoteAddr: "192.168.1.1:12345"}
	got := clientIP(r)
	if got != "192.168.1.1" {
		t.Errorf("clientIP = %q, want %q", got, "192.168.1.1")
	}
}
