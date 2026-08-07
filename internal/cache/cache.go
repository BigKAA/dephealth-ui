package cache

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/BigKAA/dephealth-ui/internal/topology"
)

// hashableResponse contains fields used for ETag computation.
// Meta is excluded because CachedAt changes on every request.
type hashableResponse struct {
	Nodes  []topology.Node      `json:"nodes"`
	Edges  []topology.Edge      `json:"edges"`
	Alerts []topology.AlertInfo `json:"alerts"`
}

// Cache provides an in-memory TTL cache for TopologyResponse.
// It uses lazy expiration (no background goroutine).
type Cache struct {
	ttl   time.Duration
	mu    sync.RWMutex
	data  *topology.TopologyResponse
	etag  string
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

// GetWithETag returns the cached response along with its ETag.
func (c *Cache) GetWithETag() (*topology.TopologyResponse, string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.data == nil {
		return nil, "", false
	}
	if time.Since(c.setAt) > c.ttl {
		return nil, "", false
	}
	return c.data, c.etag, true
}

// Set stores a response in the cache with the current timestamp and computes its ETag.
func (c *Cache) Set(resp *topology.TopologyResponse) {
	etag := computeETag(resp)

	c.mu.Lock()
	defer c.mu.Unlock()

	c.data = resp
	c.etag = etag
	c.setAt = time.Now()
}

// computeETag generates an ETag from the hashable parts of the response.
func computeETag(resp *topology.TopologyResponse) string {
	h := hashableResponse{
		Nodes:  resp.Nodes,
		Edges:  resp.Edges,
		Alerts: resp.Alerts,
	}
	data, _ := json.Marshal(h)
	sum := md5.Sum(data)
	return fmt.Sprintf(`"%x"`, sum)
}
