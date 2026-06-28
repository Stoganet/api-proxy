package jellyfin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

type ItemType string

const (
	ItemTypeMovie  ItemType = "Movie"
	ItemTypeSeries ItemType = "Series"
)

type UserData struct {
	PlaybackPositionTicks int64
	Played                bool
}

type Item struct {
	ID              string
	Name            string
	Type            ItemType
	Year            int
	Overview        string
	Genres          []string
	Runtime         int64
	PrimaryImageTag string
	BackdropTags    []string
	ProviderIDs     map[string]string
	People          []Person
	ChildCount      int
	UserData        UserData
}

type Person struct {
	Name string
	Role string
}

type ItemsResult struct {
	Items      []Item
	TotalCount int
	StartIndex int
}

const (
	SortByDateCreated     = "DateCreated"
	SortBySortName        = "SortName"
	SortByCommunityRating = "CommunityRating"
	SortByPremiereDate    = "PremiereDate"
	SortByDatePlayed      = "DatePlayed"
	SortByRandom          = "Random"
)

type GetItemsOpts struct {
	Type       ItemType // ItemTypeMovie, ItemTypeSeries, or zero value = both
	Limit      int
	StartIndex int
	ProviderID string // e.g. "Tmdb.603"; sets AnyProviderIdEquals when non-empty
	SortBy     string
	SortDesc   bool
}

func (c *Client) GetItem(ctx context.Context, userID, itemID string) (*Item, error) {
	raw, err := url.JoinPath(c.baseURL, "Users", userID, "Items", itemID)
	if err != nil {
		return nil, fmt.Errorf("jellyfin GetItem: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Emby-Authorization", authHeader(userID))
	req.Header.Set("X-Emby-Token", c.apiKey)

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jellyfin GetItem: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrItemNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jellyfin GetItem: unexpected status %d", resp.StatusCode)
	}

	var decoded jfItemResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("jellyfin GetItem: decode: %w", err)
	}
	return decoded.toItem(), nil
}

func (c *Client) GetItems(ctx context.Context, userID string, opts GetItemsOpts) (*ItemsResult, error) {
	raw, err := url.JoinPath(c.baseURL, "Users", userID, "Items")
	if err != nil {
		return nil, fmt.Errorf("jellyfin GetItems: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Set("Recursive", "true")
	q.Set("Fields", "Genres,People,ProviderIds,Overview,ChildCount")
	if opts.Type != "" {
		q.Set("IncludeItemTypes", string(opts.Type))
	} else {
		q.Set("IncludeItemTypes", "Movie,Series")
	}
	if opts.Limit > 0 {
		q.Set("Limit", fmt.Sprintf("%d", opts.Limit))
	}
	if opts.StartIndex > 0 {
		q.Set("StartIndex", fmt.Sprintf("%d", opts.StartIndex))
	}
	if opts.ProviderID != "" {
		q.Set("AnyProviderIdEquals", opts.ProviderID)
	}
	if opts.SortBy != "" {
		q.Set("SortBy", opts.SortBy)
		if opts.SortDesc {
			q.Set("SortOrder", "Descending")
		} else {
			q.Set("SortOrder", "Ascending")
		}
	}
	req.URL.RawQuery = q.Encode()

	req.Header.Set("X-Emby-Authorization", authHeader(userID))
	req.Header.Set("X-Emby-Token", c.apiKey)

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jellyfin GetItems: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jellyfin GetItems: unexpected status %d", resp.StatusCode)
	}

	var decoded struct {
		Items            []jfItemResponse `json:"Items"`
		TotalRecordCount int              `json:"TotalRecordCount"`
		StartIndex       int              `json:"StartIndex"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("jellyfin GetItems: decode: %w", err)
	}

	items := make([]Item, len(decoded.Items))
	for i, r := range decoded.Items {
		items[i] = *r.toItem()
	}
	return &ItemsResult{
		Items:      items,
		TotalCount: decoded.TotalRecordCount,
		StartIndex: decoded.StartIndex,
	}, nil
}

type jfItemResponse struct {
	ID             string            `json:"Id"`
	Name           string            `json:"Name"`
	Type           ItemType          `json:"Type"`
	ProductionYear int               `json:"ProductionYear"`
	Overview       string            `json:"Overview"`
	Genres         []string          `json:"Genres"`
	RunTimeTicks   int64             `json:"RunTimeTicks"`
	ImageTags      map[string]string `json:"ImageTags"`
	BackdropTags   []string          `json:"BackdropImageTags"`
	ProviderIDs    map[string]string `json:"ProviderIds"`
	ChildCount     int               `json:"ChildCount"`
	People         []struct {
		Name string `json:"Name"`
		Type string `json:"Type"`
	} `json:"People"`
	UserData struct {
		PlaybackPositionTicks int64 `json:"PlaybackPositionTicks"`
		Played                bool  `json:"Played"`
	} `json:"UserData"`
}

func (r *jfItemResponse) toItem() *Item {
	people := make([]Person, len(r.People))
	for i, p := range r.People {
		people[i] = Person{Name: p.Name, Role: p.Type}
	}
	return &Item{
		ID:              r.ID,
		Name:            r.Name,
		Type:            r.Type,
		Year:            r.ProductionYear,
		Overview:        r.Overview,
		Genres:          r.Genres,
		Runtime:         r.RunTimeTicks,
		PrimaryImageTag: r.ImageTags["Primary"],
		BackdropTags:    r.BackdropTags,
		ProviderIDs:     r.ProviderIDs,
		People:          people,
		ChildCount:      r.ChildCount,
		UserData: UserData{
			PlaybackPositionTicks: r.UserData.PlaybackPositionTicks,
			Played:                r.UserData.Played,
		},
	}
}
