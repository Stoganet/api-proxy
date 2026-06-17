package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Stoganet/api-proxy/internal/auth"
	"github.com/Stoganet/api-proxy/internal/gen"
	"github.com/Stoganet/api-proxy/internal/media"
)

type fakeCatalog struct {
	detail    *media.Detail
	detailErr error
	list      *media.ListResult
	listErr   error
}

func (f *fakeCatalog) GetItem(_ context.Context, _, _, _ string) (*media.Detail, error) {
	return f.detail, f.detailErr
}
func (f *fakeCatalog) List(_ context.Context, _ string, _ media.ListOpts) (*media.ListResult, error) {
	return f.list, f.listErr
}

func newCatalogServer(t *testing.T, fa *fakeAuth, fc *fakeCatalog) http.Handler {
	t.Helper()
	s := &Server{auth: fa, catalog: fc}
	strict := gen.NewStrictHandlerWithOptions(s, []gen.StrictMiddlewareFunc{
		jwtStrictMiddleware(fa),
	}, gen.StrictHTTPServerOptions{})
	return gen.Handler(strict)
}

func authedGet(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func authedFakeAuth() *fakeAuth {
	return &fakeAuth{
		verifyOut: &auth.Claims{UserID: "u1", JFUserID: "jf-uid"},
		jfTok:     "jf-tok",
	}
}

func TestGetCatalogId_Returns200WithDetail(t *testing.T) {
	fc := &fakeCatalog{detail: &media.Detail{
		Item: media.Item{
			ID:    "tmdb:movie:603",
			Title: "The Matrix",
			Year:  1999,
			Type:  "movie",
			State: "playable",
		},
		Runtime: 136,
		Genres:  []string{"Action"},
		Play: &media.PlayInfo{
			JellyfinItemID:      "jf-abc",
			JellyfinBaseURL:     "https://jf.example.com",
			JellyfinAccessToken: "jf-tok",
			JellyfinUserID:      "jf-uid",
		},
	}}

	h := newCatalogServer(t, authedFakeAuth(), fc)
	w := authedGet(t, h, "/catalog/tmdb:movie:603")

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200. body: %s", w.Code, w.Body.String())
	}
	var resp gen.CatalogDetail
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Id != "tmdb:movie:603" {
		t.Errorf("ID: got %q", resp.Id)
	}
	if resp.Title != "The Matrix" {
		t.Errorf("Title: got %q", resp.Title)
	}
	if resp.Runtime != 136 {
		t.Errorf("Runtime: got %d", resp.Runtime)
	}
}

func TestGetCatalogId_NotFound_Returns404(t *testing.T) {
	fc := &fakeCatalog{detailErr: media.ErrItemNotFound}

	h := newCatalogServer(t, authedFakeAuth(), fc)
	w := authedGet(t, h, "/catalog/missing")

	if w.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want 404", w.Code)
	}
	e := decodeError(t, w)
	if e.Error.Code != gen.ItemNotFound {
		t.Errorf("code: got %q", e.Error.Code)
	}
}

func TestGetCatalog_Returns200WithList(t *testing.T) {
	fc := &fakeCatalog{list: &media.ListResult{
		Items: []media.Item{
			{ID: "tmdb:movie:1", Title: "Movie A", Type: "movie", State: "playable"},
			{ID: "tmdb:movie:2", Title: "Movie B", Type: "movie", State: "playable"},
		},
		Total:      50,
		NextCursor: "2",
	}}

	h := newCatalogServer(t, authedFakeAuth(), fc)
	w := authedGet(t, h, "/catalog")

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d. body: %s", w.Code, w.Body.String())
	}
	var resp gen.CatalogListResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Items) != 2 {
		t.Errorf("Items: got %d", len(resp.Items))
	}
	if resp.Total != 50 {
		t.Errorf("Total: got %d", resp.Total)
	}
}

func TestGetCatalogId_NoAuth_Returns401(t *testing.T) {
	fa := &fakeAuth{}
	h := newCatalogServer(t, fa, &fakeCatalog{})

	req := httptest.NewRequest(http.MethodGet, "/catalog/some-id", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d, want 401", w.Code)
	}
}
