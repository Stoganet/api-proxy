package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Stoganet/api-proxy/internal/gen"
	"github.com/Stoganet/api-proxy/internal/media"
)

func authedPost(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func TestGetSearch_Returns200WithItems(t *testing.T) {
	fc := &fakeLibrary{searchItems: []media.Item{
		{ID: "tmdb:movie:603", Title: "The Matrix", Type: media.TypeMovie, State: media.StateRequestable},
	}}
	h := newLibraryServer(t, authedFakeAuth(), fc)
	w := authedGet(t, h, "/search?q=matrix")

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200. body: %s", w.Code, w.Body.String())
	}
	if fc.capturedQ != "matrix" {
		t.Errorf("query: got %q, want %q", fc.capturedQ, "matrix")
	}
	var resp gen.SearchResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Items) != 1 || resp.Items[0].Id != "tmdb:movie:603" {
		t.Errorf("items: %+v", resp.Items)
	}
}

func TestGetSearch_EmptyQuery_Returns400(t *testing.T) {
	fc := &fakeLibrary{}
	h := newLibraryServer(t, authedFakeAuth(), fc)
	w := authedGet(t, h, "/search?q=")

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", w.Code)
	}
	e := decodeError(t, w)
	if e.Error.Code != gen.ValidationFailed {
		t.Errorf("code: got %q", e.Error.Code)
	}
}

func TestGetSearch_UpstreamError_Returns503(t *testing.T) {
	fc := &fakeLibrary{searchErr: errors.New("seerr down")}
	h := newLibraryServer(t, authedFakeAuth(), fc)
	w := authedGet(t, h, "/search?q=matrix")

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: got %d, want 503", w.Code)
	}
	e := decodeError(t, w)
	if e.Error.Code != gen.BackendUnavailable {
		t.Errorf("code: got %q", e.Error.Code)
	}
}

func TestGetSearch_NoAuth_Returns401(t *testing.T) {
	h := newLibraryServer(t, &fakeAuth{}, &fakeLibrary{})

	req := httptest.NewRequest(http.MethodGet, "/search?q=matrix", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d, want 401", w.Code)
	}
}

func TestPostLibraryIdRequest_Returns204(t *testing.T) {
	fc := &fakeLibrary{}
	h := newLibraryServer(t, authedFakeAuth(), fc)
	w := authedPost(t, h, "/library/tmdb:movie:603/request")

	if w.Code != http.StatusNoContent {
		t.Fatalf("status: got %d, want 204. body: %s", w.Code, w.Body.String())
	}
	if fc.capturedID != "tmdb:movie:603" {
		t.Errorf("catalogID: got %q", fc.capturedID)
	}
}

func TestPostLibraryIdRequest_NotRequestable_Returns400(t *testing.T) {
	fc := &fakeLibrary{requestErr: media.ErrNotRequestable}
	h := newLibraryServer(t, authedFakeAuth(), fc)
	w := authedPost(t, h, "/library/jf:abc/request")

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", w.Code)
	}
	e := decodeError(t, w)
	if e.Error.Code != gen.ValidationFailed {
		t.Errorf("code: got %q", e.Error.Code)
	}
}

func TestPostLibraryIdRequest_UpstreamError_Returns503(t *testing.T) {
	fc := &fakeLibrary{requestErr: errors.New("seerr down")}
	h := newLibraryServer(t, authedFakeAuth(), fc)
	w := authedPost(t, h, "/library/tmdb:movie:603/request")

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: got %d, want 503", w.Code)
	}
	e := decodeError(t, w)
	if e.Error.Code != gen.BackendUnavailable {
		t.Errorf("code: got %q", e.Error.Code)
	}
}

func TestPostLibraryIdRequest_NoAuth_Returns401(t *testing.T) {
	h := newLibraryServer(t, &fakeAuth{}, &fakeLibrary{})

	req := httptest.NewRequest(http.MethodPost, "/library/tmdb:movie:603/request", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d, want 401", w.Code)
	}
}
