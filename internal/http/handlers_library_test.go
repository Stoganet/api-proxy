package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Stoganet/api-proxy/internal/auth"
	"github.com/Stoganet/api-proxy/internal/gen"
	"github.com/Stoganet/api-proxy/internal/media"
)

type fakeLibrary struct {
	detail      *media.Detail
	detailErr   error
	list        *media.ListResult
	listErr     error
	home        *media.HomeResult
	homeErr     error
	episodes    []media.Episode
	episodesErr error
}

func (f *fakeLibrary) GetItem(_ context.Context, _, _ string) (*media.Detail, error) {
	return f.detail, f.detailErr
}
func (f *fakeLibrary) List(_ context.Context, _ string, _ media.ListOpts) (*media.ListResult, error) {
	return f.list, f.listErr
}
func (f *fakeLibrary) Home(_ context.Context, _ string) (*media.HomeResult, error) {
	return f.home, f.homeErr
}
func (f *fakeLibrary) GetEpisodes(_ context.Context, _, _ string, _ int) ([]media.Episode, error) {
	return f.episodes, f.episodesErr
}

func newLibraryServer(t *testing.T, fa *fakeAuth, fc *fakeLibrary) http.Handler {
	t.Helper()
	s := &Server{auth: fa, library: fc}
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

func TestGetLibraryId_Returns200WithDetail(t *testing.T) {
	fc := &fakeLibrary{detail: &media.Detail{
		Item: media.Item{
			ID:    "tmdb:movie:603",
			Title: "The Matrix",
			Year:  1999,
			Type:  media.TypeMovie,
			State: media.StatePlayable,
		},
		Runtime: 136,
		Genres:  []string{"Action"},
		Seasons: []media.Season{},
		Play: &media.PlayInfo{
			StreamURL: "https://api.stoganet.com/stream/jf-abc",
		},
	}}

	h := newLibraryServer(t, authedFakeAuth(), fc)
	w := authedGet(t, h, "/library/tmdb:movie:603")

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200. body: %s", w.Code, w.Body.String())
	}
	var resp gen.LibraryDetail
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

func TestGetLibraryId_NotFound_Returns404(t *testing.T) {
	fc := &fakeLibrary{detailErr: media.ErrItemNotFound}

	h := newLibraryServer(t, authedFakeAuth(), fc)
	w := authedGet(t, h, "/library/missing")

	if w.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want 404", w.Code)
	}
	e := decodeError(t, w)
	if e.Error.Code != gen.ItemNotFound {
		t.Errorf("code: got %q", e.Error.Code)
	}
}

func TestGetLibrary_Returns200WithList(t *testing.T) {
	fc := &fakeLibrary{list: &media.ListResult{
		Items: []media.Item{
			{ID: "tmdb:movie:1", Title: "Movie A", Type: media.TypeMovie, State: media.StatePlayable},
			{ID: "tmdb:movie:2", Title: "Movie B", Type: media.TypeMovie, State: media.StatePlayable},
		},
		Total:      50,
		NextCursor: "2",
	}}

	h := newLibraryServer(t, authedFakeAuth(), fc)
	w := authedGet(t, h, "/library")

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d. body: %s", w.Code, w.Body.String())
	}
	var resp gen.LibraryListResponse
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

func TestGetLibraryId_NoAuth_Returns401(t *testing.T) {
	fa := &fakeAuth{}
	h := newLibraryServer(t, fa, &fakeLibrary{})

	req := httptest.NewRequest(http.MethodGet, "/library/some-id", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d, want 401", w.Code)
	}
}

func TestGetLibraryIdSeasonsSeasonNumberEpisodes_Returns200(t *testing.T) {
	thumbnail := "https://jf.example.com/Items/ep1/Images/Primary"
	fc := &fakeLibrary{
		episodes: []media.Episode{
			{
				ID: "jf:ep1", Number: 1, SeasonNumber: 1,
				Title: "Pilot", Overview: "The start.", Runtime: 47,
				Thumbnail: thumbnail,
				State:     media.StatePlayable,
				Play:      &media.PlayInfo{StreamURL: "https://api.stoganet.com/stream/ep1"},
				Progress:  &media.WatchProgress{PositionMS: 0, Played: false},
			},
		},
	}
	h := newLibraryServer(t, authedFakeAuth(), fc)
	w := authedGet(t, h, "/library/tmdb:tv:1396/seasons/1/episodes")

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200. body: %s", w.Code, w.Body.String())
	}
	var resp gen.EpisodeListResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Episodes) != 1 {
		t.Fatalf("episodes: got %d", len(resp.Episodes))
	}
	if resp.Episodes[0].Id != "jf:ep1" || resp.Episodes[0].Title != "Pilot" {
		t.Errorf("episode: %+v", resp.Episodes[0])
	}
}

func TestGetLibraryIdSeasonsSeasonNumberEpisodes_ShowNotFound_Returns404(t *testing.T) {
	fc := &fakeLibrary{episodesErr: media.ErrItemNotFound}
	h := newLibraryServer(t, authedFakeAuth(), fc)
	w := authedGet(t, h, "/library/jf:unknown/seasons/1/episodes")

	if w.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want 404", w.Code)
	}
	e := decodeError(t, w)
	if e.Error.Code != gen.ItemNotFound {
		t.Errorf("code: got %q", e.Error.Code)
	}
}

func TestGetLibraryIdSeasonsSeasonNumberEpisodes_UpstreamError_Returns503(t *testing.T) {
	fc := &fakeLibrary{episodesErr: errors.New("upstream timeout")}
	h := newLibraryServer(t, authedFakeAuth(), fc)
	w := authedGet(t, h, "/library/tmdb:tv:1396/seasons/1/episodes")

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: got %d, want 503", w.Code)
	}
	e := decodeError(t, w)
	if e.Error.Code != gen.BackendUnavailable {
		t.Errorf("code: got %q", e.Error.Code)
	}
}

func TestGetLibraryId_SeriesDetail_HasSeasonsAndResume(t *testing.T) {
	fc := &fakeLibrary{detail: &media.Detail{
		Item: media.Item{
			ID: "tmdb:tv:1396", Title: "Breaking Bad",
			Year: 2008, Type: media.TypeTV, State: media.StatePlayable,
		},
		Runtime: 0,
		Genres:  []string{"Drama"},
		Seasons: []media.Season{
			{Number: 1, Name: "Season 1", Year: 2008, EpisodeCount: 7, Poster: "https://jf.example.com/Items/s1/Images/Primary"},
		},
		Resume: &media.ResumeInfo{
			SeasonNumber: 1, EpisodeNumber: 3, EpisodeID: "jf:ep3",
			Title:    "Bit by a Dead Bee",
			Play:     media.PlayInfo{StreamURL: "https://api.stoganet.com/stream/ep3"},
			Progress: media.WatchProgress{PositionMS: 412_000, Played: false},
		},
	}}

	h := newLibraryServer(t, authedFakeAuth(), fc)
	w := authedGet(t, h, "/library/tmdb:tv:1396")

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d. body: %s", w.Code, w.Body.String())
	}
	var resp gen.LibraryDetail
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Seasons) != 1 || resp.Seasons[0].Number != 1 {
		t.Errorf("seasons: %+v", resp.Seasons)
	}
	if resp.Resume == nil || resp.Resume.EpisodeId != "jf:ep3" {
		t.Errorf("resume: %+v", resp.Resume)
	}
	if resp.Play != nil {
		t.Errorf("series must not have Play, got %+v", resp.Play)
	}
}

func TestGetLibraryId_MovieDetail_HasPlayAndProgress(t *testing.T) {
	fc := &fakeLibrary{detail: &media.Detail{
		Item: media.Item{
			ID: "tmdb:movie:603", Title: "The Matrix",
			Year: 1999, Type: media.TypeMovie, State: media.StatePlayable,
		},
		Runtime:  136,
		Genres:   []string{"Action"},
		Seasons:  []media.Season{},
		Play:     &media.PlayInfo{StreamURL: "https://api.stoganet.com/stream/mov1"},
		Progress: &media.WatchProgress{PositionMS: 240_000, Played: false},
	}}

	h := newLibraryServer(t, authedFakeAuth(), fc)
	w := authedGet(t, h, "/library/tmdb:movie:603")

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d. body: %s", w.Code, w.Body.String())
	}
	var resp gen.LibraryDetail
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Play == nil || resp.Play.StreamUrl != "https://api.stoganet.com/stream/mov1" {
		t.Errorf("play: %+v", resp.Play)
	}
	if resp.Progress == nil || resp.Progress.PositionMs != 240_000 {
		t.Errorf("progress: %+v", resp.Progress)
	}
	if resp.Resume != nil {
		t.Errorf("movie must not have resume, got %+v", resp.Resume)
	}
}
