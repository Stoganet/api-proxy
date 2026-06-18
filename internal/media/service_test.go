package media

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
	return NewService(jf, "https://jf.example.com", "https://api.stoganet.com")
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
