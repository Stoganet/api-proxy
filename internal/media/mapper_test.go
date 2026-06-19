package media

import (
	"testing"

	"github.com/Stoganet/api-proxy/internal/clients/jellyfin"
)

func TestToDetail_MovieWithTMDB_BuildsCorrectShape(t *testing.T) {
	item := jellyfin.Item{
		ID:              "jf-abc",
		Name:            "The Matrix",
		Type:            jellyfin.ItemTypeMovie,
		Year:            1999,
		Overview:        "A hacker discovers reality is a simulation.",
		Genres:          []string{"Action", "Sci-Fi"},
		Runtime:         81_600_000_000, // 136 min in ticks
		PrimaryImageTag: "tag1",
		BackdropTags:    []string{"btag1"},
		ProviderIDs:     map[string]string{"Tmdb": "603"},
		People: []jellyfin.Person{
			{Name: "Keanu Reeves", Role: "Actor"},
			{Name: "Lana Wachowski", Role: "Director"},
		},
	}

	got := toDetail(item, "https://jf.example.com", "https://api.stoganet.com")

	fields := []struct {
		name string
		got  any
		want any
	}{
		{"ID", got.ID, "tmdb:movie:603"},
		{"Title", got.Title, "The Matrix"},
		{"Year", got.Year, 1999},
		{"Type", got.Type, TypeMovie},
		{"State", got.State, StatePlayable},
		{"Overview", got.Overview, "A hacker discovers reality is a simulation."},
		{"Runtime", got.Runtime, 136},
		{"Poster", got.Poster, "https://jf.example.com/Items/jf-abc/Images/Primary"},
		{"Backdrop", got.Backdrop, "https://jf.example.com/Items/jf-abc/Images/Backdrop/0"},
	}
	for _, f := range fields {
		if f.got != f.want {
			t.Errorf("%s: got %v, want %v", f.name, f.got, f.want)
		}
	}

	if len(got.Seasons) != 0 {
		t.Errorf("movie Seasons must be empty, got %d", len(got.Seasons))
	}
	if len(got.Genres) != 2 || got.Genres[0] != "Action" {
		t.Errorf("Genres: got %v", got.Genres)
	}
	if len(got.Cast) != 2 || got.Cast[0].Name != "Keanu Reeves" || got.Cast[0].Role != "Actor" {
		t.Errorf("Cast: got %v", got.Cast)
	}
	if got.Play == nil {
		t.Fatal("Play must not be nil for playable item")
	}
	wantStreamURL := "https://api.stoganet.com/stream/jf-abc"
	if got.Play.StreamURL != wantStreamURL {
		t.Errorf("Play.StreamURL: got %q, want %q", got.Play.StreamURL, wantStreamURL)
	}
}

func TestToDetail_SeriesNoTMDB_FallsBackToJFID(t *testing.T) {
	item := jellyfin.Item{
		ID:   "jf-xyz",
		Name: "Home Video",
		Type: jellyfin.ItemTypeSeries,
		Year: 2020,
	}

	got := toDetail(item, "https://jf.example.com", "https://api.stoganet.com")

	if got.ID != "jf:jf-xyz" {
		t.Errorf("ID: got %q, want %q", got.ID, "jf:jf-xyz")
	}
	if got.Type != TypeTV {
		t.Errorf("Type: got %q, want %q", got.Type, TypeTV)
	}
	if got.Backdrop != "" {
		t.Errorf("Backdrop should be empty when no BackdropTags, got %q", got.Backdrop)
	}
	if len(got.Seasons) != 0 {
		t.Errorf("toDetail Seasons must be empty slice, got %d", len(got.Seasons))
	}
}

func TestToDetail_TVShowWithTMDB_BuildsCorrectID(t *testing.T) {
	item := jellyfin.Item{
		ID:          "jf-tv-1",
		Name:        "Breaking Bad",
		Type:        jellyfin.ItemTypeSeries,
		ProviderIDs: map[string]string{"Tmdb": "1396"},
	}

	got := toDetail(item, "https://jf.example.com", "https://api.stoganet.com")

	if got.ID != "tmdb:tv:1396" {
		t.Errorf("ID: got %q, want %q", got.ID, "tmdb:tv:1396")
	}
}

func TestToItem_MovieWithTMDB_BuildsCorrectShape(t *testing.T) {
	item := jellyfin.Item{
		ID:              "jf-abc",
		Name:            "The Matrix",
		Type:            jellyfin.ItemTypeMovie,
		Year:            1999,
		PrimaryImageTag: "tag1",
		ProviderIDs:     map[string]string{"Tmdb": "603"},
	}

	got := toItem(item, "https://jf.example.com")

	fields := []struct {
		name string
		got  any
		want any
	}{
		{"ID", got.ID, "tmdb:movie:603"},
		{"Title", got.Title, "The Matrix"},
		{"Year", got.Year, 1999},
		{"Type", got.Type, TypeMovie},
		{"State", got.State, StatePlayable},
		{"Poster", got.Poster, "https://jf.example.com/Items/jf-abc/Images/Primary"},
	}
	for _, f := range fields {
		if f.got != f.want {
			t.Errorf("%s: got %v, want %v", f.name, f.got, f.want)
		}
	}
}

func TestToWatchProgress_NeverWatched_ReturnsNil(t *testing.T) {
	ud := jellyfin.UserData{PlaybackPositionTicks: 0, Played: false}
	if got := toWatchProgress(ud); got != nil {
		t.Errorf("expected nil for unwatched, got %+v", got)
	}
}

func TestToWatchProgress_InProgress_ReturnsMSConversion(t *testing.T) {
	ud := jellyfin.UserData{PlaybackPositionTicks: 4_120_000_000, Played: false}
	got := toWatchProgress(ud)
	if got == nil {
		t.Fatal("expected non-nil")
	}
	if got.PositionMS != 412_000 {
		t.Errorf("PositionMS: got %d, want 412000", got.PositionMS)
	}
	if got.Played {
		t.Error("Played should be false")
	}
}

func TestToWatchProgress_Played_ReturnsPlayed(t *testing.T) {
	ud := jellyfin.UserData{PlaybackPositionTicks: 0, Played: true}
	got := toWatchProgress(ud)
	if got == nil {
		t.Fatal("expected non-nil for played content")
	}
	if !got.Played {
		t.Error("Played should be true")
	}
}

func TestToSeason_MapsFieldsCorrectly(t *testing.T) {
	jfs := jellyfin.Season{
		ID: "s1", Number: 2, Name: "Season 2", Year: 2009,
		EpisodeCount: 13, PrimaryImageTag: "tag",
	}
	got := toSeason(jfs, "http://jf.example.com")
	if got.Number != 2 || got.Name != "Season 2" || got.EpisodeCount != 13 {
		t.Errorf("season mismatch: %+v", got)
	}
	if got.Poster != "http://jf.example.com/Items/s1/Images/Primary" {
		t.Errorf("poster: got %q", got.Poster)
	}
}

func TestToSeason_NoImage_EmptyPoster(t *testing.T) {
	jfs := jellyfin.Season{ID: "s1", Number: 1, Name: "Season 1", PrimaryImageTag: ""}
	got := toSeason(jfs, "http://jf.example.com")
	if got.Poster != "" {
		t.Errorf("expected empty poster, got %q", got.Poster)
	}
}

func TestToEpisode_MapsFieldsCorrectly(t *testing.T) {
	jfe := jellyfin.Episode{
		ID: "ep1", Name: "Pilot", IndexNumber: 1, ParentIndexNumber: 1,
		Overview: "The beginning.", RunTimeTicks: 17_640_000_000,
		PrimaryImageTag: "tag",
		UserData:        jellyfin.UserData{PlaybackPositionTicks: 0, Played: false},
	}
	got := toEpisode(jfe, "http://jf.example.com", "https://api.stoganet.com")
	if got.ID != "jf:ep1" {
		t.Errorf("ID: got %q, want jf:ep1", got.ID)
	}
	if got.Number != 1 || got.SeasonNumber != 1 {
		t.Errorf("numbers: %+v", got)
	}
	if got.Runtime != 29 { // 17_640_000_000 / 600_000_000 = 29.4 → 29
		t.Errorf("runtime: got %d, want 29", got.Runtime)
	}
	if got.Thumbnail != "http://jf.example.com/Items/ep1/Images/Primary" {
		t.Errorf("thumbnail: got %q", got.Thumbnail)
	}
	if got.Play == nil || got.Play.StreamURL != "https://api.stoganet.com/stream/ep1" {
		t.Errorf("play: %+v", got.Play)
	}
	if got.Progress != nil {
		t.Errorf("progress should be nil for unwatched episode")
	}
}

func TestToDetail_Movie_HasPlayAndProgress(t *testing.T) {
	jf := jellyfin.Item{
		ID: "mov1", Name: "The Matrix", Type: jellyfin.ItemTypeMovie,
		Year: 1999, Runtime: 81_600_000_000,
		UserData: jellyfin.UserData{PlaybackPositionTicks: 2_400_000_000, Played: false},
	}
	d := toDetail(jf, "http://jf.example.com", "https://api.stoganet.com")
	if d.Play == nil {
		t.Error("movie must have Play")
	}
	if d.Progress == nil || d.Progress.PositionMS != 240_000 {
		t.Errorf("progress: %+v", d.Progress)
	}
	if d.Resume != nil {
		t.Error("movie must not have Resume")
	}
	if len(d.Seasons) != 0 {
		t.Errorf("movie seasons must be empty slice, got %v", d.Seasons)
	}
}

func TestToSeriesDetail_HasSeasonsAndResume(t *testing.T) {
	jf := jellyfin.Item{
		ID: "tv1", Name: "Breaking Bad", Type: jellyfin.ItemTypeSeries,
	}
	seasons := []jellyfin.Season{
		{ID: "s1", Number: 1, Name: "Season 1", Year: 2008, EpisodeCount: 7},
	}
	nextUp := &jellyfin.Episode{
		ID: "ep3", Name: "Pilot", IndexNumber: 1, ParentIndexNumber: 1,
		UserData: jellyfin.UserData{PlaybackPositionTicks: 4_120_000_000, Played: false},
	}
	d := toSeriesDetail(jf, seasons, nextUp, "http://jf.example.com", "https://api.stoganet.com")
	if d.Play != nil {
		t.Error("series must not have Play")
	}
	if d.Progress != nil {
		t.Error("series must not have Progress")
	}
	if len(d.Seasons) != 1 {
		t.Errorf("seasons: got %d", len(d.Seasons))
	}
	if d.Resume == nil {
		t.Fatal("Resume must not be nil")
	}
	if d.Resume.EpisodeID != "jf:ep3" {
		t.Errorf("Resume.EpisodeID: got %q", d.Resume.EpisodeID)
	}
	if d.Resume.Progress.PositionMS != 412_000 {
		t.Errorf("Resume.Progress.PositionMS: got %d", d.Resume.Progress.PositionMS)
	}
}

func TestToSeriesDetail_NoNextUp_NilResume(t *testing.T) {
	jf := jellyfin.Item{ID: "tv1", Name: "Breaking Bad", Type: jellyfin.ItemTypeSeries}
	d := toSeriesDetail(jf, nil, nil, "http://jf.example.com", "https://api.stoganet.com")
	if d.Resume != nil {
		t.Errorf("Resume should be nil for unwatched series, got %+v", d.Resume)
	}
}
