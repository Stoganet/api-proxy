package media

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Stoganet/api-proxy/internal/clients/jellyfin"
)

var ErrItemNotFound = errors.New("catalog: item not found")

const tmdbIndexRefreshInterval = 5 * time.Minute

type JellyfinClient interface {
	GetItem(ctx context.Context, userID, itemID string) (*jellyfin.Item, error)
	GetItems(ctx context.Context, userID string, opts jellyfin.GetItemsOpts) (*jellyfin.ItemsResult, error)
	GetSeasons(ctx context.Context, userID, seriesID string) ([]jellyfin.Season, error)
	GetEpisodes(ctx context.Context, userID, seriesID string, seasonNumber int) ([]jellyfin.Episode, error)
	GetNextUp(ctx context.Context, userID, seriesID string) (*jellyfin.Episode, error)
	GetFirstEpisode(ctx context.Context, userID, seriesID string) (*jellyfin.Episode, error)
}

type Service struct {
	jf           JellyfinClient
	baseURL      string
	proxyBaseURL string
	logger       *slog.Logger
	tmdbIndex    atomic.Pointer[map[string]string]
}

func NewService(jf JellyfinClient, jellyfinBaseURL, proxyBaseURL string, logger *slog.Logger) *Service {
	return &Service{jf: jf, baseURL: jellyfinBaseURL, proxyBaseURL: proxyBaseURL, logger: logger}
}

func (s *Service) RefreshTmdbIndex(ctx context.Context) error {
	index := make(map[string]string)

	for _, t := range []struct {
		jfType   jellyfin.ItemType
		catalogT Type
	}{
		{jellyfin.ItemTypeMovie, TypeMovie},
		{jellyfin.ItemTypeSeries, TypeTV},
	} {
		result, err := s.jf.GetItems(ctx, "", jellyfin.GetItemsOpts{
			Type:   t.jfType,
			Fields: jellyfin.FieldsProviderIDsOnly,
		})
		if err != nil {
			return fmt.Errorf("RefreshTmdbIndex: %w", err)
		}
		for i := range result.Items {
			tmdbID := result.Items[i].ProviderIDs["Tmdb"]
			if tmdbID == "" {
				continue
			}
			index[string(t.catalogT)+":"+tmdbID] = result.Items[i].ID
		}
	}

	s.tmdbIndex.Store(&index)
	return nil
}

func (s *Service) StartTmdbIndexRefresher(ctx context.Context) {
	ticker := time.NewTicker(tmdbIndexRefreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.RefreshTmdbIndex(ctx); err != nil {
				s.logger.Warn("tmdb index refresh failed", "err", err)
			}
		}
	}
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
	var (
		seasons      []jellyfin.Season
		seasonsErr   error
		nextUp       *jellyfin.Episode
		nextUpErr    error
		firstEpisode *jellyfin.Episode
		firstEpErr   error
	)
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		seasons, seasonsErr = s.jf.GetSeasons(ctx, jfUserID, item.ID)
	}()
	go func() {
		defer wg.Done()
		nextUp, nextUpErr = s.jf.GetNextUp(ctx, jfUserID, item.ID)
	}()
	go func() {
		defer wg.Done()
		firstEpisode, firstEpErr = s.jf.GetFirstEpisode(ctx, jfUserID, item.ID)
	}()
	wg.Wait()

	if seasonsErr != nil {
		return nil, fmt.Errorf("getSeriesDetail: GetSeasons: %w", seasonsErr)
	}
	if nextUpErr != nil {
		return nil, fmt.Errorf("getSeriesDetail: GetNextUp: %w", nextUpErr)
	}
	if firstEpErr != nil {
		return nil, fmt.Errorf("getSeriesDetail: GetFirstEpisode: %w", firstEpErr)
	}
	d := toSeriesDetail(item, seasons, nextUp, firstEpisode, s.baseURL, s.proxyBaseURL)
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
		switch Type(parts[1]) {
		case TypeMovie, TypeTV:
		default:
			return nil, ErrItemNotFound
		}

		index := s.tmdbIndex.Load()
		if index == nil {
			return nil, fmt.Errorf("catalog resolveItem: tmdb index not built yet")
		}
		jfID, ok := (*index)[parts[1]+":"+parts[2]]
		if !ok {
			return nil, ErrItemNotFound
		}

		item, err := s.jf.GetItem(ctx, jfUserID, jfID)
		if err != nil {
			if errors.Is(err, jellyfin.ErrItemNotFound) {
				return nil, ErrItemNotFound
			}
			return nil, fmt.Errorf("catalog resolveItem: %w", err)
		}
		return item, nil
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
	for i, r := range results {
		if r.err != nil {
			s.logger.Warn("home: section failed", "section", homeSections[i].id, "err", r.err)
			continue
		}
		sections = append(sections, r.section)
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
