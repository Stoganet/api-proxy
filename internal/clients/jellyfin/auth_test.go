package jellyfin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthenticateByName_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/Users/AuthenticateByName" {
			http.NotFound(w, r)
			return
		}
		var body struct{ Username, Pw string }
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Username != "alice" || body.Pw != "hunter2" {
			http.Error(w, "bad", http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"AccessToken": "tok-abc",
			"User": map[string]any{
				"Id":   "jf-user-1",
				"Name": "alice",
			},
		})
	}))
	defer server.Close()

	c := New(server.URL, "")
	res, err := c.AuthenticateByName(context.Background(), "alice", "hunter2")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if res.AccessToken != "tok-abc" || res.UserID != "jf-user-1" || res.Username != "alice" {
		t.Fatalf("bad response: %+v", res)
	}
}

func TestAuthenticateByName_BadCreds(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "", http.StatusUnauthorized)
	}))
	defer server.Close()

	c := New(server.URL, "")
	_, err := c.AuthenticateByName(context.Background(), "alice", "wrong")
	if err == nil {
		t.Fatal("expected error")
	}
	if !IsInvalidCredentials(err) {
		t.Fatalf("want ErrInvalidCredentials, got %v", err)
	}
}
