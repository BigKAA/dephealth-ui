package cache

import (
	"sync"
	"testing"
	"time"

	"github.com/BigKAA/dephealth-ui/internal/topology"
)

func TestGetEmpty(t *testing.T) {
	c := New(10 * time.Second)
	resp, ok := c.Get()
	if ok {
		t.Error("expected ok=false for empty cache")
	}
	if resp != nil {
		t.Error("expected nil response for empty cache")
	}
}

func TestSetAndGet(t *testing.T) {
	c := New(10 * time.Second)
	want := &topology.TopologyResponse{
		Nodes: []topology.Node{{ID: "svc-go", Label: "svc-go"}},
	}

	c.Set(want)
	got, ok := c.Get()
	if !ok {
		t.Fatal("expected ok=true after Set")
	}
	if len(got.Nodes) != 1 || got.Nodes[0].ID != "svc-go" {
		t.Errorf("got %+v, want node with ID svc-go", got)
	}
}

func TestExpired(t *testing.T) {
	c := New(1 * time.Millisecond)
	c.Set(&topology.TopologyResponse{
		Nodes: []topology.Node{{ID: "svc-go"}},
	})

	time.Sleep(5 * time.Millisecond)

	_, ok := c.Get()
	if ok {
		t.Error("expected ok=false for expired entry")
	}
}

func TestNotExpired(t *testing.T) {
	c := New(1 * time.Hour)
	c.Set(&topology.TopologyResponse{
		Nodes: []topology.Node{{ID: "svc-go"}},
	})

	resp, ok := c.Get()
	if !ok {
		t.Fatal("expected ok=true for non-expired entry")
	}
	if resp.Nodes[0].ID != "svc-go" {
		t.Errorf("unexpected node ID: %s", resp.Nodes[0].ID)
	}
}

func TestGetWithETag(t *testing.T) {
	c := New(10 * time.Second)
	want := &topology.TopologyResponse{
		Nodes: []topology.Node{{ID: "svc-go", Label: "svc-go"}},
		Edges: []topology.Edge{{Source: "svc-go", Target: "postgres"}},
	}

	c.Set(want)
	resp, etag, ok := c.GetWithETag()
	if !ok {
		t.Fatal("expected ok=true after Set")
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if etag == "" {
		t.Fatal("expected non-empty ETag")
	}
	// ETag should be quoted hex
	if etag[0] != '"' || etag[len(etag)-1] != '"' {
		t.Errorf("ETag should be quoted, got %q", etag)
	}
}

func TestETagChangesWithData(t *testing.T) {
	c := New(10 * time.Second)

	c.Set(&topology.TopologyResponse{
		Nodes: []topology.Node{{ID: "svc-go"}},
	})
	_, etag1, _ := c.GetWithETag()

	c.Set(&topology.TopologyResponse{
		Nodes: []topology.Node{{ID: "svc-python"}},
	})
	_, etag2, _ := c.GetWithETag()

	if etag1 == etag2 {
		t.Error("ETag should change when data changes")
	}
}

func TestETagStableForSameData(t *testing.T) {
	c := New(10 * time.Second)
	resp := &topology.TopologyResponse{
		Nodes: []topology.Node{{ID: "svc-go", State: "ok"}},
		Edges: []topology.Edge{{Source: "svc-go", Target: "postgres"}},
	}

	c.Set(resp)
	_, etag1, _ := c.GetWithETag()

	c.Set(resp)
	_, etag2, _ := c.GetWithETag()

	if etag1 != etag2 {
		t.Errorf("ETag should be stable for same data, got %q and %q", etag1, etag2)
	}
}

func TestGetWithETagEmpty(t *testing.T) {
	c := New(10 * time.Second)
	_, etag, ok := c.GetWithETag()
	if ok {
		t.Error("expected ok=false for empty cache")
	}
	if etag != "" {
		t.Error("expected empty ETag for empty cache")
	}
}

func TestConcurrentAccess(t *testing.T) {
	c := New(1 * time.Hour)
	var wg sync.WaitGroup

	// Concurrent writers
	for i := range 10 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			c.Set(&topology.TopologyResponse{
				Nodes: []topology.Node{{ID: "svc", DependencyCount: n}},
			})
		}(i)
	}

	// Concurrent readers
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Get()
		}()
	}

	wg.Wait()

	// Should have some data after all writes
	resp, ok := c.Get()
	if !ok {
		t.Fatal("expected ok=true after concurrent writes")
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
}
