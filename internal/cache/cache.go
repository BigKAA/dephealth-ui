package cache

import (
	"sync"
	"time"

	"github.com/BigKAA/dephealth-ui/internal/topology"
)

// Cache provides an in-memory TTL cache for TopologyResponse.
// It uses lazy expiration (no background goroutine).
type Cache struct {
	ttl   time.Duration
	mu    sync.RWMutex
	data  *topology.TopologyResponse
	setAt time.Time
}

// New creates a new Cache with the given TTL.
func New(ttl time.Duration) *Cache {
	return &Cache{ttl: ttl}
}

// Get returns the cached response if it exists and has not expired.
func (c *Cache) Get() (*topology.TopologyResponse, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.data == nil {
		return nil, false
	}
	if time.Since(c.setAt) > c.ttl {
		return nil, false
	}
	return c.data, true
}

// Set stores a response in the cache with the current timestamp.
func (c *Cache) Set(resp *topology.TopologyResponse) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.data = resp
	c.setAt = time.Now()
}
