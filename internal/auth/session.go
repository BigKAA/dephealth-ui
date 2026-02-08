package auth

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// UserInfo holds claims extracted from the OIDC ID token.
type UserInfo struct {
	Subject string `json:"sub"`
	Name    string `json:"name"`
	Email   string `json:"email"`
}

// Session represents an authenticated user session.
type Session struct {
	ID        string
	User      UserInfo
	ExpiresAt time.Time
}

// SessionStore manages in-memory user sessions with TTL-based expiry.
type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	ttl      time.Duration
	done     chan struct{}
}

// NewSessionStore creates a new store with the given session TTL and
// starts a background goroutine that cleans up expired sessions every 5 minutes.
func NewSessionStore(ttl time.Duration) *SessionStore {
	s := &SessionStore{
		sessions: make(map[string]*Session),
		ttl:      ttl,
		done:     make(chan struct{}),
	}
	go s.cleanup()
	return s
}

// Create stores a new session for the user and returns the session ID.
func (s *SessionStore) Create(user UserInfo) (string, error) {
	id, err := generateSessionID()
	if err != nil {
		return "", err
	}

	s.mu.Lock()
	s.sessions[id] = &Session{
		ID:        id,
		User:      user,
		ExpiresAt: time.Now().Add(s.ttl),
	}
	s.mu.Unlock()

	return id, nil
}

// Get retrieves a session by ID. Returns nil if the session does not exist or is expired.
func (s *SessionStore) Get(id string) *Session {
	s.mu.RLock()
	sess, ok := s.sessions[id]
	s.mu.RUnlock()

	if !ok {
		return nil
	}
	if time.Now().After(sess.ExpiresAt) {
		s.Delete(id)
		return nil
	}
	return sess
}

// Delete removes a session by ID.
func (s *SessionStore) Delete(id string) {
	s.mu.Lock()
	delete(s.sessions, id)
	s.mu.Unlock()
}

// Stop terminates the background cleanup goroutine.
func (s *SessionStore) Stop() {
	close(s.done)
}

func (s *SessionStore) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.done:
			return
		case <-ticker.C:
			now := time.Now()
			s.mu.Lock()
			for id, sess := range s.sessions {
				if now.After(sess.ExpiresAt) {
					delete(s.sessions, id)
				}
			}
			s.mu.Unlock()
		}
	}
}

func generateSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
