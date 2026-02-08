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
