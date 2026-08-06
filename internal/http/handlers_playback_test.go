package http

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Stoganet/api-proxy/internal/gen"
	"github.com/Stoganet/api-proxy/internal/media"
)

func authedPut(t *testing.T, h http.Handler, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPut, path, bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func TestPutPlaybackIdProgress_Returns204(t *testing.T) {
	fc := &fakeLibrary{}
	h := newLibraryServer(t, authedFakeAuth(), fc)

	w := authedPut(t, h, "/playback/item-1/progress", `{"position_ms":5000,"played":false}`)

	if w.Code != http.StatusNoContent {
		t.Fatalf("status: got %d, want 204", w.Code)
	}
}

func TestPutPlaybackIdProgress_NegativePosition_Returns400(t *testing.T) {
	fc := &fakeLibrary{}
	h := newLibraryServer(t, authedFakeAuth(), fc)

	w := authedPut(t, h, "/playback/item-1/progress", `{"position_ms":-1,"played":false}`)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", w.Code)
	}
	e := decodeError(t, w)
	if e.Error.Code != gen.ValidationFailed {
		t.Errorf("code: got %q", e.Error.Code)
	}
}

func TestPutPlaybackIdProgress_Unauthorized_Returns401(t *testing.T) {
	fc := &fakeLibrary{}
	h := newLibraryServer(t, authedFakeAuth(), fc)

	req := httptest.NewRequest(http.MethodPut, "/playback/item-1/progress", bytes.NewBufferString(`{"position_ms":0,"played":false}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d, want 401", w.Code)
	}
}

func TestPutPlaybackIdProgress_ItemNotFound_Returns404(t *testing.T) {
	fc := &fakeLibrary{progressErr: media.ErrItemNotFound}
	h := newLibraryServer(t, authedFakeAuth(), fc)

	w := authedPut(t, h, "/playback/missing/progress", `{"position_ms":0,"played":false}`)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want 404", w.Code)
	}
	e := decodeError(t, w)
	if e.Error.Code != gen.ItemNotFound {
		t.Errorf("code: got %q", e.Error.Code)
	}
}

func TestPutPlaybackIdProgress_UpstreamError_Returns503(t *testing.T) {
	fc := &fakeLibrary{progressErr: errors.New("upstream timeout")}
	h := newLibraryServer(t, authedFakeAuth(), fc)

	w := authedPut(t, h, "/playback/item-1/progress", `{"position_ms":0,"played":false}`)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: got %d, want 503", w.Code)
	}
	e := decodeError(t, w)
	if e.Error.Code != gen.BackendUnavailable {
		t.Errorf("code: got %q", e.Error.Code)
	}
}
