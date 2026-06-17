package media

import (
	"testing"

	"github.com/Stoganet/api-proxy/internal/clients/jellyfin"
)

func TestToDetail_MovieWithTMDB_BuildsCorrectShape(t *testing.T) {
	item := jellyfin.Item{
		ID:              "jf-abc",
		Name:            "The Matrix",
		Type:            "Movie",
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

	got := toDetail(item, "https://jf.example.com", "jf-tok-123", "jf-uid-456")

	fields := []struct {
		name string
		got  any
		want any
	}{
		{"ID", got.ID, "tmdb:movie:603"},
		{"Title", got.Title, "The Matrix"},
		{"Year", got.Year, 1999},
		{"Type", got.Type, "movie"},
		{"State", got.State, "playable"},
		{"Overview", got.Overview, "A hacker discovers reality is a simulation."},
		{"Runtime", got.Runtime, 136},
		{"Poster", got.Poster, "https://jf.example.com/Items/jf-abc/Images/Primary"},
		{"Backdrop", got.Backdrop, "https://jf.example.com/Items/jf-abc/Images/Backdrop/0"},
		{"Seasons", got.Seasons, 0},
	}
	for _, f := range fields {
		if f.got != f.want {
			t.Errorf("%s: got %v, want %v", f.name, f.got, f.want)
		}
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
	if got.Play.JellyfinItemID != "jf-abc" {
		t.Errorf("Play.JellyfinItemID: got %q", got.Play.JellyfinItemID)
	}
	if got.Play.JellyfinAccessToken != "jf-tok-123" {
		t.Errorf("Play.JellyfinAccessToken: got %q", got.Play.JellyfinAccessToken)
	}
	if got.Play.JellyfinUserID != "jf-uid-456" {
		t.Errorf("Play.JellyfinUserID: got %q", got.Play.JellyfinUserID)
	}
	if got.Play.JellyfinBaseURL != "https://jf.example.com" {
		t.Errorf("Play.JellyfinBaseURL: got %q", got.Play.JellyfinBaseURL)
	}
}

func TestToDetail_SeriesNoTMDB_FallsBackToJFID(t *testing.T) {
	item := jellyfin.Item{
		ID:         "jf-xyz",
		Name:       "Home Video",
		Type:       "Series",
		Year:       2020,
		ChildCount: 3,
	}

	got := toDetail(item, "https://jf.example.com", "tok", "uid")

	if got.ID != "jf:jf-xyz" {
		t.Errorf("ID: got %q, want %q", got.ID, "jf:jf-xyz")
	}
	if got.Type != "tv" {
		t.Errorf("Type: got %q, want %q", got.Type, "tv")
	}
	if got.Backdrop != "" {
		t.Errorf("Backdrop should be empty when no BackdropTags, got %q", got.Backdrop)
	}
	if got.Seasons != 3 {
		t.Errorf("Seasons: got %d, want 3", got.Seasons)
	}
}

func TestToDetail_TVShowWithTMDB_BuildsCorrectID(t *testing.T) {
	item := jellyfin.Item{
		ID:          "jf-tv-1",
		Name:        "Breaking Bad",
		Type:        "Series",
		ProviderIDs: map[string]string{"Tmdb": "1396"},
	}

	got := toDetail(item, "https://jf.example.com", "tok", "uid")

	if got.ID != "tmdb:tv:1396" {
		t.Errorf("ID: got %q, want %q", got.ID, "tmdb:tv:1396")
	}
}

func TestToItem_MovieWithTMDB_BuildsCorrectShape(t *testing.T) {
	item := jellyfin.Item{
		ID:              "jf-abc",
		Name:            "The Matrix",
		Type:            "Movie",
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
		{"Type", got.Type, "movie"},
		{"State", got.State, "playable"},
		{"Poster", got.Poster, "https://jf.example.com/Items/jf-abc/Images/Primary"},
	}
	for _, f := range fields {
		if f.got != f.want {
			t.Errorf("%s: got %v, want %v", f.name, f.got, f.want)
		}
	}
}
