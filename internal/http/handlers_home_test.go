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

func TestGetHome_Returns200WithSections(t *testing.T) {
	fc := &fakeLibrary{home: &media.HomeResult{
		Sections: []media.HomeSection{
			{
				ID:      "recently_added_movies",
				Items:   []media.Item{{ID: "tmdb:movie:1", Title: "Movie A", Type: media.TypeMovie, State: media.StatePlayable}},
				HasMore: true,
			},
			{
				ID:      "all_tv",
				Items:   []media.Item{{ID: "tmdb:tv:2", Title: "Show B", Type: media.TypeTV, State: media.StatePlayable}},
				HasMore: false,
			},
		},
	}}

	h := newLibraryServer(t, authedFakeAuth(), fc)
	w := authedGet(t, h, "/home")

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200. body: %s", w.Code, w.Body.String())
	}
	var resp gen.HomeResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Sections) != 2 {
		t.Fatalf("sections: got %d, want 2", len(resp.Sections))
	}
	if resp.Sections[0].Id != "recently_added_movies" {
		t.Errorf("section[0].id: got %q", resp.Sections[0].Id)
	}
	if !resp.Sections[0].HasMore {
		t.Errorf("section[0].has_more: want true")
	}
	if len(resp.Sections[0].Items) != 1 {
		t.Errorf("section[0].items: got %d, want 1", len(resp.Sections[0].Items))
	}
	if resp.Sections[1].HasMore {
		t.Errorf("section[1].has_more: want false")
	}
}

func TestGetHome_EmptySections_Returns200(t *testing.T) {
	fc := &fakeLibrary{home: &media.HomeResult{Sections: []media.HomeSection{}}}

	h := newLibraryServer(t, authedFakeAuth(), fc)
	w := authedGet(t, h, "/home")

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}
	var resp gen.HomeResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Sections) != 0 {
		t.Errorf("sections: got %d, want 0", len(resp.Sections))
	}
}

func TestGetHome_ServiceError_Returns503(t *testing.T) {
	fc := &fakeLibrary{homeErr: errors.New("all sections failed")}

	h := newLibraryServer(t, authedFakeAuth(), fc)
	w := authedGet(t, h, "/home")

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: got %d, want 503", w.Code)
	}
	e := decodeError(t, w)
	if e.Error.Code != gen.BackendUnavailable {
		t.Errorf("code: got %q", e.Error.Code)
	}
}

func TestGetHome_NoAuth_Returns401(t *testing.T) {
	h := newLibraryServer(t, &fakeAuth{}, &fakeLibrary{})

	req := httptest.NewRequest(http.MethodGet, "/home", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d, want 401", w.Code)
	}
}
