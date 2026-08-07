package seerr

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newSeerrClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	s := httptest.NewServer(handler)
	t.Cleanup(s.Close)
	return New(s.URL, "test-api-key")
}

func TestSearch_SendsQueryAndAPIKey(t *testing.T) {
	c := newSeerrClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/search" {
			http.NotFound(w, r)
			return
		}
		if got := r.URL.Query().Get("query"); got != "matrix" {
			t.Errorf("query param: got %q, want %q", got, "matrix")
		}
		if got := r.Header.Get("X-Api-Key"); got != "test-api-key" {
			t.Errorf("X-Api-Key: got %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{
					"id":           603,
					"mediaType":    "movie",
					"title":        "The Matrix",
					"posterPath":   "/poster.jpg",
					"backdropPath": "/backdrop.jpg",
					"releaseDate":  "1999-03-31",
					"overview":     "A hacker discovers reality is a simulation.",
				},
			},
		})
	})

	results, err := c.Search(context.Background(), "matrix")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("results: got %d, want 1", len(results))
	}
	r := results[0]
	if r.TmdbID != 603 || r.Title != "The Matrix" || r.PosterPath != "/poster.jpg" {
		t.Errorf("result: %+v", r)
	}
	if r.MediaInfo != nil {
		t.Errorf("expected nil MediaInfo when absent, got %+v", r.MediaInfo)
	}
}

func TestSearch_FiltersOutNonMovieResults(t *testing.T) {
	c := newSeerrClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{"id": 1, "mediaType": "movie", "title": "A Movie"},
				{"id": 2, "mediaType": "tv", "name": "A Show"},
				{"id": 3, "mediaType": "person", "name": "Someone"},
			},
		})
	})

	results, err := c.Search(context.Background(), "query")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 || results[0].TmdbID != 1 {
		t.Errorf("results: %+v, want only the movie result", results)
	}
}

func TestSearch_IncludesMediaInfoStatus(t *testing.T) {
	c := newSeerrClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{
					"id": 1, "mediaType": "movie", "title": "Available Movie",
					"mediaInfo": map[string]any{"status": 5},
				},
			},
		})
	})

	results, err := c.Search(context.Background(), "query")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results[0].MediaInfo == nil || results[0].MediaInfo.Status != StatusAvailable {
		t.Errorf("MediaInfo: %+v", results[0].MediaInfo)
	}
}

func TestSearch_UpstreamNon200_ReturnsError(t *testing.T) {
	c := newSeerrClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	_, err := c.Search(context.Background(), "query")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRequestMovie_SendsCorrectBodyAndAPIKey(t *testing.T) {
	var capturedBody map[string]any
	c := newSeerrClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/request" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodPost {
			t.Errorf("method: got %q, want POST", r.Method)
		}
		if got := r.Header.Get("X-Api-Key"); got != "test-api-key" {
			t.Errorf("X-Api-Key: got %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": 42})
	})

	if err := c.RequestMovie(context.Background(), 603); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedBody["mediaType"] != "movie" {
		t.Errorf("mediaType: got %v", capturedBody["mediaType"])
	}
	if capturedBody["mediaId"] != float64(603) {
		t.Errorf("mediaId: got %v", capturedBody["mediaId"])
	}
}

func TestRequestMovie_UpstreamNon201_ReturnsError(t *testing.T) {
	c := newSeerrClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	})

	err := c.RequestMovie(context.Background(), 603)
	if err == nil {
		t.Fatal("expected error")
	}
}
