package service

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"msp/internal/constants"
)

type SessionService struct {
	mu       sync.RWMutex
	sessions map[string]time.Time
}

func NewSessionService() *SessionService {
	return &SessionService{
		sessions: make(map[string]time.Time),
	}
}

func (s *SessionService) CreateSession() (string, error) {
	token := make([]byte, constants.SessionTokenLength)
	if _, err := rand.Read(token); err != nil {
		return "", err
	}
	tokenStr := hex.EncodeToString(token)

	s.mu.Lock()
	defer s.mu.Unlock()

	s.sessions[tokenStr] = time.Now().Add(time.Duration(constants.CookieMaxAge)*time.Second)

	s.cleanupExpiredSessionsLocked()

	return tokenStr, nil
}

func (s *SessionService) ValidateSession(token string) bool {
	if token == "" {
		return false
	}

	s.mu.RLock()
	expiry, exists := s.sessions[token]
	s.mu.RUnlock()

	if !exists {
		return false
	}

	if time.Now().After(expiry) {
		s.mu.Lock()
		delete(s.sessions, token)
		s.mu.Unlock()
		return false
	}

	return true
}

func (s *SessionService) cleanupExpiredSessionsLocked() {
	now := time.Now()
	for token, expiry := range s.sessions {
		if now.After(expiry) {
			delete(s.sessions, token)
		}
	}
}