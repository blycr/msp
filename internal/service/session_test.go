package service

import (
	"testing"
	"time"
)

func TestNewSessionService(t *testing.T) {
	s := NewSessionService()
	if s == nil {
		t.Fatal("NewSessionService returned nil")
	}
	if s.sessions == nil {
		t.Fatal("sessions map should be initialized")
	}
}

func TestSessionService_CreateAndValidate(t *testing.T) {
	s := NewSessionService()

	token, err := s.CreateSession()
	if err != nil {
		t.Fatalf("CreateSession error: %v", err)
	}
	if token == "" {
		t.Fatal("token should not be empty")
	}
	// token 是 hex 编码的，长度应为 SessionTokenLength*2
	if len(token) < 32 {
		t.Errorf("token too short: %q", token)
	}

	if !s.ValidateSession(token) {
		t.Error("newly created session should be valid")
	}
}

func TestSessionService_ValidateEmpty(t *testing.T) {
	s := NewSessionService()
	if s.ValidateSession("") {
		t.Error("empty token should be invalid")
	}
}

func TestSessionService_ValidateUnknown(t *testing.T) {
	s := NewSessionService()
	if s.ValidateSession("deadbeefdeadbeef") {
		t.Error("unknown token should be invalid")
	}
}

func TestSessionService_ExpiredSession(t *testing.T) {
	s := NewSessionService()

	// 手动插入一个已过期的 token
	expiredToken := "expiredtoken1234"
	s.mu.Lock()
	s.sessions[expiredToken] = time.Now().Add(-1 * time.Second)
	s.mu.Unlock()

	if s.ValidateSession(expiredToken) {
		t.Error("expired session should be invalid")
	}

	// 验证后过期 token 应被清理
	s.mu.RLock()
	_, exists := s.sessions[expiredToken]
	s.mu.RUnlock()
	if exists {
		t.Error("expired token should be removed from sessions map after validation")
	}
}

func TestSessionService_MultipleTokens(t *testing.T) {
	s := NewSessionService()

	t1, err := s.CreateSession()
	if err != nil {
		t.Fatalf("CreateSession 1 error: %v", err)
	}
	t2, err := s.CreateSession()
	if err != nil {
		t.Fatalf("CreateSession 2 error: %v", err)
	}

	if t1 == t2 {
		t.Error("two sessions should produce different tokens")
	}
	if !s.ValidateSession(t1) {
		t.Error("session 1 should be valid")
	}
	if !s.ValidateSession(t2) {
		t.Error("session 2 should be valid")
	}
}

func TestSessionService_CleanupExpiredOnCreate(t *testing.T) {
	s := NewSessionService()

	// 插入一批过期 token
	s.mu.Lock()
	for i := 0; i < 5; i++ {
		s.sessions["old"+string(rune('0'+i))] = time.Now().Add(-1 * time.Second)
	}
	s.mu.Unlock()

	// 创建新 session 会触发 cleanupExpiredSessionsLocked
	_, err := s.CreateSession()
	if err != nil {
		t.Fatalf("CreateSession error: %v", err)
	}

	// 过期 token 应已被清理（只剩 1 个新创建的）
	s.mu.RLock()
	count := len(s.sessions)
	s.mu.RUnlock()
	if count > 1 {
		t.Errorf("expected 1 active session after cleanup, got %d", count)
	}
}
