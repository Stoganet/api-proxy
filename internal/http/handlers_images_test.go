package http

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Stoganet/api-proxy/internal/auth"
)

func newImageServer(t *testing.T, fa *fakeAuth, jellyfinURL string) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle("GET /images/{jfId}/{kind}", requireJWT(fa, newImageHandler(fa, jellyfinURL, noopLogger())))
	return mux
}

func TestImages_NoJWT_Returns401(t *testing.T) {
	h := newImageServer(t, &fakeAuth{}, "http://jf.example.com")

	req := httptest.NewRequest(http.MethodGet, "/images/abc123/primary", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("got %d, want 401", w.Code)
	}
}

func TestImages_Primary_PipesImageBytes(t *testing.T) {
	var capturedPath, capturedAPIKey string
	jfSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedAPIKey = r.URL.Query().Get("api_key")
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte("fake-jpeg-bytes"))
	}))
	defer jfSrv.Close()

	fa := &fakeAuth{
		verifyOut: &auth.Claims{UserID: "u1", JFUserID: "jf-uid"},
		jfTok:     "jf-tok",
	}
	h := newImageServer(t, fa, jfSrv.URL)

	req := httptest.NewRequest(http.MethodGet, "/images/abc123/primary", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200. body: %s", w.Code, w.Body.String())
	}
	if capturedPath != "/Items/abc123/Images/Primary" {
		t.Errorf("jellyfin path: got %q", capturedPath)
	}
	if capturedAPIKey != "jf-tok" {
		t.Errorf("api_key: got %q, want jf-tok", capturedAPIKey)
	}
	if body := w.Body.String(); body != "fake-jpeg-bytes" {
		t.Errorf("body: got %q", body)
	}
}

func TestImages_Backdrop_UsesBackdropPath(t *testing.T) {
	var capturedPath string
	jfSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		_, _ = w.Write([]byte("fake-jpeg-bytes"))
	}))
	defer jfSrv.Close()

	fa := &fakeAuth{
		verifyOut: &auth.Claims{UserID: "u1", JFUserID: "jf-uid"},
		jfTok:     "jf-tok",
	}
	h := newImageServer(t, fa, jfSrv.URL)

	req := httptest.NewRequest(http.MethodGet, "/images/abc123/backdrop", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if capturedPath != "/Items/abc123/Images/Backdrop/0" {
		t.Errorf("jellyfin path: got %q", capturedPath)
	}
}

func TestImages_UnknownKind_Returns404(t *testing.T) {
	fa := &fakeAuth{
		verifyOut: &auth.Claims{UserID: "u1", JFUserID: "jf-uid"},
		jfTok:     "jf-tok",
	}
	h := newImageServer(t, fa, "http://jf.example.com")

	req := httptest.NewRequest(http.MethodGet, "/images/abc123/thumbnail", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("got %d, want 404", w.Code)
	}
}

func TestImages_JellyfinTokenLookupFails_Returns503(t *testing.T) {
	fa := &fakeAuth{
		verifyOut: &auth.Claims{UserID: "u1", JFUserID: "jf-uid"},
		jfTokErr:  errors.New("db error"),
	}
	h := newImageServer(t, fa, "http://jf.example.com")

	req := httptest.NewRequest(http.MethodGet, "/images/abc123/primary", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("got %d, want 503", w.Code)
	}
}

func TestImages_JellyfinUnreachable_Returns503(t *testing.T) {
	fa := &fakeAuth{
		verifyOut: &auth.Claims{UserID: "u1", JFUserID: "jf-uid"},
		jfTok:     "jf-tok",
	}
	h := newImageServer(t, fa, "http://127.0.0.1:1")

	req := httptest.NewRequest(http.MethodGet, "/images/abc123/primary", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("got %d, want 503", w.Code)
	}
}

func TestImages_AuthorizationHeader_NotForwardedToJellyfin(t *testing.T) {
	var capturedAuth string
	jfSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte("fake-jpeg-bytes"))
	}))
	defer jfSrv.Close()

	fa := &fakeAuth{
		verifyOut: &auth.Claims{UserID: "u1", JFUserID: "jf-uid"},
		jfTok:     "jf-tok",
	}
	h := newImageServer(t, fa, jfSrv.URL)

	req := httptest.NewRequest(http.MethodGet, "/images/abc123/primary", nil)
	req.Header.Set("Authorization", "Bearer client-jwt")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if capturedAuth != "" {
		t.Errorf("Authorization header must not reach Jellyfin, got %q", capturedAuth)
	}
}

func TestImages_JellyfinNotFound_Returns404WithEmptyBody(t *testing.T) {
	jfSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"Item not found"}`))
	}))
	defer jfSrv.Close()

	fa := &fakeAuth{
		verifyOut: &auth.Claims{UserID: "u1", JFUserID: "jf-uid"},
		jfTok:     "jf-tok",
	}
	h := newImageServer(t, fa, jfSrv.URL)

	req := httptest.NewRequest(http.MethodGet, "/images/missing/primary", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("got %d, want 404", w.Code)
	}
	if body := w.Body.String(); body != "" {
		t.Errorf("jellyfin error body must not reach client, got %q", body)
	}
}
