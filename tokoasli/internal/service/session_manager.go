package service

import (
	"sync"
	"time"

	"tokoasli/internal/util"
)

type Session struct {
	Token     string
	ExpiresAt time.Time
}

type SessionManager struct {
	sessions map[string]time.Time
	mu       sync.RWMutex
	ttl      time.Duration
}

func NewSessionManager(ttl time.Duration) *SessionManager {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	sm := &SessionManager{
		sessions: make(map[string]time.Time),
		ttl:      ttl,
	}

	// Periodically cleanup expired sessions
	go sm.cleanupWorker()

	return sm
}

// CreateSession generates a new 64-char crypto session token and stores it with expiration
func (s *SessionManager) CreateSession() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	token := util.NewAdminSessionToken()
	s.sessions[token] = time.Now().UTC().Add(s.ttl)
	return token
}

// ValidateSession checks if token exists and has not expired
func (s *SessionManager) ValidateSession(token string) bool {
	if token == "" {
		return false
	}

	s.mu.RLock()
	expiry, exists := s.sessions[token]
	s.mu.RUnlock()

	if !exists {
		return false
	}

	if time.Now().UTC().After(expiry) {
		s.RevokeSession(token)
		return false
	}

	return true
}

// RevokeSession invalidates a session token
func (s *SessionManager) RevokeSession(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, token)
}

func (s *SessionManager) cleanupWorker() {
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.Lock()
		now := time.Now().UTC()
		for token, expiry := range s.sessions {
			if now.After(expiry) {
				delete(s.sessions, token)
			}
		}
		s.mu.Unlock()
	}
}
