package jellyfin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetSeasons_ReturnsParsedSeasons(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("UserId") == "" {
			t.Error("UserId query param missing")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"Items": []map[string]any{
				{"Id": "s1", "Name": "Season 1", "IndexNumber": 1, "ProductionYear": 2008,
					"ChildCount": 7, "ImageTags": map[string]string{"Primary": "tag1"}},
				{"Id": "s0", "Name": "Specials", "IndexNumber": 0}, // must be filtered
			},
		})
	}))
	defer srv.Close()

	c := &Client{baseURL: srv.URL, hc: srv.Client()}
	seasons, err := c.GetSeasons(context.Background(), "uid", "series1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(seasons) != 1 {
		t.Fatalf("got %d seasons, want 1 (specials filtered)", len(seasons))
	}
	if seasons[0].Number != 1 || seasons[0].Name != "Season 1" || seasons[0].EpisodeCount != 7 {
		t.Errorf("season mismatch: %+v", seasons[0])
	}
}

func TestGetSeasons_ExcludesMissingSeasons(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("IsMissing"); got != "false" {
			t.Errorf("IsMissing: got %q, want \"false\"", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"Items": []map[string]any{}})
	}))
	defer srv.Close()

	c := &Client{baseURL: srv.URL, hc: srv.Client()}
	if _, err := c.GetSeasons(context.Background(), "uid", "series1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetEpisodes_ReturnsParsedEpisodes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("SeasonNumber") != "1" {
			t.Errorf("SeasonNumber: got %q", r.URL.Query().Get("SeasonNumber"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"Items": []map[string]any{
				{
					"Id": "ep1", "Name": "Pilot", "IndexNumber": 1,
					"ParentIndexNumber": 1, "Overview": "overview text",
					"RunTimeTicks": 17_640_000_000,
					"ImageTags":    map[string]string{"Primary": "tag"},
					"UserData":     map[string]any{"PlaybackPositionTicks": int64(0), "Played": false},
				},
			},
		})
	}))
	defer srv.Close()

	c := &Client{baseURL: srv.URL, hc: srv.Client()}
	eps, err := c.GetEpisodes(context.Background(), "uid", "series1", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(eps) != 1 || eps[0].ID != "ep1" || eps[0].IndexNumber != 1 {
		t.Errorf("episode mismatch: %+v", eps)
	}
}

func TestGetEpisodes_ExcludesMissingEpisodes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("IsMissing"); got != "false" {
			t.Errorf("IsMissing: got %q, want \"false\"", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"Items": []map[string]any{}})
	}))
	defer srv.Close()

	c := &Client{baseURL: srv.URL, hc: srv.Client()}
	if _, err := c.GetEpisodes(context.Background(), "uid", "series1", 1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetEpisodes_EmptyResult_ReturnsEmptySlice(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"Items": []any{}})
	}))
	defer srv.Close()

	c := &Client{baseURL: srv.URL, hc: srv.Client()}
	eps, err := c.GetEpisodes(context.Background(), "uid", "series1", 99)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(eps) != 0 {
		t.Errorf("expected empty slice, got %d episodes", len(eps))
	}
}

func TestGetNextUp_ReturnsEpisode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("SeriesId") != "series1" {
			t.Errorf("SeriesId: got %q", r.URL.Query().Get("SeriesId"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"Items": []map[string]any{
				{"Id": "ep3", "Name": "Bit by a Dead Bee", "IndexNumber": 3,
					"ParentIndexNumber": 2,
					"UserData":          map[string]any{"PlaybackPositionTicks": int64(4_120_000_000), "Played": false}},
			},
		})
	}))
	defer srv.Close()

	c := &Client{baseURL: srv.URL, hc: srv.Client()}
	ep, err := c.GetNextUp(context.Background(), "uid", "series1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ep == nil || ep.ID != "ep3" || ep.ParentIndexNumber != 2 {
		t.Errorf("episode mismatch: %+v", ep)
	}
	if ep.UserData.PlaybackPositionTicks != 4_120_000_000 {
		t.Errorf("UserData ticks: got %d", ep.UserData.PlaybackPositionTicks)
	}
}

func TestGetNextUp_NothingToResume_ReturnsNil(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"Items": []any{}})
	}))
	defer srv.Close()

	c := &Client{baseURL: srv.URL, hc: srv.Client()}
	ep, err := c.GetNextUp(context.Background(), "uid", "series1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ep != nil {
		t.Errorf("expected nil, got %+v", ep)
	}
}

func TestGetFirstEpisode_ReturnsEpisode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("SeasonNumber") != "1" {
			t.Errorf("SeasonNumber: got %q, want 1", r.URL.Query().Get("SeasonNumber"))
		}
		if r.URL.Query().Get("Limit") != "1" {
			t.Errorf("Limit: got %q, want 1", r.URL.Query().Get("Limit"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"Items": []map[string]any{
				{
					"Id": "ep1", "Name": "Pilot", "IndexNumber": 1,
					"ParentIndexNumber": 1,
					"ImageTags":         map[string]string{"Primary": "tag"},
					"UserData":          map[string]any{"PlaybackPositionTicks": int64(0), "Played": false},
				},
			},
		})
	}))
	defer srv.Close()

	c := &Client{baseURL: srv.URL, hc: srv.Client()}
	ep, err := c.GetFirstEpisode(context.Background(), "uid", "series1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ep == nil {
		t.Fatal("expected episode, got nil")
	}
	if ep.ID != "ep1" || ep.IndexNumber != 1 || ep.Name != "Pilot" {
		t.Errorf("episode mismatch: %+v", ep)
	}
}

func TestGetFirstEpisode_ExcludesMissingEpisodes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("IsMissing"); got != "false" {
			t.Errorf("IsMissing: got %q, want \"false\"", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"Items": []map[string]any{}})
	}))
	defer srv.Close()

	c := &Client{baseURL: srv.URL, hc: srv.Client()}
	if _, err := c.GetFirstEpisode(context.Background(), "uid", "series1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetFirstEpisode_EmptyResult_ReturnsNil(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"Items": []any{}})
	}))
	defer srv.Close()

	c := &Client{baseURL: srv.URL, hc: srv.Client()}
	ep, err := c.GetFirstEpisode(context.Background(), "uid", "series1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ep != nil {
		t.Errorf("expected nil, got %+v", ep)
	}
}

func TestGetNextEpisode_ReturnsEpisodeAfterCurrent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/Shows/series1/Episodes" {
			t.Errorf("path: got %q", r.URL.Path)
		}
		if r.URL.Query().Get("StartItemId") != "ep3" {
			t.Errorf("StartItemId: got %q, want ep3", r.URL.Query().Get("StartItemId"))
		}
		if r.URL.Query().Get("IsMissing") != "false" {
			t.Errorf("IsMissing: got %q, want false", r.URL.Query().Get("IsMissing"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"Items": []map[string]any{
				{"Id": "ep3", "Name": "Current", "IndexNumber": 3, "ParentIndexNumber": 1},
				{
					"Id": "ep4", "Name": "Next", "IndexNumber": 4, "ParentIndexNumber": 1,
					"UserData": map[string]any{"PlaybackPositionTicks": int64(1_200_000_000), "Played": false},
				},
			},
		})
	}))
	defer srv.Close()

	c := &Client{baseURL: srv.URL, hc: srv.Client()}
	ep, err := c.GetNextEpisode(context.Background(), "uid", "series1", "ep3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ep == nil {
		t.Fatal("expected episode, got nil")
	}
	if ep.ID != "ep4" || ep.IndexNumber != 4 || ep.ParentIndexNumber != 1 || ep.Name != "Next" {
		t.Errorf("episode mismatch: %+v", ep)
	}
	if ep.UserData.PlaybackPositionTicks != 1_200_000_000 {
		t.Errorf("UserData: got %+v", ep.UserData)
	}
}

func TestGetNextEpisode_LastEpisode_ReturnsNil(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"Items": []map[string]any{{"Id": "ep10", "IndexNumber": 10, "ParentIndexNumber": 2}},
		})
	}))
	defer srv.Close()

	c := &Client{baseURL: srv.URL, hc: srv.Client()}
	ep, err := c.GetNextEpisode(context.Background(), "uid", "series1", "ep10")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ep != nil {
		t.Errorf("expected nil, got %+v", ep)
	}
}

func TestGetNextEpisode_SkipsSpecials(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"Items": []map[string]any{
				{"Id": "ep10", "IndexNumber": 10, "ParentIndexNumber": 1},
				{"Id": "sp1", "IndexNumber": 1, "ParentIndexNumber": 0},
				{"Id": "sp2", "IndexNumber": 2, "ParentIndexNumber": 0},
				{"Id": "s2e1", "IndexNumber": 1, "ParentIndexNumber": 2},
			},
		})
	}))
	defer srv.Close()

	c := &Client{baseURL: srv.URL, hc: srv.Client()}
	ep, err := c.GetNextEpisode(context.Background(), "uid", "series1", "ep10")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ep == nil || ep.ID != "s2e1" {
		t.Errorf("expected s2e1, got %+v", ep)
	}
}

func TestGetNextEpisode_OnlySpecialsAfter_ReturnsNil(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"Items": []map[string]any{
				{"Id": "ep10", "IndexNumber": 10, "ParentIndexNumber": 2},
				{"Id": "sp1", "IndexNumber": 1, "ParentIndexNumber": 0},
			},
		})
	}))
	defer srv.Close()

	c := &Client{baseURL: srv.URL, hc: srv.Client()}
	ep, err := c.GetNextEpisode(context.Background(), "uid", "series1", "ep10")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ep != nil {
		t.Errorf("expected nil, got %+v", ep)
	}
}

// Guards against Jellyfin ignoring StartItemId and listing from the first episode.
func TestGetNextEpisode_FirstItemNotCurrent_ReturnsNil(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"Items": []map[string]any{
				{"Id": "ep1", "IndexNumber": 1, "ParentIndexNumber": 1},
				{"Id": "ep2", "IndexNumber": 2, "ParentIndexNumber": 1},
			},
		})
	}))
	defer srv.Close()

	c := &Client{baseURL: srv.URL, hc: srv.Client()}
	ep, err := c.GetNextEpisode(context.Background(), "uid", "series1", "ep3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ep != nil {
		t.Errorf("expected nil, got %+v", ep)
	}
}
