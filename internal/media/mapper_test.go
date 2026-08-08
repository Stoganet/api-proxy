package media

import (
	"testing"

	"github.com/Stoganet/api-proxy/internal/clients/jellyfin"
	"github.com/Stoganet/api-proxy/internal/clients/seerr"
)

func TestToDetail_MovieWithTMDB_BuildsCorrectShape(t *testing.T) {
	item := jellyfin.Item{
		ID:              "jf-abc",
		Name:            "Test Movie",
		Type:            jellyfin.ItemTypeMovie,
		Year:            1999,
		Overview:        "A hacker discovers reality is a simulation.",
		Genres:          []string{"Action", "Sci-Fi"},
		Runtime:         81_600_000_000, // 136 min in ticks
		PrimaryImageTag: "tag1",
		BackdropTags:    []string{"btag1"},
		ProviderIDs:     map[string]string{"Tmdb": "603"},
		People: []jellyfin.Person{
			{Name: "Test Actor", Role: "Actor"},
			{Name: "Test Director", Role: "Director"},
		},
	}

	got := toDetail(item, nil, "https://jf.example.com", "https://api.stoganet.com")

	fields := []struct {
		name string
		got  any
		want any
	}{
		{"ID", got.ID, "tmdb:movie:603"},
		{"Title", got.Title, "Test Movie"},
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
	if len(got.Cast) != 2 || got.Cast[0].Name != "Test Actor" || got.Cast[0].Role != "Actor" {
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

func TestToDetail_SubtitleTracks_MappedFromJellyfin(t *testing.T) {
	item := jellyfin.Item{ID: "jf-abc", Type: jellyfin.ItemTypeMovie}
	tracks := []jellyfin.SubtitleTrack{
		{Index: 2, Language: "eng", Title: "English", Codec: "subrip", IsDefault: true, IsForced: false, IsExternal: true},
	}

	got := toDetail(item, tracks, "https://jf.example.com", "https://api.stoganet.com")

	if len(got.Play.SubtitleTracks) != 1 {
		t.Fatalf("SubtitleTracks: got %d, want 1", len(got.Play.SubtitleTracks))
	}
	track := got.Play.SubtitleTracks[0]
	if track.Index != 2 || track.Language != "eng" || track.Title != "English" || track.Codec != "subrip" ||
		!track.IsDefault || track.IsForced || !track.IsExternal {
		t.Errorf("SubtitleTracks[0]: got %+v", track)
	}
}

func TestToDetail_NoSubtitleTracks_ReturnsEmptySlice(t *testing.T) {
	item := jellyfin.Item{ID: "jf-abc", Type: jellyfin.ItemTypeMovie}

	got := toDetail(item, nil, "https://jf.example.com", "https://api.stoganet.com")

	if len(got.Play.SubtitleTracks) != 0 {
		t.Errorf("SubtitleTracks: got %d, want 0", len(got.Play.SubtitleTracks))
	}
}

func TestToDetail_SeriesNoTMDB_FallsBackToJFID(t *testing.T) {
	item := jellyfin.Item{
		ID:   "jf-xyz",
		Name: "Home Video",
		Type: jellyfin.ItemTypeSeries,
		Year: 2020,
	}

	got := toDetail(item, nil, "https://jf.example.com", "https://api.stoganet.com")

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
		Name:        "Test Show",
		Type:        jellyfin.ItemTypeSeries,
		ProviderIDs: map[string]string{"Tmdb": "1396"},
	}

	got := toDetail(item, nil, "https://jf.example.com", "https://api.stoganet.com")

	if got.ID != "tmdb:tv:1396" {
		t.Errorf("ID: got %q, want %q", got.ID, "tmdb:tv:1396")
	}
}

func TestToItem_MovieWithTMDB_BuildsCorrectShape(t *testing.T) {
	item := jellyfin.Item{
		ID:              "jf-abc",
		Name:            "Test Movie",
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
		{"Title", got.Title, "Test Movie"},
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
		ID: "mov1", Name: "Test Movie", Type: jellyfin.ItemTypeMovie,
		Year: 1999, Runtime: 81_600_000_000,
		UserData: jellyfin.UserData{PlaybackPositionTicks: 2_400_000_000, Played: false},
	}
	d := toDetail(jf, nil, "http://jf.example.com", "https://api.stoganet.com")
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
		ID: "tv1", Name: "Test Show", Type: jellyfin.ItemTypeSeries,
	}
	seasons := []jellyfin.Season{
		{ID: "s1", Number: 1, Name: "Season 1", Year: 2008, EpisodeCount: 7},
	}
	nextUp := &jellyfin.Episode{
		ID: "ep3", Name: "Pilot", IndexNumber: 1, ParentIndexNumber: 1,
		UserData: jellyfin.UserData{PlaybackPositionTicks: 4_120_000_000, Played: false},
	}
	firstEp := &jellyfin.Episode{ID: "ep1", Name: "Pilot", IndexNumber: 1, ParentIndexNumber: 1}
	d := toSeriesDetail(jf, seasons, nextUp, firstEp, "http://jf.example.com", "https://api.stoganet.com")
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
	jf := jellyfin.Item{ID: "tv1", Name: "Test Show", Type: jellyfin.ItemTypeSeries}
	d := toSeriesDetail(jf, nil, nil, nil, "http://jf.example.com", "https://api.stoganet.com")
	if d.Resume != nil {
		t.Errorf("Resume should be nil for unwatched series, got %+v", d.Resume)
	}
}

func TestToSeriesDetail_Start_PopulatedFromFirstEpisode(t *testing.T) {
	jf := jellyfin.Item{ID: "tv1", Name: "Test Show", Type: jellyfin.ItemTypeSeries}
	firstEp := &jellyfin.Episode{
		ID: "ep1", Name: "Pilot", IndexNumber: 1, ParentIndexNumber: 1,
		PrimaryImageTag: "tag1",
		UserData:        jellyfin.UserData{PlaybackPositionTicks: 9_000_000_000, Played: true},
	}
	d := toSeriesDetail(jf, nil, nil, firstEp, "http://jf.example.com", "https://api.stoganet.com")
	if d.Start == nil {
		t.Fatal("Start must not be nil when firstEpisode is provided")
	}
	if d.Start.EpisodeID != "jf:ep1" {
		t.Errorf("Start.EpisodeID: got %q", d.Start.EpisodeID)
	}
	if d.Start.Play.StreamURL != "https://api.stoganet.com/stream/ep1" {
		t.Errorf("Start.Play.StreamURL: got %q", d.Start.Play.StreamURL)
	}
	if d.Start.Progress.PositionMS != 0 || d.Start.Progress.Played {
		t.Errorf("Start.Progress must be zeroed, got %+v", d.Start.Progress)
	}
	if d.Start.Thumbnail != "http://jf.example.com/Items/ep1/Images/Primary" {
		t.Errorf("Start.Thumbnail: got %q", d.Start.Thumbnail)
	}
}

func TestToSeriesDetail_NoFirstEpisode_NilStart(t *testing.T) {
	jf := jellyfin.Item{ID: "tv1", Name: "Test Show", Type: jellyfin.ItemTypeSeries}
	d := toSeriesDetail(jf, nil, nil, nil, "http://jf.example.com", "https://api.stoganet.com")
	if d.Start != nil {
		t.Errorf("Start should be nil when no firstEpisode, got %+v", d.Start)
	}
}

func TestToSearchItem_BuildsCorrectShape(t *testing.T) {
	sr := seerr.SearchResult{
		TmdbID:       603,
		MediaType:    "movie",
		Title:        "Test Movie",
		Overview:     "A hacker discovers reality is a simulation.",
		PosterPath:   "/poster.jpg",
		BackdropPath: "/backdrop.jpg",
		ReleaseDate:  "1999-03-31",
	}

	got := toSearchItem(sr)

	fields := []struct {
		name string
		got  any
		want any
	}{
		{"ID", got.ID, "tmdb:movie:603"},
		{"Title", got.Title, "Test Movie"},
		{"Year", got.Year, 1999},
		{"Type", got.Type, TypeMovie},
		{"State", got.State, StateRequestable},
		{"Overview", got.Overview, "A hacker discovers reality is a simulation."},
		{"Poster", got.Poster, "https://image.tmdb.org/t/p/w500/poster.jpg"},
		{"Backdrop", got.Backdrop, "https://image.tmdb.org/t/p/w500/backdrop.jpg"},
	}
	for _, f := range fields {
		if f.got != f.want {
			t.Errorf("%s: got %v, want %v", f.name, f.got, f.want)
		}
	}
}

func TestToSearchItem_NoImages_EmptyStrings(t *testing.T) {
	got := toSearchItem(seerr.SearchResult{TmdbID: 1, Title: "No Poster"})
	if got.Poster != "" {
		t.Errorf("Poster: got %q, want empty", got.Poster)
	}
	if got.Backdrop != "" {
		t.Errorf("Backdrop: got %q, want empty", got.Backdrop)
	}
}

func TestStateFromSeerrMediaInfo(t *testing.T) {
	cases := []struct {
		name string
		mi   *seerr.MediaInfo
		want State
	}{
		{"nil mediaInfo", nil, StateRequestable},
		{"unknown", &seerr.MediaInfo{Status: seerr.StatusUnknown}, StateRequestable},
		{"deleted", &seerr.MediaInfo{Status: seerr.StatusDeleted}, StateRequestable},
		{"pending", &seerr.MediaInfo{Status: seerr.StatusPending}, StateDownloading},
		{"processing", &seerr.MediaInfo{Status: seerr.StatusProcessing}, StateDownloading},
		{"available", &seerr.MediaInfo{Status: seerr.StatusAvailable}, StatePlayable},
		{"partially available", &seerr.MediaInfo{Status: seerr.StatusPartiallyAvailable}, StatePlayable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := stateFromSeerrMediaInfo(tc.mi); got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}
