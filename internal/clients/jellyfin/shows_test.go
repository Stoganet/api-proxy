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

func TestGetEpisodes_EmptyResult_ReturnsErrItemNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"Items": []any{}})
	}))
	defer srv.Close()

	c := &Client{baseURL: srv.URL, hc: srv.Client()}
	_, err := c.GetEpisodes(context.Background(), "uid", "series1", 99)
	if err != ErrItemNotFound {
		t.Errorf("got %v, want ErrItemNotFound", err)
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
