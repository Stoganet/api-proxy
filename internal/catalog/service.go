package catalog

import (
	"context"
	"errors"
	"fmt"

	"github.com/Stoganet/api-proxy/internal/clients/jellyfin"
)

var ErrItemNotFound = errors.New("catalog: item not found")

type JellyfinClient interface {
	GetItem(ctx context.Context, userID, itemID string) (*jellyfin.Item, error)
	GetItems(ctx context.Context, userID string, opts jellyfin.GetItemsOpts) (*jellyfin.ItemsResult, error)
}

type Service struct {
	jf      JellyfinClient
	baseURL string
}

func NewService(jf JellyfinClient, jellyfinBaseURL string) *Service {
	return &Service{jf: jf, baseURL: jellyfinBaseURL}
}

func (s *Service) GetItem(ctx context.Context, jfUserID, jfToken, itemID string) (*Detail, error) {
	item, err := s.jf.GetItem(ctx, jfUserID, itemID)
	if err != nil {
		if errors.Is(err, jellyfin.ErrItemNotFound) {
			return nil, ErrItemNotFound
		}
		return nil, fmt.Errorf("catalog GetItem: %w", err)
	}
	d := toDetail(*item, s.baseURL, jfToken, jfUserID)
	return &d, nil
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
	case "movie":
		jfOpts.Type = "Movie"
	case "tv":
		jfOpts.Type = "Series"
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
