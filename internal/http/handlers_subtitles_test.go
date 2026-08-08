package http

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Stoganet/api-proxy/internal/auth"
)

func newSubtitleServer(t *testing.T, fa *fakeAuth, jellyfinURL string) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle("GET /stream/{jfId}/subtitles/{index}", requireJWT(fa, newSubtitleHandler(fa, jellyfinURL, noopLogger())))
	return mux
}

func TestSubtitles_NoJWT_Returns401(t *testing.T) {
	h := newSubtitleServer(t, &fakeAuth{}, "http://jf.example.com")

	req := httptest.NewRequest(http.MethodGet, "/stream/abc123/subtitles/2", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("got %d, want 401", w.Code)
	}
}

func TestSubtitles_ValidJWT_PipesVTTBytes(t *testing.T) {
	var capturedPath, capturedAPIKey string
	jfSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedAPIKey = r.URL.Query().Get("api_key")
		w.Header().Set("Content-Type", "text/vtt")
		_, _ = w.Write([]byte("WEBVTT\n\nfake subtitle content"))
	}))
	defer jfSrv.Close()

	fa := &fakeAuth{
		verifyOut: &auth.Claims{UserID: "u1", JFUserID: "jf-uid"},
		jfTok:     "jf-tok",
	}
	h := newSubtitleServer(t, fa, jfSrv.URL)

	req := httptest.NewRequest(http.MethodGet, "/stream/abc123/subtitles/2", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200. body: %s", w.Code, w.Body.String())
	}
	if capturedPath != "/Videos/abc123/abc123/Subtitles/2/Stream.vtt" {
		t.Errorf("jellyfin path: got %q", capturedPath)
	}
	if capturedAPIKey != "jf-tok" {
		t.Errorf("api_key: got %q, want jf-tok", capturedAPIKey)
	}
	body, _ := io.ReadAll(w.Body)
	if string(body) != "WEBVTT\n\nfake subtitle content" {
		t.Errorf("body: got %q", string(body))
	}
}

func TestSubtitles_JellyfinTokenLookupFails_Returns503(t *testing.T) {
	fa := &fakeAuth{
		verifyOut: &auth.Claims{UserID: "u1", JFUserID: "jf-uid"},
		jfTokErr:  errors.New("db error"),
	}
	h := newSubtitleServer(t, fa, "http://jf.example.com")

	req := httptest.NewRequest(http.MethodGet, "/stream/abc123/subtitles/2", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("got %d, want 503", w.Code)
	}
}

func TestSubtitles_JellyfinUnreachable_Returns503(t *testing.T) {
	fa := &fakeAuth{
		verifyOut: &auth.Claims{UserID: "u1", JFUserID: "jf-uid"},
		jfTok:     "jf-tok",
	}
	h := newSubtitleServer(t, fa, "http://127.0.0.1:1")

	req := httptest.NewRequest(http.MethodGet, "/stream/abc123/subtitles/2", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("got %d, want 503", w.Code)
	}
}

func TestSubtitles_AuthorizationHeader_NotForwardedToJellyfin(t *testing.T) {
	var capturedAuth string
	jfSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "text/vtt")
		_, _ = w.Write([]byte("WEBVTT"))
	}))
	defer jfSrv.Close()

	fa := &fakeAuth{
		verifyOut: &auth.Claims{UserID: "u1", JFUserID: "jf-uid"},
		jfTok:     "jf-tok",
	}
	h := newSubtitleServer(t, fa, jfSrv.URL)

	req := httptest.NewRequest(http.MethodGet, "/stream/abc123/subtitles/2", nil)
	req.Header.Set("Authorization", "Bearer client-jwt")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if capturedAuth != "" {
		t.Errorf("Authorization header must not reach Jellyfin, got %q", capturedAuth)
	}
}

func TestSubtitles_JellyfinNotFound_Returns404WithEmptyBody(t *testing.T) {
	jfSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"Subtitle not found"}`))
	}))
	defer jfSrv.Close()

	fa := &fakeAuth{
		verifyOut: &auth.Claims{UserID: "u1", JFUserID: "jf-uid"},
		jfTok:     "jf-tok",
	}
	h := newSubtitleServer(t, fa, jfSrv.URL)

	req := httptest.NewRequest(http.MethodGet, "/stream/abc123/subtitles/99", nil)
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

func TestSubtitles_IndexPathSegment_ForwardedCorrectly(t *testing.T) {
	var capturedPath string
	jfSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		_, _ = w.Write([]byte("WEBVTT"))
	}))
	defer jfSrv.Close()

	fa := &fakeAuth{
		verifyOut: &auth.Claims{UserID: "u1", JFUserID: "jf-uid"},
		jfTok:     "jf-tok",
	}
	h := newSubtitleServer(t, fa, jfSrv.URL)

	req := httptest.NewRequest(http.MethodGet, "/stream/xyz789/subtitles/7", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if !strings.Contains(capturedPath, "/xyz789/xyz789/Subtitles/7/") {
		t.Errorf("jellyfin path: got %q", capturedPath)
	}
}
