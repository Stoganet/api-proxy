package media

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/Stoganet/api-proxy/internal/clients/jellyfin"
)

type fakeJF struct {
	item               *jellyfin.Item
	items              *jellyfin.ItemsResult
	err                error
	capturedItemID     string
	capturedOpts       jellyfin.GetItemsOpts
	getSeasons         []jellyfin.Season
	getSeasonsErr      error
	getEpisodes        []jellyfin.Episode
	getEpisodesErr     error
	getNextUp          *jellyfin.Episode
	getNextUpErr       error
	getFirstEpisode    *jellyfin.Episode
	getFirstEpisodeErr error
}

func (f *fakeJF) GetItem(_ context.Context, _, itemID string) (*jellyfin.Item, error) {
	f.capturedItemID = itemID
	return f.item, f.err
}

func (f *fakeJF) GetItems(_ context.Context, _ string, opts jellyfin.GetItemsOpts) (*jellyfin.ItemsResult, error) {
	f.capturedOpts = opts
	return f.items, f.err
}

func (f *fakeJF) GetSeasons(_ context.Context, _, _ string) ([]jellyfin.Season, error) {
	return f.getSeasons, f.getSeasonsErr
}

func (f *fakeJF) GetEpisodes(_ context.Context, _, _ string, _ int) ([]jellyfin.Episode, error) {
	return f.getEpisodes, f.getEpisodesErr
}

func (f *fakeJF) GetNextUp(_ context.Context, _, _ string) (*jellyfin.Episode, error) {
	return f.getNextUp, f.getNextUpErr
}

func (f *fakeJF) GetFirstEpisode(_ context.Context, _, _ string) (*jellyfin.Episode, error) {
	return f.getFirstEpisode, f.getFirstEpisodeErr
}

func newSvc(jf JellyfinClient) *Service {
	return NewService(jf, "https://jf.example.com", "https://api.stoganet.com", slog.Default())
}

func TestService_GetItem_JFPrefix_StripsPrefix(t *testing.T) {
	jf := &fakeJF{item: &jellyfin.Item{
		ID:   "abc-uuid",
		Name: "Home Video",
		Type: "Movie",
	}}
	svc := newSvc(jf)
	_, err := svc.GetItem(context.Background(), "jf-user-1", "jf:abc-uuid")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if jf.capturedItemID != "abc-uuid" {
		t.Errorf("capturedItemID: got %q, want %q", jf.capturedItemID, "abc-uuid")
	}
}

func TestService_GetItem_TMDBPrefix_UsesProviderIDSearch(t *testing.T) {
	jf := &fakeJF{items: &jellyfin.ItemsResult{
		Items: []jellyfin.Item{{
			ID:          "abc-uuid",
			Name:        "The Matrix",
			Type:        "Movie",
			ProviderIDs: map[string]string{"Tmdb": "603"},
		}},
		TotalCount: 1,
	}}
	svc := newSvc(jf)
	d, err := svc.GetItem(context.Background(), "jf-user-1", "tmdb:movie:603")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if jf.capturedOpts.ProviderID != "Tmdb.603" {
		t.Errorf("ProviderID filter: got %q, want %q", jf.capturedOpts.ProviderID, "Tmdb.603")
	}
	if d.ID != "tmdb:movie:603" {
		t.Errorf("catalog ID: got %q", d.ID)
	}
}

func TestService_GetItem_TMDBPrefix_NotInJellyfin_ReturnsNotFound(t *testing.T) {
	jf := &fakeJF{items: &jellyfin.ItemsResult{Items: []jellyfin.Item{}, TotalCount: 0}}
	svc := newSvc(jf)
	_, err := svc.GetItem(context.Background(), "jf-user-1", "tmdb:movie:999")
	if !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("want ErrItemNotFound, got %v", err)
	}
}

func TestService_GetItem_InvalidID_ReturnsNotFound(t *testing.T) {
	svc := newSvc(&fakeJF{})
	_, err := svc.GetItem(context.Background(), "jf-user-1", "not-a-valid-id")
	if !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("want ErrItemNotFound, got %v", err)
	}
}

func TestService_GetItem_PropagatesNotFound(t *testing.T) {
	jf := &fakeJF{err: jellyfin.ErrItemNotFound}
	svc := newSvc(jf)
	_, err := svc.GetItem(context.Background(), "jf-user-1", "jf:missing-uuid")
	if !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("want ErrItemNotFound, got %v", err)
	}
}

func TestService_GetItem_ReturnsDetail(t *testing.T) {
	jf := &fakeJF{item: &jellyfin.Item{
		ID:          "jf-1",
		Name:        "The Matrix",
		Type:        "Movie",
		Year:        1999,
		ProviderIDs: map[string]string{"Tmdb": "603"},
		Runtime:     81_600_000_000,
	}}
	svc := newSvc(jf)
	d, err := svc.GetItem(context.Background(), "jf-user-1", "jf:jf-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.ID != "tmdb:movie:603" {
		t.Errorf("ID: got %q", d.ID)
	}
	if d.Play == nil || d.Play.StreamURL != "https://api.stoganet.com/stream/jf-1" {
		t.Errorf("Play.StreamURL: got %q", d.Play.StreamURL)
	}
}

func TestService_GetItem_Series_WithFirstEpisode_PopulatesStart(t *testing.T) {
	jf := &fakeJF{
		item: &jellyfin.Item{
			ID:   "series-1",
			Name: "Breaking Bad",
			Type: jellyfin.ItemTypeSeries,
		},
		getFirstEpisode: &jellyfin.Episode{
			ID: "ep1", Name: "Pilot", IndexNumber: 1, ParentIndexNumber: 1,
		},
	}
	svc := newSvc(jf)
	d, err := svc.GetItem(context.Background(), "jf-user-1", "jf:series-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Start == nil {
		t.Fatal("Start must not be nil when firstEpisode returned")
	}
	if d.Start.EpisodeID != "jf:ep1" {
		t.Errorf("Start.EpisodeID: got %q", d.Start.EpisodeID)
	}
	if d.Start.Progress.PositionMS != 0 || d.Start.Progress.Played {
		t.Errorf("Start.Progress must be zeroed, got %+v", d.Start.Progress)
	}
}

func TestService_List_ReturnsPaginatedResult(t *testing.T) {
	jf := &fakeJF{items: &jellyfin.ItemsResult{
		Items: []jellyfin.Item{
			{ID: "jf-1", Name: "Movie A", Type: "Movie", ProviderIDs: map[string]string{"Tmdb": "1"}},
			{ID: "jf-2", Name: "Movie B", Type: "Movie", ProviderIDs: map[string]string{"Tmdb": "2"}},
		},
		TotalCount: 50,
		StartIndex: 0,
	}}

	svc := newSvc(jf)
	res, err := svc.List(context.Background(), "jf-user-1", ListOpts{Limit: 2, StartIndex: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Items) != 2 {
		t.Errorf("Items: got %d", len(res.Items))
	}
	if res.Total != 50 {
		t.Errorf("Total: got %d", res.Total)
	}
	if res.NextCursor != "2" {
		t.Errorf("NextCursor: got %q, want %q", res.NextCursor, "2")
	}
}

func TestService_List_EmptyNextCursorOnLastPage(t *testing.T) {
	jf := &fakeJF{items: &jellyfin.ItemsResult{
		Items:      []jellyfin.Item{{ID: "jf-1", Type: "Movie"}},
		TotalCount: 1,
		StartIndex: 0,
	}}

	svc := newSvc(jf)
	res, err := svc.List(context.Background(), "jf-user-1", ListOpts{Limit: 40})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.NextCursor != "" {
		t.Errorf("NextCursor should be empty on last page, got %q", res.NextCursor)
	}
}

// fakeJFFunc allows per-call control of GetItems for Home() fan-out tests.
type fakeJFFunc struct {
	fn func(opts jellyfin.GetItemsOpts) (*jellyfin.ItemsResult, error)
}

func (f *fakeJFFunc) GetItem(_ context.Context, _, _ string) (*jellyfin.Item, error) {
	return nil, jellyfin.ErrItemNotFound
}

func (f *fakeJFFunc) GetItems(_ context.Context, _ string, opts jellyfin.GetItemsOpts) (*jellyfin.ItemsResult, error) {
	return f.fn(opts)
}

func (f *fakeJFFunc) GetSeasons(_ context.Context, _, _ string) ([]jellyfin.Season, error) {
	return nil, nil
}

func (f *fakeJFFunc) GetEpisodes(_ context.Context, _, _ string, _ int) ([]jellyfin.Episode, error) {
	return nil, nil
}

func (f *fakeJFFunc) GetNextUp(_ context.Context, _, _ string) (*jellyfin.Episode, error) {
	return nil, nil
}

func (f *fakeJFFunc) GetFirstEpisode(_ context.Context, _, _ string) (*jellyfin.Episode, error) {
	return nil, nil
}

func okSection() *jellyfin.ItemsResult {
	return &jellyfin.ItemsResult{
		Items:      []jellyfin.Item{{ID: "jf-1", Type: jellyfin.ItemTypeMovie}},
		TotalCount: 1,
	}
}

func TestService_Home_AllSectionsSucceed(t *testing.T) {
	jf := &fakeJFFunc{fn: func(_ jellyfin.GetItemsOpts) (*jellyfin.ItemsResult, error) {
		return okSection(), nil
	}}
	res, err := newSvc(jf).Home(context.Background(), "uid")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Sections) != len(homeSections) {
		t.Errorf("sections: got %d, want %d", len(res.Sections), len(homeSections))
	}
}

func TestService_Home_OneSectionFails_OmitsIt(t *testing.T) {
	failOpts := homeSections[0].opts
	jf := &fakeJFFunc{fn: func(opts jellyfin.GetItemsOpts) (*jellyfin.ItemsResult, error) {
		if opts == failOpts {
			return nil, errors.New("upstream error")
		}
		return okSection(), nil
	}}
	res, err := newSvc(jf).Home(context.Background(), "uid")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Sections) != len(homeSections)-1 {
		t.Errorf("sections: got %d, want %d", len(res.Sections), len(homeSections)-1)
	}
}

func TestService_Home_AllSectionsFail_ReturnsError(t *testing.T) {
	jf := &fakeJFFunc{fn: func(_ jellyfin.GetItemsOpts) (*jellyfin.ItemsResult, error) {
		return nil, errors.New("upstream error")
	}}
	_, err := newSvc(jf).Home(context.Background(), "uid")
	if err == nil {
		t.Fatal("expected error when all sections fail")
	}
}

func TestService_Home_HasMore_TrueWhenTotalExceedsReturned(t *testing.T) {
	jf := &fakeJFFunc{fn: func(_ jellyfin.GetItemsOpts) (*jellyfin.ItemsResult, error) {
		return &jellyfin.ItemsResult{
			Items:      []jellyfin.Item{{ID: "jf-1", Type: jellyfin.ItemTypeMovie}},
			TotalCount: 100,
		}, nil
	}}
	res, err := newSvc(jf).Home(context.Background(), "uid")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, sec := range res.Sections {
		if !sec.HasMore {
			t.Errorf("section %q: HasMore should be true when TotalCount > len(Items)", sec.ID)
		}
	}
}

func TestService_Home_HasMore_FalseWhenAllReturned(t *testing.T) {
	jf := &fakeJFFunc{fn: func(_ jellyfin.GetItemsOpts) (*jellyfin.ItemsResult, error) {
		return okSection(), nil
	}}
	res, err := newSvc(jf).Home(context.Background(), "uid")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, sec := range res.Sections {
		if sec.HasMore {
			t.Errorf("section %q: HasMore should be false when TotalCount == len(Items)", sec.ID)
		}
	}
}

func TestGetItem_Series_ReturnsSeasonsAndResume(t *testing.T) {
	jf := &fakeJF{
		item: &jellyfin.Item{ID: "tv1", Name: "Breaking Bad", Type: jellyfin.ItemTypeSeries},
		getSeasons: []jellyfin.Season{
			{ID: "s1", Number: 1, Name: "Season 1", Year: 2008, EpisodeCount: 7},
		},
		getNextUp: &jellyfin.Episode{
			ID: "ep3", Name: "Bit by a Dead Bee",
			IndexNumber: 3, ParentIndexNumber: 2,
			UserData: jellyfin.UserData{PlaybackPositionTicks: 4_120_000_000},
		},
	}
	svc := NewService(jf, "http://jf.example.com", "https://api.stoganet.com", slog.Default())
	d, err := svc.GetItem(context.Background(), "uid", "jf:tv1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Play != nil {
		t.Error("series must not have Play")
	}
	if len(d.Seasons) != 1 || d.Seasons[0].Number != 1 {
		t.Errorf("seasons: %+v", d.Seasons)
	}
	if d.Resume == nil || d.Resume.EpisodeID != "jf:ep3" {
		t.Errorf("resume: %+v", d.Resume)
	}
}

func TestGetItem_Movie_HasPlayAndProgress(t *testing.T) {
	jf := &fakeJF{
		item: &jellyfin.Item{
			ID: "mov1", Name: "The Matrix", Type: jellyfin.ItemTypeMovie,
			UserData: jellyfin.UserData{PlaybackPositionTicks: 2_400_000_000},
		},
	}
	svc := NewService(jf, "http://jf.example.com", "https://api.stoganet.com", slog.Default())
	d, err := svc.GetItem(context.Background(), "uid", "jf:mov1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Play == nil {
		t.Error("movie must have Play")
	}
	if d.Progress == nil || d.Progress.PositionMS != 240_000 {
		t.Errorf("progress: %+v", d.Progress)
	}
	if d.Resume != nil {
		t.Error("movie must not have Resume")
	}
}

func TestGetEpisodes_ReturnsMappedEpisodes(t *testing.T) {
	jf := &fakeJF{
		item: &jellyfin.Item{ID: "tv1", Type: jellyfin.ItemTypeSeries},
		getEpisodes: []jellyfin.Episode{
			{ID: "ep1", Name: "Pilot", IndexNumber: 1, ParentIndexNumber: 1,
				RunTimeTicks: 17_640_000_000},
		},
	}
	svc := NewService(jf, "http://jf.example.com", "https://api.stoganet.com", slog.Default())
	eps, err := svc.GetEpisodes(context.Background(), "uid", "jf:tv1", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(eps) != 1 || eps[0].ID != "jf:ep1" {
		t.Errorf("episodes: %+v", eps)
	}
	if eps[0].Play == nil || eps[0].Play.StreamURL != "https://api.stoganet.com/stream/ep1" {
		t.Errorf("play: %+v", eps[0].Play)
	}
}

func TestGetEpisodes_JellyfinSeriesNotFound_ReturnsErrItemNotFound(t *testing.T) {
	jf := &fakeJF{
		item:           &jellyfin.Item{ID: "tv1", Type: jellyfin.ItemTypeSeries},
		getEpisodesErr: jellyfin.ErrItemNotFound,
	}
	svc := NewService(jf, "http://jf.example.com", "https://api.stoganet.com", slog.Default())
	_, err := svc.GetEpisodes(context.Background(), "uid", "jf:tv1", 99)
	if !errors.Is(err, ErrItemNotFound) {
		t.Errorf("got %v, want ErrItemNotFound", err)
	}
}

func TestGetEpisodes_EmptyResult_ReturnsEmptySlice(t *testing.T) {
	jf := &fakeJF{
		item:        &jellyfin.Item{ID: "tv1", Type: jellyfin.ItemTypeSeries},
		getEpisodes: []jellyfin.Episode{},
	}
	svc := NewService(jf, "http://jf.example.com", "https://api.stoganet.com", slog.Default())
	eps, err := svc.GetEpisodes(context.Background(), "uid", "jf:tv1", 99)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(eps) != 0 {
		t.Errorf("expected empty slice, got %d", len(eps))
	}
}
