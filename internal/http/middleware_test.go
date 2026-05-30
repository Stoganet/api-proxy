package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Stoganet/api-proxy/internal/auth"
	"github.com/Stoganet/api-proxy/internal/gen"
)

func TestRequestIDMiddleware_SetsHeader(t *testing.T) {
	h := RequestID(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Header().Get("X-Request-Id") == "" {
		t.Fatal("missing X-Request-Id")
	}
}

func TestJWTMiddleware_RejectsMissingHeader(t *testing.T) {
	svc := newTestAuthSvc(t)
	mw := jwtStrictMiddleware(svc)
	inner := mw(func(_ context.Context, w http.ResponseWriter, _ *http.Request, _ any) (any, error) {
		w.WriteHeader(http.StatusOK)
		return nil, nil
	}, "postAuthLogout")

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	w := httptest.NewRecorder()
	_, _ = inner(req.Context(), w, req, nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("got %d, want 401", w.Code)
	}
}

func TestJWTMiddleware_AcceptsValidToken(t *testing.T) {
	svc := newTestAuthSvc(t)
	tok, err := svc.IssueAccessTokenForTest("user-1", "a@b", "jf-1")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	mw := jwtStrictMiddleware(svc)
	inner := mw(func(_ context.Context, w http.ResponseWriter, _ *http.Request, _ any) (any, error) {
		w.WriteHeader(http.StatusOK)
		return nil, nil
	}, "postAuthLogout")

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	_, _ = inner(req.Context(), w, req, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", w.Code)
	}
}

func TestJWTMiddleware_RejectsMalformedToken(t *testing.T) {
	svc := newTestAuthSvc(t)
	mw := jwtStrictMiddleware(svc)
	inner := mw(func(_ context.Context, w http.ResponseWriter, _ *http.Request, _ any) (any, error) {
		w.WriteHeader(http.StatusOK)
		return nil, nil
	}, "postAuthLogout")

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer not.a.jwt")
	w := httptest.NewRecorder()
	_, _ = inner(req.Context(), w, req, nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("got %d, want 401", w.Code)
	}
	var e gen.Error
	if err := json.NewDecoder(w.Body).Decode(&e); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if e.Error.Code != gen.TokenInvalid {
		t.Fatalf("got error code %q, want %q", e.Error.Code, gen.TokenInvalid)
	}
}

func TestJWTMiddleware_RejectsExpiredToken(t *testing.T) {
	// Clock at issue time is in the past relative to the verification clock.
	pastSvc := auth.NewService(auth.Options{
		SignKey:   []byte("01234567890123456789012345678901"),
		Clock:     func() time.Time { return time.Unix(1_000_000_000, 0) },
		AccessTTL: time.Second,
	})
	tok, err := pastSvc.IssueAccessTokenForTest("user-1", "a@b", "jf-1")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	// Verify with a service whose clock is well past the token's expiry.
	verifySvc := newTestAuthSvc(t) // clock at 1_700_000_000, far beyond 1_000_000_001
	mw := jwtStrictMiddleware(verifySvc)
	inner := mw(func(_ context.Context, w http.ResponseWriter, _ *http.Request, _ any) (any, error) {
		w.WriteHeader(http.StatusOK)
		return nil, nil
	}, "postAuthLogout")

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	_, _ = inner(req.Context(), w, req, nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("got %d, want 401", w.Code)
	}
	var e gen.Error
	if err := json.NewDecoder(w.Body).Decode(&e); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if e.Error.Code != gen.TokenExpired {
		t.Fatalf("got error code %q, want %q", e.Error.Code, gen.TokenExpired)
	}
}

func TestJWTMiddleware_PassesThroughNonProtectedRoutes(t *testing.T) {
	svc := newTestAuthSvc(t)
	mw := jwtStrictMiddleware(svc)
	called := false
	inner := mw(func(_ context.Context, w http.ResponseWriter, _ *http.Request, _ any) (any, error) {
		called = true
		w.WriteHeader(http.StatusOK)
		return nil, nil
	}, "postAuthLogin")

	req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
	w := httptest.NewRecorder()
	_, _ = inner(req.Context(), w, req, nil)
	if !called {
		t.Fatal("handler should be called for non-protected route")
	}
}

func newTestAuthSvc(t *testing.T) *auth.Service {
	t.Helper()
	return auth.NewService(auth.Options{
		SignKey:   []byte("01234567890123456789012345678901"),
		Clock:     func() time.Time { return time.Unix(1_700_000_000, 0) },
		AccessTTL: time.Hour,
	})
}

// Ensure Server satisfies the generated StrictServerInterface at compile time.
var _ gen.StrictServerInterface = (*Server)(nil)
