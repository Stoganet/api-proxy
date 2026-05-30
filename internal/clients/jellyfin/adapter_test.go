package jellyfin

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Stoganet/api-proxy/internal/auth"
)

func newAdapterServer(t *testing.T, handler http.HandlerFunc) auth.JellyfinAuthenticator {
	t.Helper()
	s := httptest.NewServer(handler)
	t.Cleanup(s.Close)
	return AsAuthAdapter(New(s.URL, ""))
}

func TestAdapter_AuthenticateByName_TranslatesResult(t *testing.T) {
	a := newAdapterServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"AccessToken": "tok",
			"User":        map[string]any{"Id": "jf-1", "Name": "alice"},
		})
	})
	res, err := a.AuthenticateByName(context.Background(), "alice", "pw")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.AccessToken != "tok" || res.UserID != "jf-1" || res.Username != "alice" {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestAdapter_AuthenticateByName_TranslatesInvalidCredentials(t *testing.T) {
	a := newAdapterServer(t, func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "", http.StatusUnauthorized)
	})
	_, err := a.AuthenticateByName(context.Background(), "alice", "wrong")
	if !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatalf("want auth.ErrInvalidCredentials, got %v", err)
	}
}

func TestAdapter_AuthenticateByName_TranslatesUpstreamUnavailable(t *testing.T) {
	a := newAdapterServer(t, func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "", http.StatusInternalServerError)
	})
	_, err := a.AuthenticateByName(context.Background(), "alice", "pw")
	if !errors.Is(err, auth.ErrJellyfinUnavailable) {
		t.Fatalf("want auth.ErrJellyfinUnavailable, got %v", err)
	}
}

func TestAdapter_QuickConnectInitiate_TranslatesResult(t *testing.T) {
	a := newAdapterServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/QuickConnect/Initiate" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"Secret": "s", "Code": "C"})
	})
	res, err := a.QuickConnectInitiate(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Secret != "s" || res.Code != "C" {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestAdapter_QuickConnectInitiate_TranslatesUpstreamUnavailable(t *testing.T) {
	a := newAdapterServer(t, func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "", http.StatusInternalServerError)
	})
	_, err := a.QuickConnectInitiate(context.Background())
	if !errors.Is(err, auth.ErrJellyfinUnavailable) {
		t.Fatalf("want auth.ErrJellyfinUnavailable, got %v", err)
	}
}

func TestAdapter_QuickConnectAuthenticate_TranslatesPending(t *testing.T) {
	a := newAdapterServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/QuickConnect/Authenticate" {
			http.Error(w, "", http.StatusUnauthorized)
			return
		}
		http.NotFound(w, r)
	})
	_, err := a.QuickConnectAuthenticate(context.Background(), "secret")
	if !errors.Is(err, auth.ErrQuickConnectPending) {
		t.Fatalf("want auth.ErrQuickConnectPending, got %v", err)
	}
}

func TestAdapter_QuickConnectAuthenticate_TranslatesResult(t *testing.T) {
	a := newAdapterServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/QuickConnect/Authenticate" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"AccessToken": "tok-qc",
				"User":        map[string]any{"Id": "jf-2", "Name": "bob"},
			})
			return
		}
		http.NotFound(w, r)
	})
	res, err := a.QuickConnectAuthenticate(context.Background(), "secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.AccessToken != "tok-qc" || res.UserID != "jf-2" || res.Username != "bob" {
		t.Fatalf("unexpected result: %+v", res)
	}
}
