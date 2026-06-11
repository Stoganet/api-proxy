package auth

import (
	"context"
	"testing"
	"time"
)

func TestGetJellyfinToken_ReturnsTokenAfterLogin(t *testing.T) {
	s := newLoginSvc(t, &fakeJF{res: &JFAuthResult{
		AccessToken: "jf-tok-xyz",
		UserID:      "jf-uid-1",
		Username:    "alice",
	}})

	_, err := s.Login(context.Background(), "alice", "pw", nil)
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	// Retrieve proxy user ID from a fresh login to get the row in DB.
	pair, err := s.Login(context.Background(), "alice", "pw", nil)
	if err != nil {
		t.Fatalf("second login: %v", err)
	}

	tok, err := s.GetJellyfinToken(context.Background(), pair.User.ID)
	if err != nil {
		t.Fatalf("GetJellyfinToken: %v", err)
	}
	if tok != "jf-tok-xyz" {
		t.Errorf("token: got %q, want %q", tok, "jf-tok-xyz")
	}
}

func TestGetJellyfinToken_UnknownUser_ReturnsError(t *testing.T) {
	d := openTestDB(t)
	s := NewService(Options{
		DB:       d,
		Jellyfin: &fakeJF{},
		SignKey:  []byte("01234567890123456789012345678901"),
		Clock:    func() time.Time { return time.Unix(1_700_000_000, 0) },
	})

	_, err := s.GetJellyfinToken(context.Background(), "no-such-id")
	if err == nil {
		t.Fatal("expected error for unknown user")
	}
}
