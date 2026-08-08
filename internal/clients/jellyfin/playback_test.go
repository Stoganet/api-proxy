package jellyfin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetSubtitleTracks_ReturnsOnlySubtitleStreams(t *testing.T) {
	c := newItemsClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/Items/item-1/PlaybackInfo" {
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("UserId") != "uid-1" {
			t.Errorf("UserId: got %q", r.URL.Query().Get("UserId"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"MediaSources": []map[string]any{
				{
					"MediaStreams": []map[string]any{
						{"Type": "Video", "Index": 0, "Codec": "h264"},
						{"Type": "Audio", "Index": 1, "Codec": "aac"},
						{
							"Type": "Subtitle", "Index": 2, "Language": "eng",
							"DisplayTitle": "English", "Codec": "subrip",
							"IsDefault": true, "IsForced": false, "IsExternal": true,
						},
					},
				},
			},
		})
	})

	tracks, err := c.GetSubtitleTracks(context.Background(), "uid-1", "item-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tracks) != 1 {
		t.Fatalf("tracks: got %d, want 1", len(tracks))
	}
	got := tracks[0]
	if got.Index != 2 || got.Language != "eng" || got.Title != "English" || got.Codec != "subrip" ||
		!got.IsDefault || got.IsForced || !got.IsExternal {
		t.Errorf("track: got %+v", got)
	}
}

func TestGetSubtitleTracks_NoMediaSources_ReturnsEmpty(t *testing.T) {
	c := newItemsClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"MediaSources": []map[string]any{}})
	})

	tracks, err := c.GetSubtitleTracks(context.Background(), "uid-1", "item-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tracks) != 0 {
		t.Errorf("tracks: got %d, want 0", len(tracks))
	}
}

func TestGetSubtitleTracks_NotFound_ReturnsErrItemNotFound(t *testing.T) {
	c := newItemsClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	_, err := c.GetSubtitleTracks(context.Background(), "uid-1", "missing")
	if err != ErrItemNotFound {
		t.Errorf("got %v, want ErrItemNotFound", err)
	}
}

func TestGetSubtitleTracks_MalformedJSON_ReturnsDecodeError(t *testing.T) {
	c := newItemsClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("{not json"))
	})

	_, err := c.GetSubtitleTracks(context.Background(), "uid-1", "item-1")
	if err == nil {
		t.Fatal("expected decode error")
	}
}

func TestGetSubtitleTracks_MultipleMediaSources_OnlyUsesFirst(t *testing.T) {
	c := newItemsClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"MediaSources": []map[string]any{
				{
					"MediaStreams": []map[string]any{
						{"Type": "Subtitle", "Index": 2, "Language": "eng"},
					},
				},
				{
					"MediaStreams": []map[string]any{
						{"Type": "Subtitle", "Index": 5, "Language": "fre"},
					},
				},
			},
		})
	})

	tracks, err := c.GetSubtitleTracks(context.Background(), "uid-1", "item-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tracks) != 1 || tracks[0].Language != "eng" {
		t.Errorf("tracks: got %+v, want only the first media source's eng track", tracks)
	}
}

func TestGetSubtitleTracks_UpstreamError_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	c := New(srv.URL, "test-api-key")

	_, err := c.GetSubtitleTracks(context.Background(), "uid-1", "item-1")
	if err == nil {
		t.Fatal("expected error")
	}
}
