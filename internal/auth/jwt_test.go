package auth

import (
	"testing"
	"time"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	return NewService(Options{
		SignKey:    []byte("01234567890123456789012345678901"),
		Clock:      func() time.Time { return time.Unix(1_700_000_000, 0) },
		AccessTTL:  time.Hour,
	})
}

func TestIssueAndVerifyJWT(t *testing.T) {
	s := newTestService(t)
	tok, err := s.issueJWT("user-1", "alice@example.com", "jf-1")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	claims, err := s.VerifyJWT(tok)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if claims.UserID != "user-1" || claims.Email != "alice@example.com" || claims.JFUserID != "jf-1" {
		t.Fatalf("bad claims: %+v", claims)
	}
}

func TestVerifyJWT_Expired(t *testing.T) {
	s := newTestService(t)
	tok, _ := s.issueJWT("user-1", "a@b", "jf-1")
	s.clock = func() time.Time { return time.Unix(1_700_000_000, 0).Add(2 * time.Hour) }
	_, err := s.VerifyJWT(tok)
	if err == nil {
		t.Fatal("expected expiry error")
	}
}

func TestVerifyJWT_BadSignature(t *testing.T) {
	s := newTestService(t)
	tok, _ := s.issueJWT("user-1", "a@b", "jf-1")
	tampered := tok[:len(tok)-2] + "xx"
	if _, err := s.VerifyJWT(tampered); err == nil {
		t.Fatal("expected signature error")
	}
}
