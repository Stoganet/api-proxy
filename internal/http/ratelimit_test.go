package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Stoganet/api-proxy/internal/gen"
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
	return h
}

func requestFrom(h http.Handler, ip string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = ip + ":1234"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func TestRateLimit_AllowsUnderLimit(t *testing.T) {
	mw, _, _ := newRateLimitStrictMiddleware(2, 2, 2, 2, time.Minute)
	h := rateLimitedHandler(t, mw, "GetSearch")

	for i := range 2 {
		w := requestFrom(h, "10.0.0.1")
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: got %d, want 200", i, w.Code)
		}
	}
}

func TestRateLimit_BlocksOverLimit(t *testing.T) {
	mw, _, _ := newRateLimitStrictMiddleware(2, 2, 2, 2, time.Minute)
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
	mw, _, _ := newRateLimitStrictMiddleware(1, 1, 1, 1, time.Minute)
	h := rateLimitedHandler(t, mw, "GetSearch")

	if w := requestFrom(h, "10.0.0.3"); w.Code != http.StatusOK {
		t.Fatalf("ip1: got %d, want 200", w.Code)
	}
	if w := requestFrom(h, "10.0.0.4"); w.Code != http.StatusOK {
		t.Fatalf("ip2: got %d, want 200", w.Code)
	}
}

func TestRateLimit_ExemptOperationBypassesLimit(t *testing.T) {
	mw, _, _ := newRateLimitStrictMiddleware(1, 1, 1, 1, time.Minute)
	h := rateLimitedHandler(t, mw, "GetHealthz")

	for i := range 5 {
		w := requestFrom(h, "10.0.0.5")
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: got %d, want 200", i, w.Code)
		}
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
	mw, _, _ := newRateLimitStrictMiddleware(3, 1, 1, 1, time.Minute)
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

func TestRateLimit_IgnoresForwardedFor(t *testing.T) {
	mw, _, _ := newRateLimitStrictMiddleware(1, 1, 1, 1, time.Minute)
	h := rateLimitedHandler(t, mw, "GetSearch")

	for i, xff := range []string{"1.1.1.1", "2.2.2.2"} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "10.0.0.7:1234"
		req.Header.Set("X-Forwarded-For", xff)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		want := http.StatusOK
		if i == 1 {
			want = http.StatusTooManyRequests
		}
		if w.Code != want {
			t.Fatalf("request %d: got %d, want %d", i, w.Code, want)
		}
	}
}
