package auth

import (
	"context"
	"errors"
	"testing"
)

func TestLogout_RevokesOneToken(t *testing.T) {
	s := newLoginSvc(t, &fakeJF{res: &JFAuthResult{
		AccessToken: "jf-tok", UserID: "jf-1", Username: "alice",
	}})
	a, _ := s.Login(context.Background(), "alice", "x", nil)
	b, _ := s.Login(context.Background(), "alice", "x", nil)
	if err := s.Logout(context.Background(), a.RefreshToken); err != nil {
		t.Fatalf("%v", err)
	}
	if _, err := s.Refresh(context.Background(), b.RefreshToken); err != nil {
		t.Fatalf("b should still work: %v", err)
	}
	if _, err := s.Refresh(context.Background(), a.RefreshToken); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("expected invalid, got %v", err)
	}
}

func TestLogoutAll_RevokesAllForUser(t *testing.T) {
	s := newLoginSvc(t, &fakeJF{res: &JFAuthResult{
		AccessToken: "jf-tok", UserID: "jf-1", Username: "alice",
	}})
	a, _ := s.Login(context.Background(), "alice", "x", nil)
	b, _ := s.Login(context.Background(), "alice", "x", nil)
	pair, _ := s.Refresh(context.Background(), a.RefreshToken)
	if err := s.LogoutAll(context.Background(), pair.User.ID); err != nil {
		t.Fatalf("%v", err)
	}
	if _, err := s.Refresh(context.Background(), b.RefreshToken); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("b should be invalid")
	}
}
