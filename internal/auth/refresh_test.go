package auth

import (
	"context"
	"errors"
	"testing"
)

func TestRefresh_RotatesToken(t *testing.T) {
	s := newLoginSvc(t, &fakeJF{res: &JFAuthResult{
		AccessToken: "jf-tok", UserID: "jf-1", Username: "alice",
	}})
	pair, _ := s.Login(context.Background(), "alice", "x", nil)
	newPair, err := s.Refresh(context.Background(), pair.RefreshToken)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if newPair.RefreshToken == pair.RefreshToken {
		t.Fatal("refresh token should rotate")
	}
}

func TestRefresh_ReuseRevokesAll(t *testing.T) {
	s := newLoginSvc(t, &fakeJF{res: &JFAuthResult{
		AccessToken: "jf-tok", UserID: "jf-1", Username: "alice",
	}})
	pair, _ := s.Login(context.Background(), "alice", "x", nil)
	if _, err := s.Refresh(context.Background(), pair.RefreshToken); err != nil {
		t.Fatalf("first refresh: %v", err)
	}
	_, err := s.Refresh(context.Background(), pair.RefreshToken)
	if !errors.Is(err, ErrTokenReused) {
		t.Fatalf("expected reuse, got %v", err)
	}
}

func TestRefresh_UnknownToken(t *testing.T) {
	s := newLoginSvc(t, &fakeJF{})
	_, err := s.Refresh(context.Background(), "deadbeef")
	if !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("got %v", err)
	}
}
