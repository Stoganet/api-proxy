package jellyfin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestQuickConnectInitiate(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/QuickConnect/Initiate" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"Secret": "secret-xyz",
			"Code":   "ABC123",
		})
	}))
	defer s.Close()

	c := New(s.URL, "")
	res, err := c.QuickConnectInitiate(context.Background())
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if res.Secret != "secret-xyz" || res.Code != "ABC123" {
		t.Fatalf("bad: %+v", res)
	}
}

func TestQuickConnectAuthenticate_Approved(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/QuickConnect/Authenticate" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"AccessToken": "tok-qc",
			"User":        map[string]any{"Id": "jf-user-1", "Name": "alice"},
		})
	}))
	defer s.Close()

	c := New(s.URL, "")
	res, err := c.QuickConnectAuthenticate(context.Background(), "secret-xyz")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if res.AccessToken != "tok-qc" {
		t.Fatalf("bad: %+v", res)
	}
}

func TestQuickConnectAuthenticate_NotYetApproved(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "", http.StatusUnauthorized)
	}))
	defer s.Close()
	c := New(s.URL, "")
	_, err := c.QuickConnectAuthenticate(context.Background(), "secret-xyz")
	if !IsPending(err) {
		t.Fatalf("want IsPending, got %v", err)
	}
}
