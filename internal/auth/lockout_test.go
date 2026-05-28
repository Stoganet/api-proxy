package auth

import (
	"context"
	"testing"
	"time"

	"github.com/Stoganet/api-proxy/internal/db"
)

func openTestDB(t *testing.T) *db.DB {
	t.Helper()
	d, err := db.Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return d
}

func newLockoutSvc(t *testing.T, now time.Time) *Service {
	t.Helper()
	d := openTestDB(t)
	return NewService(Options{
		DB:      d.DB,
		SignKey:  []byte("01234567890123456789012345678901"),
		Clock:   func() time.Time { return now },
	})
}

func TestIsLocked_NoAttempts(t *testing.T) {
	s := newLockoutSvc(t, time.Unix(1_700_000_000, 0))
	locked, err := s.IsLocked(context.Background(), "alice")
	if err != nil {
		t.Fatalf("%v", err)
	}
	if locked {
		t.Fatal("should not be locked")
	}
}

func TestIsLocked_AfterFiveFails(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	s := newLockoutSvc(t, now)
	for i := 0; i < 5; i++ {
		if err := s.RecordAttempt(context.Background(), "alice", false); err != nil {
			t.Fatalf("%v", err)
		}
	}
	locked, err := s.IsLocked(context.Background(), "alice")
	if err != nil {
		t.Fatalf("%v", err)
	}
	if !locked {
		t.Fatal("expected locked after 5 fails")
	}
}

func TestIsLocked_ExpiresAfterWindow(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	s := newLockoutSvc(t, now)
	for i := 0; i < 5; i++ {
		_ = s.RecordAttempt(context.Background(), "alice", false)
	}
	s.clock = func() time.Time { return now.Add(16 * time.Minute) }
	locked, _ := s.IsLocked(context.Background(), "alice")
	if locked {
		t.Fatal("lockout should have expired")
	}
}
