package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Stoganet/api-proxy/internal/auth"
	"github.com/Stoganet/api-proxy/internal/gen"
)

// fakeAuth is a configurable stub for authService used in handler tests.
type fakeAuth struct {
	loginOut     *auth.TokenPair
	loginErr     error
	refreshOut   *auth.TokenPair
	refreshErr   error
	logoutErr    error
	logoutAllErr error
	qcStartOut   *auth.QuickConnectStartOut
	qcStartErr   error
	qcPollOut    *auth.TokenPair
	qcPollErr    error
	verifyOut    *auth.Claims
	verifyErr    error
	jfTok        string
	jfTokErr     error
}

func (f *fakeAuth) Login(_ context.Context, _, _ string, _ *string) (*auth.TokenPair, error) {
	return f.loginOut, f.loginErr
}
func (f *fakeAuth) Refresh(_ context.Context, _ string) (*auth.TokenPair, error) {
	return f.refreshOut, f.refreshErr
}
func (f *fakeAuth) Logout(_ context.Context, _ string) error    { return f.logoutErr }
func (f *fakeAuth) LogoutAll(_ context.Context, _ string) error { return f.logoutAllErr }
func (f *fakeAuth) QuickConnectStart(_ context.Context) (*auth.QuickConnectStartOut, error) {
	return f.qcStartOut, f.qcStartErr
}
func (f *fakeAuth) QuickConnectPoll(_ context.Context, _ string) (*auth.TokenPair, error) {
	return f.qcPollOut, f.qcPollErr
}
func (f *fakeAuth) VerifyJWT(_ string) (*auth.Claims, error) { return f.verifyOut, f.verifyErr }
func (f *fakeAuth) GetJellyfinToken(_ context.Context, _ string) (string, error) {
	return f.jfTok, f.jfTokErr
}

var testTokenPair = &auth.TokenPair{
	AccessToken:  "access",
	RefreshToken: "refresh",
	User:         auth.User{ID: "u1", Email: "a@b.com", DisplayName: "A"},
}

func newTestServer(t *testing.T, fa *fakeAuth) http.Handler {
	t.Helper()
	s := &Server{auth: fa}
	strict := gen.NewStrictHandlerWithOptions(s, nil, gen.StrictHTTPServerOptions{})
	return gen.Handler(strict)
}

func newTestServerWithJWT(t *testing.T, fa *fakeAuth) http.Handler {
	t.Helper()
	s := &Server{auth: fa}
	strict := gen.NewStrictHandlerWithOptions(s, []gen.StrictMiddlewareFunc{
		jwtStrictMiddleware(fa),
	}, gen.StrictHTTPServerOptions{})
	return gen.Handler(strict)
}

func TestPublicEndpoints_NoTokenRequired(t *testing.T) {
	fa := &fakeAuth{loginOut: testTokenPair, refreshOut: testTokenPair}
	h := newTestServerWithJWT(t, fa)

	endpoints := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/auth/login", `{"username":"u","password":"p"}`},
		{http.MethodPost, "/auth/refresh", `{"refresh_token":"tok"}`},
		{http.MethodGet, "/healthz", ""},
	}
	for _, ep := range endpoints {
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			w := do(t, h, ep.method, ep.path, ep.body)
			if w.Code == http.StatusUnauthorized {
				t.Fatalf("public endpoint %s %s returned 401 — JWT middleware is blocking it", ep.method, ep.path)
			}
		})
	}
}

func do(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var buf *bytes.Buffer
	if body != "" {
		buf = bytes.NewBufferString(body)
	} else {
		buf = bytes.NewBuffer(nil)
	}
	req := httptest.NewRequest(method, path, buf)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func decodeError(t *testing.T, w *httptest.ResponseRecorder) gen.Error {
	t.Helper()
	var e gen.Error
	if err := json.NewDecoder(w.Body).Decode(&e); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	return e
}

func TestGetHealthz(t *testing.T) {
	h := newTestServer(t, &fakeAuth{})
	w := do(t, h, http.MethodGet, "/healthz", "")
	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", w.Code)
	}
}

func TestPostAuthLogin(t *testing.T) {
	body := `{"username":"u","password":"p"}`
	tests := []struct {
		name     string
		fa       *fakeAuth
		wantCode int
		wantErr  gen.ErrorErrorCode
	}{
		{"ok", &fakeAuth{loginOut: testTokenPair}, http.StatusOK, ""},
		{"invalid creds", &fakeAuth{loginErr: auth.ErrInvalidCredentials}, http.StatusUnauthorized, gen.InvalidCredentials},
		{"account locked", &fakeAuth{loginErr: auth.ErrAccountLocked}, http.StatusLocked, gen.AccountLocked},
		{"backend unavailable", &fakeAuth{loginErr: auth.ErrJellyfinUnavailable}, http.StatusServiceUnavailable, gen.BackendUnavailable},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := do(t, newTestServer(t, tc.fa), http.MethodPost, "/auth/login", body)
			if w.Code != tc.wantCode {
				t.Fatalf("got %d, want %d", w.Code, tc.wantCode)
			}
			if tc.wantErr != "" {
				e := decodeError(t, w)
				if e.Error.Code != tc.wantErr {
					t.Fatalf("got error code %q, want %q", e.Error.Code, tc.wantErr)
				}
			}
		})
	}
}

func TestPostAuthRefresh(t *testing.T) {
	body := `{"refresh_token":"tok"}`
	tests := []struct {
		name     string
		fa       *fakeAuth
		wantCode int
		wantErr  gen.ErrorErrorCode
	}{
		{"ok", &fakeAuth{refreshOut: testTokenPair}, http.StatusOK, ""},
		{"expired", &fakeAuth{refreshErr: auth.ErrTokenExpired}, http.StatusUnauthorized, gen.TokenExpired},
		{"invalid", &fakeAuth{refreshErr: auth.ErrTokenInvalid}, http.StatusUnauthorized, gen.TokenInvalid},
		{"reused", &fakeAuth{refreshErr: auth.ErrTokenReused}, http.StatusUnauthorized, gen.TokenInvalid},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := do(t, newTestServer(t, tc.fa), http.MethodPost, "/auth/refresh", body)
			if w.Code != tc.wantCode {
				t.Fatalf("got %d, want %d", w.Code, tc.wantCode)
			}
			if tc.wantErr != "" {
				e := decodeError(t, w)
				if e.Error.Code != tc.wantErr {
					t.Fatalf("got error code %q, want %q", e.Error.Code, tc.wantErr)
				}
			}
		})
	}
}

func TestPostAuthLogout(t *testing.T) {
	body := `{"refresh_token":"tok"}`
	w := do(t, newTestServer(t, &fakeAuth{}), http.MethodPost, "/auth/logout", body)
	if w.Code != http.StatusNoContent {
		t.Fatalf("got %d, want 204", w.Code)
	}
}

func TestPostAuthLogoutAll_MissingUID(t *testing.T) {
	// Without JWT middleware the ctxUserID key is absent and handler must return an error.
	s := &Server{auth: &fakeAuth{}}
	_, err := s.PostAuthLogoutAll(context.Background(), gen.PostAuthLogoutAllRequestObject{})
	if err == nil || !strings.Contains(err.Error(), "ctxUserID missing") {
		t.Fatalf("expected ctxUserID error, got %v", err)
	}
}

func TestPostAuthLogoutAll_WithUID(t *testing.T) {
	s := &Server{auth: &fakeAuth{}}
	ctx := context.WithValue(context.Background(), ctxUserID, "user-1")
	resp, err := s.PostAuthLogoutAll(ctx, gen.PostAuthLogoutAllRequestObject{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := resp.(gen.PostAuthLogoutAll204Response); !ok {
		t.Fatalf("got %T, want PostAuthLogoutAll204Response", resp)
	}
}

func TestPostAuthQuickConnectStart(t *testing.T) {
	tests := []struct {
		name     string
		fa       *fakeAuth
		wantCode int
	}{
		{"ok", &fakeAuth{qcStartOut: &auth.QuickConnectStartOut{Code: "ABC", PollToken: "tok"}}, http.StatusOK},
		{"backend unavailable", &fakeAuth{qcStartErr: auth.ErrJellyfinUnavailable}, http.StatusServiceUnavailable},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := do(t, newTestServer(t, tc.fa), http.MethodPost, "/auth/quick-connect/start", "")
			if w.Code != tc.wantCode {
				t.Fatalf("got %d, want %d", w.Code, tc.wantCode)
			}
		})
	}
}

func TestPostAuthQuickConnectPoll(t *testing.T) {
	body := `{"poll_token":"tok"}`
	tests := []struct {
		name     string
		fa       *fakeAuth
		wantCode int
	}{
		{"ok", &fakeAuth{qcPollOut: testTokenPair}, http.StatusOK},
		{"pending", &fakeAuth{qcPollErr: auth.ErrQuickConnectPending}, http.StatusAccepted},
		{"expired", &fakeAuth{qcPollErr: auth.ErrQuickConnectExpired}, http.StatusGone},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := do(t, newTestServer(t, tc.fa), http.MethodPost, "/auth/quick-connect/poll", body)
			if w.Code != tc.wantCode {
				t.Fatalf("got %d, want %d", w.Code, tc.wantCode)
			}
		})
	}
}
