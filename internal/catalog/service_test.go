package catalog

import (
	"context"
	"errors"
	"testing"

	"github.com/Stoganet/api-proxy/internal/clients/jellyfin"
)

type fakeJF struct {
	item           *jellyfin.Item
	items          *jellyfin.ItemsResult
	err            error
	capturedItemID string
	capturedOpts   jellyfin.GetItemsOpts
}

func (f *fakeJF) GetItem(_ context.Context, _, itemID string) (*jellyfin.Item, error) {
	f.capturedItemID = itemID
	return f.item, f.err
}

func (f *fakeJF) GetItems(_ context.Context, _ string, opts jellyfin.GetItemsOpts) (*jellyfin.ItemsResult, error) {
	f.capturedOpts = opts
	return f.items, f.err
}

func newSvc(jf JellyfinClient) *Service {
	return NewService(jf, "https://jf.example.com")
}

func TestService_GetItem_JFPrefix_StripsPrefix(t *testing.T) {
	jf := &fakeJF{item: &jellyfin.Item{
		ID:   "abc-uuid",
		Name: "Home Video",
		Type: "Movie",
	}}
	svc := newSvc(jf)
	_, err := svc.GetItem(context.Background(), "jf-user-1", "tok", "jf:abc-uuid")
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
	d, err := svc.GetItem(context.Background(), "jf-user-1", "tok", "tmdb:movie:603")
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
	_, err := svc.GetItem(context.Background(), "jf-user-1", "tok", "tmdb:movie:999")
	if !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("want ErrItemNotFound, got %v", err)
	}
}

func TestService_GetItem_InvalidID_ReturnsNotFound(t *testing.T) {
	svc := newSvc(&fakeJF{})
	_, err := svc.GetItem(context.Background(), "jf-user-1", "tok", "not-a-valid-id")
	if !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("want ErrItemNotFound, got %v", err)
	}
}

func TestService_GetItem_PropagatesNotFound(t *testing.T) {
	jf := &fakeJF{err: jellyfin.ErrItemNotFound}
	svc := newSvc(jf)
	_, err := svc.GetItem(context.Background(), "jf-user-1", "tok", "jf:missing-uuid")
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
	d, err := svc.GetItem(context.Background(), "jf-user-1", "jf-tok", "jf:jf-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.ID != "tmdb:movie:603" {
		t.Errorf("ID: got %q", d.ID)
	}
	if d.Play == nil || d.Play.JellyfinAccessToken != "jf-tok" {
		t.Errorf("Play.JellyfinAccessToken not propagated")
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
