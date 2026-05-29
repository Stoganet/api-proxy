package auth

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeJF struct {
	res *JFAuthResult
	err error
}

func (f *fakeJF) AuthenticateByName(_ context.Context, _, _ string) (*JFAuthResult, error) {
	return f.res, f.err
}
func (f *fakeJF) QuickConnectInitiate(_ context.Context) (*JFQuickConnectInit, error) {
	return nil, errors.New("unused")
}
func (f *fakeJF) QuickConnectAuthenticate(_ context.Context, _ string) (*JFAuthResult, error) {
	return nil, errors.New("unused")
}

func newLoginSvc(t *testing.T, jf JellyfinAuthenticator) *Service {
	t.Helper()
	d := openTestDB(t)
	return NewService(Options{
		DB:       d.DB,
		Jellyfin: jf,
		SignKey:  []byte("01234567890123456789012345678901"),
		Clock:    func() time.Time { return time.Unix(1_700_000_000, 0) },
	})
}

func TestLogin_Success_CreatesUserAndTokens(t *testing.T) {
	s := newLoginSvc(t, &fakeJF{res: &JFAuthResult{
		AccessToken: "jf-tok", UserID: "jf-1", Username: "alice",
	}})
	pair, err := s.Login(context.Background(), "alice", "hunter2", nil)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Fatal("missing tokens")
	}
	if pair.User.DisplayName != "alice" {
		t.Fatalf("display: %q", pair.User.DisplayName)
	}
}

func TestLogin_BadCreds_RecordsFailure(t *testing.T) {
	s := newLoginSvc(t, &fakeJF{err: ErrInvalidCredentials})
	_, err := s.Login(context.Background(), "alice", "wrong", nil)
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("got %v", err)
	}
	locked, _ := s.IsLocked(context.Background(), "alice")
	if locked {
		t.Fatal("one failure should not lock")
	}
}

func TestLogin_LockedAccount_Returns423WithoutCallingJF(t *testing.T) {
	jf := &fakeJF{err: errors.New("should not be called")}
	s := newLoginSvc(t, jf)
	for i := 0; i < 5; i++ {
		_ = s.RecordAttempt(context.Background(), "alice", false)
	}
	_, err := s.Login(context.Background(), "alice", "any", nil)
	if !errors.Is(err, ErrAccountLocked) {
		t.Fatalf("expected locked, got %v", err)
	}
}
