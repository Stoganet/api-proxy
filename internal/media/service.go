package media

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/Stoganet/api-proxy/internal/clients/jellyfin"
)

var ErrItemNotFound = errors.New("catalog: item not found")

type JellyfinClient interface {
	GetItem(ctx context.Context, userID, itemID string) (*jellyfin.Item, error)
	GetItems(ctx context.Context, userID string, opts jellyfin.GetItemsOpts) (*jellyfin.ItemsResult, error)
	GetSeasons(ctx context.Context, userID, seriesID string) ([]jellyfin.Season, error)
	GetEpisodes(ctx context.Context, userID, seriesID string, seasonNumber int) ([]jellyfin.Episode, error)
	GetNextUp(ctx context.Context, userID, seriesID string) (*jellyfin.Episode, error)
}

type Service struct {
	jf           JellyfinClient
	baseURL      string
	proxyBaseURL string
}

func NewService(jf JellyfinClient, jellyfinBaseURL, proxyBaseURL string) *Service {
	return &Service{jf: jf, baseURL: jellyfinBaseURL, proxyBaseURL: proxyBaseURL}
}

func (s *Service) GetItem(ctx context.Context, jfUserID, catalogID string) (*Detail, error) {
	item, err := s.resolveItem(ctx, jfUserID, catalogID)
	if err != nil {
		return nil, err
	}
	if item.Type == jellyfin.ItemTypeSeries {
		return s.getSeriesDetail(ctx, jfUserID, *item)
	}
	d := toDetail(*item, s.baseURL, s.proxyBaseURL)
	return &d, nil
}

func (s *Service) getSeriesDetail(ctx context.Context, jfUserID string, item jellyfin.Item) (*Detail, error) {
	seasons, err := s.jf.GetSeasons(ctx, jfUserID, item.ID)
	if err != nil {
		return nil, fmt.Errorf("getSeriesDetail: GetSeasons: %w", err)
	}
	nextUp, err := s.jf.GetNextUp(ctx, jfUserID, item.ID)
	if err != nil {
		return nil, fmt.Errorf("getSeriesDetail: GetNextUp: %w", err)
	}
	d := toSeriesDetail(item, seasons, nextUp, s.baseURL, s.proxyBaseURL)
	return &d, nil
}

func (s *Service) GetEpisodes(ctx context.Context, jfUserID, catalogID string, seasonNumber int) ([]Episode, error) {
	item, err := s.resolveItem(ctx, jfUserID, catalogID)
	if err != nil {
		return nil, err
	}
	episodes, err := s.jf.GetEpisodes(ctx, jfUserID, item.ID, seasonNumber)
	if err != nil {
		if errors.Is(err, jellyfin.ErrItemNotFound) {
			return nil, ErrItemNotFound
		}
		return nil, fmt.Errorf("GetEpisodes: %w", err)
	}
	result := make([]Episode, len(episodes))
	for i, ep := range episodes {
		result[i] = toEpisode(ep, s.baseURL, s.proxyBaseURL)
	}
	return result, nil
}

// resolveItem translates a catalog ID to a Jellyfin item.
//
// Catalog IDs have two forms:
//   - "jf:{jellyfinUUID}"      → direct Jellyfin lookup after stripping prefix
//   - "tmdb:{type}:{tmdbID}"   → provider-ID search via AnyProviderIdEquals
func (s *Service) resolveItem(ctx context.Context, jfUserID, catalogID string) (*jellyfin.Item, error) {
	if jfID, ok := strings.CutPrefix(catalogID, "jf:"); ok {
		item, err := s.jf.GetItem(ctx, jfUserID, jfID)
		if err != nil {
			if errors.Is(err, jellyfin.ErrItemNotFound) {
				return nil, ErrItemNotFound
			}
			return nil, fmt.Errorf("catalog resolveItem: %w", err)
		}
		return item, nil
	}

	if strings.HasPrefix(catalogID, "tmdb:") {
		parts := strings.SplitN(catalogID, ":", 3)
		if len(parts) != 3 {
			return nil, ErrItemNotFound
		}
		providerID := "Tmdb." + parts[2]
		result, err := s.jf.GetItems(ctx, jfUserID, jellyfin.GetItemsOpts{
			ProviderID: providerID,
			Limit:      1,
		})
		if err != nil {
			return nil, fmt.Errorf("catalog resolveItem: %w", err)
		}
		if len(result.Items) == 0 {
			return nil, ErrItemNotFound
		}
		return &result.Items[0], nil
	}

	return nil, ErrItemNotFound
}

const homeRowLimit = 20

type sectionDef struct {
	id   string
	opts jellyfin.GetItemsOpts
}

var homeSections = []sectionDef{
	{id: "recently_added_movies", opts: jellyfin.GetItemsOpts{Type: jellyfin.ItemTypeMovie, SortBy: jellyfin.SortByDateCreated, SortDesc: true, Limit: homeRowLimit}},
	{id: "recently_added_tv", opts: jellyfin.GetItemsOpts{Type: jellyfin.ItemTypeSeries, SortBy: jellyfin.SortByDateCreated, SortDesc: true, Limit: homeRowLimit}},
	{id: "all_movies", opts: jellyfin.GetItemsOpts{Type: jellyfin.ItemTypeMovie, Limit: homeRowLimit}},
	{id: "all_tv", opts: jellyfin.GetItemsOpts{Type: jellyfin.ItemTypeSeries, Limit: homeRowLimit}},
}

func (s *Service) Home(ctx context.Context, jfUserID string) (*HomeResult, error) {
	type result struct {
		section HomeSection
		err     error
	}

	results := make([]result, len(homeSections))
	var wg sync.WaitGroup
	wg.Add(len(homeSections))

	for i, def := range homeSections {
		go func() {
			defer wg.Done()
			res, err := s.jf.GetItems(ctx, jfUserID, def.opts)
			if err != nil {
				results[i] = result{err: err}
				return
			}
			items := make([]Item, len(res.Items))
			for j, jfi := range res.Items {
				items[j] = toItem(jfi, s.baseURL)
			}
			results[i] = result{section: HomeSection{
				ID:      def.id,
				Items:   items,
				HasMore: res.TotalCount > len(res.Items),
			}}
		}()
	}
	wg.Wait()

	sections := make([]HomeSection, 0, len(homeSections))
	for _, r := range results {
		if r.err == nil {
			sections = append(sections, r.section)
		}
	}
	if len(sections) == 0 && len(homeSections) > 0 {
		return nil, fmt.Errorf("home: all sections failed")
	}
	return &HomeResult{Sections: sections}, nil
}

func (s *Service) List(ctx context.Context, jfUserID string, opts ListOpts) (*ListResult, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = 40
	}
	if limit > 100 {
		limit = 100
	}

	jfOpts := jellyfin.GetItemsOpts{
		Limit:      limit,
		StartIndex: opts.StartIndex,
	}
	switch opts.Type {
	case TypeMovie:
		jfOpts.Type = jellyfin.ItemTypeMovie
	case TypeTV:
		jfOpts.Type = jellyfin.ItemTypeSeries
	}

	result, err := s.jf.GetItems(ctx, jfUserID, jfOpts)
	if err != nil {
		return nil, fmt.Errorf("catalog List: %w", err)
	}

	items := make([]Item, len(result.Items))
	for i, jfi := range result.Items {
		items[i] = toItem(jfi, s.baseURL)
	}

	nextCursor := ""
	nextIndex := result.StartIndex + len(result.Items)
	if nextIndex < result.TotalCount {
		nextCursor = fmt.Sprintf("%d", nextIndex)
	}

	return &ListResult{
		Items:      items,
		Total:      result.TotalCount,
		NextCursor: nextCursor,
	}, nil
}
