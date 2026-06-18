package http

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Stoganet/api-proxy/internal/auth"
)

func noopLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newStreamServer(t *testing.T, fa *fakeAuth, jellyfinURL string) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle("GET /stream/{jfId}", requireJWT(fa, newStreamHandler(fa, jellyfinURL, noopLogger())))
	return mux
}

func TestStream_NoJWT_Returns401(t *testing.T) {
	h := newStreamServer(t, &fakeAuth{}, "http://jf.example.com")

	req := httptest.NewRequest(http.MethodGet, "/stream/abc123", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("got %d, want 401", w.Code)
	}
}

func TestStream_ValidJWT_PipesBytes(t *testing.T) {
	jfSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/Videos/") {
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("api_key") != "jf-tok" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "video/mp4")
		_, _ = w.Write([]byte("fake-video-bytes"))
	}))
	defer jfSrv.Close()

	fa := &fakeAuth{
		verifyOut: &auth.Claims{UserID: "u1", JFUserID: "jf-uid"},
		jfTok:     "jf-tok",
	}
	h := newStreamServer(t, fa, jfSrv.URL)

	req := httptest.NewRequest(http.MethodGet, "/stream/abc123", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200. body: %s", w.Code, w.Body.String())
	}
	body, _ := io.ReadAll(w.Body)
	if string(body) != "fake-video-bytes" {
		t.Errorf("body: got %q, want %q", string(body), "fake-video-bytes")
	}
}

func TestStream_RangeHeader_ForwardedAndReturns206(t *testing.T) {
	var capturedRange string
	jfSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedRange = r.Header.Get("Range")
		w.Header().Set("Content-Range", "bytes 0-999/5000")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte("partial-bytes"))
	}))
	defer jfSrv.Close()

	fa := &fakeAuth{
		verifyOut: &auth.Claims{UserID: "u1", JFUserID: "jf-uid"},
		jfTok:     "jf-tok",
	}
	h := newStreamServer(t, fa, jfSrv.URL)

	req := httptest.NewRequest(http.MethodGet, "/stream/abc123", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Range", "bytes=0-999")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if capturedRange != "bytes=0-999" {
		t.Errorf("Range not forwarded to Jellyfin: got %q", capturedRange)
	}
	if w.Code != http.StatusPartialContent {
		t.Errorf("got %d, want 206", w.Code)
	}
}

func TestStream_JellyfinTokenLookupFails_Returns503(t *testing.T) {
	fa := &fakeAuth{
		verifyOut: &auth.Claims{UserID: "u1", JFUserID: "jf-uid"},
		jfTokErr:  errors.New("db error"),
	}
	h := newStreamServer(t, fa, "http://jf.example.com")

	req := httptest.NewRequest(http.MethodGet, "/stream/abc123", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("got %d, want 503", w.Code)
	}
}

func TestStream_JellyfinUnreachable_Returns503(t *testing.T) {
	fa := &fakeAuth{
		verifyOut: &auth.Claims{UserID: "u1", JFUserID: "jf-uid"},
		jfTok:     "jf-tok",
	}
	h := newStreamServer(t, fa, "http://127.0.0.1:1")

	req := httptest.NewRequest(http.MethodGet, "/stream/abc123", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("got %d, want 503", w.Code)
	}
}

func TestStream_AuthorizationHeader_NotForwardedToJellyfin(t *testing.T) {
	var capturedAuth string
	jfSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "video/mp4")
		_, _ = w.Write([]byte("bytes"))
	}))
	defer jfSrv.Close()

	fa := &fakeAuth{
		verifyOut: &auth.Claims{UserID: "u1", JFUserID: "jf-uid"},
		jfTok:     "jf-tok",
	}
	h := newStreamServer(t, fa, jfSrv.URL)

	req := httptest.NewRequest(http.MethodGet, "/stream/abc123", nil)
	req.Header.Set("Authorization", "Bearer client-jwt")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if capturedAuth != "" {
		t.Errorf("Authorization header must not reach Jellyfin, got %q", capturedAuth)
	}
}

func TestStream_JellyfinNotFound_Returns404WithEmptyBody(t *testing.T) {
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
	h := newStreamServer(t, fa, jfSrv.URL)

	req := httptest.NewRequest(http.MethodGet, "/stream/missing-id", nil)
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
