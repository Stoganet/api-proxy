package jellyfin

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newItemsClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	s := httptest.NewServer(handler)
	t.Cleanup(s.Close)
	return New(s.URL, "test-api-key")
}

func TestGetItem_ReturnsItem(t *testing.T) {
	c := newItemsClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/Users/uid-1/Items/item-1" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"Id":             "item-1",
			"Name":           "The Matrix",
			"Type":           "Movie",
			"ProductionYear": 1999,
			"ProviderIds":    map[string]string{"Tmdb": "603"},
			"RunTimeTicks":   81_600_000_000,
			"ImageTags":      map[string]string{"Primary": "tag1"},
			"Genres":         []string{"Action"},
		})
	})

	item, err := c.GetItem(context.Background(), "uid-1", "item-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item.ID != "item-1" {
		t.Errorf("ID: got %q", item.ID)
	}
	if item.Name != "The Matrix" {
		t.Errorf("Name: got %q", item.Name)
	}
	if item.Year != 1999 {
		t.Errorf("Year: got %d", item.Year)
	}
	if item.ProviderIDs["Tmdb"] != "603" {
		t.Errorf("ProviderIDs[Tmdb]: got %q", item.ProviderIDs["Tmdb"])
	}
	if item.Runtime != 81_600_000_000 {
		t.Errorf("Runtime: got %d", item.Runtime)
	}
	if item.PrimaryImageTag != "tag1" {
		t.Errorf("PrimaryImageTag: got %q", item.PrimaryImageTag)
	}
}

func TestGetItem_NotFound_ReturnsErrItemNotFound(t *testing.T) {
	c := newItemsClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	_, err := c.GetItem(context.Background(), "uid-1", "missing")
	if !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("want ErrItemNotFound, got %v", err)
	}
}

func TestGetItems_ReturnsPaginatedResult(t *testing.T) {
	c := newItemsClient(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("IncludeItemTypes") == "" {
			http.Error(w, "missing IncludeItemTypes", http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"Items": []map[string]any{
				{"Id": "m1", "Name": "Movie 1", "Type": "Movie"},
				{"Id": "m2", "Name": "Movie 2", "Type": "Movie"},
			},
			"TotalRecordCount": 100,
			"StartIndex":       0,
		})
	})

	res, err := c.GetItems(context.Background(), "uid-1", GetItemsOpts{Type: "Movie", Limit: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Items) != 2 {
		t.Errorf("Items: got %d", len(res.Items))
	}
	if res.TotalCount != 100 {
		t.Errorf("TotalCount: got %d", res.TotalCount)
	}
}
