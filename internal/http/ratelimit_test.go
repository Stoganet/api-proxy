package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Stoganet/api-proxy/internal/gen"
	"github.com/go-chi/chi/v5/middleware"
)

func rateLimitedHandler(t *testing.T, mw gen.StrictMiddlewareFunc, operationID string) http.Handler {
	t.Helper()
	inner := func(_ context.Context, _ http.ResponseWriter, _ *http.Request, _ any) (any, error) {
		return nil, nil
	}
	wrapped := mw(inner, operationID)
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := wrapped(r.Context(), w, r, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	return middleware.ClientIPFromXFF()(h)
}

func requestFrom(h http.Handler, ip string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", ip)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func TestRateLimit_AllowsUnderLimit(t *testing.T) {
	mw, _ := newRateLimitStrictMiddleware(2, 2, 2, time.Minute)
	h := rateLimitedHandler(t, mw, "GetSearch")

	for i := range 2 {
		w := requestFrom(h, "10.0.0.1")
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: got %d, want 200", i, w.Code)
		}
	}
}

func TestRateLimit_BlocksOverLimit(t *testing.T) {
	mw, _ := newRateLimitStrictMiddleware(2, 2, 2, time.Minute)
	h := rateLimitedHandler(t, mw, "GetSearch")

	for range 2 {
		requestFrom(h, "10.0.0.2")
	}
	w := requestFrom(h, "10.0.0.2")
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("got %d, want 429", w.Code)
	}

	e := decodeError(t, w)
	if e.Error.Code != gen.RateLimited {
		t.Errorf("code: got %q", e.Error.Code)
	}
}

func TestRateLimit_SeparateKeysPerIP(t *testing.T) {
	mw, _ := newRateLimitStrictMiddleware(1, 1, 1, time.Minute)
	h := rateLimitedHandler(t, mw, "GetSearch")

	if w := requestFrom(h, "10.0.0.3"); w.Code != http.StatusOK {
		t.Fatalf("ip1: got %d, want 200", w.Code)
	}
	if w := requestFrom(h, "10.0.0.4"); w.Code != http.StatusOK {
		t.Fatalf("ip2: got %d, want 200", w.Code)
	}
}

func TestRateLimit_ExemptOperationBypassesLimit(t *testing.T) {
	mw, _ := newRateLimitStrictMiddleware(1, 1, 1, time.Minute)
	h := rateLimitedHandler(t, mw, "GetHealthz")

	for i := range 5 {
		w := requestFrom(h, "10.0.0.5")
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: got %d, want 200", i, w.Code)
		}
	}
}

func TestStripUntrustedForwardedFor_KeepsHeaderFromTraefik(t *testing.T) {
	orig := lookupHost
	lookupHost = func(string) ([]string, error) { return []string{"172.20.0.5"}, nil }
	t.Cleanup(func() { lookupHost = orig })

	var got string
	h := stripUntrustedForwardedFor(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("X-Forwarded-For")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "172.20.0.5:4321"
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	h.ServeHTTP(httptest.NewRecorder(), req)

	if got != "1.2.3.4" {
		t.Errorf("X-Forwarded-For: got %q, want kept", got)
	}
}

func TestStripUntrustedForwardedFor_StripsHeaderFromOtherContainer(t *testing.T) {
	orig := lookupHost
	lookupHost = func(string) ([]string, error) { return []string{"172.20.0.5"}, nil }
	t.Cleanup(func() { lookupHost = orig })

	var found bool
	h := stripUntrustedForwardedFor(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, found = r.Header["X-Forwarded-For"]
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "172.20.0.9:4321"
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	h.ServeHTTP(httptest.NewRecorder(), req)

	if found {
		t.Error("X-Forwarded-For should be stripped when RemoteAddr isn't Traefik")
	}
}

func TestStripUntrustedForwardedFor_DNSFailure_Strips(t *testing.T) {
	orig := lookupHost
	lookupHost = func(string) ([]string, error) { return nil, errors.New("no such host") }
	t.Cleanup(func() { lookupHost = orig })

	var found bool
	h := stripUntrustedForwardedFor(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, found = r.Header["X-Forwarded-For"]
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "172.20.0.5:4321"
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	h.ServeHTTP(httptest.NewRecorder(), req)

	if found {
		t.Error("X-Forwarded-For should be stripped when Traefik lookup fails")
	}
}

func TestRateLimitKey_FallsBackToRemoteAddr(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "9.9.9.9:5555"

	key, err := rateLimitKey(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "9.9.9.9" {
		t.Errorf("key: got %q, want %q", key, "9.9.9.9")
	}
}

func TestRateLimit_PollOperationUsesPollTier(t *testing.T) {
	mw, _ := newRateLimitStrictMiddleware(3, 1, 1, time.Minute)
	h := rateLimitedHandler(t, mw, "PostAuthQuickConnectPoll")

	for i := range 3 {
		w := requestFrom(h, "10.0.0.6")
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: got %d, want 200", i, w.Code)
		}
	}
	w := requestFrom(h, "10.0.0.6")
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("got %d, want 429", w.Code)
	}
}
