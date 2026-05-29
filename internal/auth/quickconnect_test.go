package auth

import (
	"context"
	"errors"
	"testing"
)

type qcJF struct {
	init *JFQuickConnectInit
	res  *JFAuthResult
	err  error
}

func (q *qcJF) AuthenticateByName(_ context.Context, _, _ string) (*JFAuthResult, error) {
	return nil, errors.New("unused")
}
func (q *qcJF) QuickConnectInitiate(_ context.Context) (*JFQuickConnectInit, error) {
	return q.init, nil
}
func (q *qcJF) QuickConnectAuthenticate(_ context.Context, _ string) (*JFAuthResult, error) {
	return q.res, q.err
}

func TestQuickConnect_StartIssuesPollToken(t *testing.T) {
	s := newLoginSvc(t, &qcJF{init: &JFQuickConnectInit{Secret: "sec", Code: "ABC123"}})
	out, err := s.QuickConnectStart(context.Background())
	if err != nil {
		t.Fatalf("%v", err)
	}
	if out.Code != "ABC123" || out.PollToken == "" {
		t.Fatalf("bad: %+v", out)
	}
}

func TestQuickConnect_PollPending(t *testing.T) {
	s := newLoginSvc(t, &qcJF{
		init: &JFQuickConnectInit{Secret: "sec", Code: "ABC123"},
		err:  ErrQuickConnectPending,
	})
	start, _ := s.QuickConnectStart(context.Background())
	_, err := s.QuickConnectPoll(context.Background(), start.PollToken)
	if !errors.Is(err, ErrQuickConnectPending) {
		t.Fatalf("got %v", err)
	}
}

func TestQuickConnect_PollApproved(t *testing.T) {
	s := newLoginSvc(t, &qcJF{
		init: &JFQuickConnectInit{Secret: "sec", Code: "ABC123"},
		res:  &JFAuthResult{AccessToken: "jf-tok", UserID: "jf-1", Username: "alice"},
	})
	start, _ := s.QuickConnectStart(context.Background())
	pair, err := s.QuickConnectPoll(context.Background(), start.PollToken)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Fatal("missing tokens")
	}
}

func TestQuickConnect_UnknownPollToken(t *testing.T) {
	s := newLoginSvc(t, &qcJF{init: &JFQuickConnectInit{Secret: "sec", Code: "X"}})
	_, err := s.QuickConnectPoll(context.Background(), "no-such-token")
	if !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("got %v", err)
	}
}
