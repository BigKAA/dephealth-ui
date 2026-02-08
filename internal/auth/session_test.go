package auth

import (
	"testing"
	"time"
)

func TestSessionStore_CreateAndGet(t *testing.T) {
	store := NewSessionStore(1 * time.Hour)
	defer store.Stop()

	user := UserInfo{Subject: "user-123", Name: "Alice", Email: "alice@example.com"}
	id, err := store.Create(user)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if id == "" {
		t.Fatal("Create() returned empty session ID")
	}

	sess := store.Get(id)
	if sess == nil {
		t.Fatal("Get() returned nil for valid session")
	}
	if sess.User.Subject != "user-123" {
		t.Errorf("User.Subject = %q, want %q", sess.User.Subject, "user-123")
	}
	if sess.User.Name != "Alice" {
		t.Errorf("User.Name = %q, want %q", sess.User.Name, "Alice")
	}
	if sess.User.Email != "alice@example.com" {
		t.Errorf("User.Email = %q, want %q", sess.User.Email, "alice@example.com")
	}
}

func TestSessionStore_Expiry(t *testing.T) {
	store := NewSessionStore(1 * time.Millisecond)
	defer store.Stop()

	user := UserInfo{Subject: "user-456"}
	id, err := store.Create(user)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	sess := store.Get(id)
	if sess != nil {
		t.Error("Get() should return nil for expired session")
	}
}

func TestSessionStore_Delete(t *testing.T) {
	store := NewSessionStore(1 * time.Hour)
	defer store.Stop()

	user := UserInfo{Subject: "user-789"}
	id, err := store.Create(user)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	store.Delete(id)

	sess := store.Get(id)
	if sess != nil {
		t.Error("Get() should return nil after Delete()")
	}
}

func TestSessionStore_NonExistent(t *testing.T) {
	store := NewSessionStore(1 * time.Hour)
	defer store.Stop()

	sess := store.Get("nonexistent-id")
	if sess != nil {
		t.Error("Get() should return nil for non-existent session")
	}
}

func TestSessionStore_UniqueIDs(t *testing.T) {
	store := NewSessionStore(1 * time.Hour)
	defer store.Stop()

	user := UserInfo{Subject: "user-1"}
	ids := make(map[string]bool)
	for range 100 {
		id, err := store.Create(user)
		if err != nil {
			t.Fatalf("Create() error: %v", err)
		}
		if ids[id] {
			t.Fatalf("duplicate session ID: %s", id)
		}
		ids[id] = true
	}
}
