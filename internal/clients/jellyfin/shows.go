package jellyfin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

type Season struct {
	ID              string
	Number          int
	Name            string
	Year            int
	EpisodeCount    int
	Overview        string
	PrimaryImageTag string
}

type Episode struct {
	ID                string
	Name              string
	IndexNumber       int
	ParentIndexNumber int
	Overview          string
	RunTimeTicks      int64
	PrimaryImageTag   string
	UserData          UserData
}

func (c *Client) GetSeasons(ctx context.Context, userID, seriesID string) ([]Season, error) {
	raw, err := url.JoinPath(c.baseURL, "Shows", seriesID, "Seasons")
	if err != nil {
		return nil, fmt.Errorf("jellyfin GetSeasons: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return nil, err
	}
	q := req.URL.Query()
	q.Set("UserId", userID)
	q.Set("Fields", "Overview")
	req.URL.RawQuery = q.Encode()
	req.Header.Set("X-Emby-Authorization", authHeader(userID))
	req.Header.Set("X-Emby-Token", c.apiKey)

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jellyfin GetSeasons: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrItemNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jellyfin GetSeasons: unexpected status %d", resp.StatusCode)
	}

	var body struct {
		Items []jfSeasonResponse `json:"Items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("jellyfin GetSeasons: decode: %w", err)
	}

	seasons := make([]Season, 0, len(body.Items))
	for _, s := range body.Items {
		if s.IndexNumber == 0 {
			continue // skip Specials
		}
		seasons = append(seasons, s.toSeason())
	}
	return seasons, nil
}

func (c *Client) GetEpisodes(ctx context.Context, userID, seriesID string, seasonNumber int) ([]Episode, error) {
	raw, err := url.JoinPath(c.baseURL, "Shows", seriesID, "Episodes")
	if err != nil {
		return nil, fmt.Errorf("jellyfin GetEpisodes: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return nil, err
	}
	q := req.URL.Query()
	q.Set("UserId", userID)
	q.Set("SeasonNumber", fmt.Sprintf("%d", seasonNumber))
	q.Set("Fields", "Overview,UserData,RunTimeTicks")
	req.URL.RawQuery = q.Encode()
	req.Header.Set("X-Emby-Authorization", authHeader(userID))
	req.Header.Set("X-Emby-Token", c.apiKey)

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jellyfin GetEpisodes: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrItemNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jellyfin GetEpisodes: unexpected status %d", resp.StatusCode)
	}

	var body struct {
		Items []jfEpisodeResponse `json:"Items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("jellyfin GetEpisodes: decode: %w", err)
	}

	episodes := make([]Episode, len(body.Items))
	for i, e := range body.Items {
		episodes[i] = e.toEpisode()
	}
	return episodes, nil
}

func (c *Client) GetNextUp(ctx context.Context, userID, seriesID string) (*Episode, error) {
	raw, err := url.JoinPath(c.baseURL, "Shows", "NextUp")
	if err != nil {
		return nil, fmt.Errorf("jellyfin GetNextUp: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return nil, err
	}
	q := req.URL.Query()
	q.Set("UserId", userID)
	q.Set("SeriesId", seriesID)
	q.Set("Fields", "UserData")
	req.URL.RawQuery = q.Encode()
	req.Header.Set("X-Emby-Authorization", authHeader(userID))
	req.Header.Set("X-Emby-Token", c.apiKey)

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jellyfin GetNextUp: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jellyfin GetNextUp: unexpected status %d", resp.StatusCode)
	}

	var body struct {
		Items []jfEpisodeResponse `json:"Items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("jellyfin GetNextUp: decode: %w", err)
	}

	if len(body.Items) == 0 {
		return nil, nil // nothing to resume
	}
	ep := body.Items[0].toEpisode()
	return &ep, nil
}

type jfSeasonResponse struct {
	ID             string            `json:"Id"`
	Name           string            `json:"Name"`
	IndexNumber    int               `json:"IndexNumber"`
	ProductionYear int               `json:"ProductionYear"`
	ChildCount     int               `json:"ChildCount"`
	Overview       string            `json:"Overview"`
	ImageTags      map[string]string `json:"ImageTags"`
}

func (r *jfSeasonResponse) toSeason() Season {
	return Season{
		ID:              r.ID,
		Number:          r.IndexNumber,
		Name:            r.Name,
		Year:            r.ProductionYear,
		EpisodeCount:    r.ChildCount,
		Overview:        r.Overview,
		PrimaryImageTag: r.ImageTags["Primary"],
	}
}

type jfEpisodeResponse struct {
	ID                string            `json:"Id"`
	Name              string            `json:"Name"`
	IndexNumber       int               `json:"IndexNumber"`
	ParentIndexNumber int               `json:"ParentIndexNumber"`
	Overview          string            `json:"Overview"`
	RunTimeTicks      int64             `json:"RunTimeTicks"`
	ImageTags         map[string]string `json:"ImageTags"`
	UserData          struct {
		PlaybackPositionTicks int64 `json:"PlaybackPositionTicks"`
		Played                bool  `json:"Played"`
	} `json:"UserData"`
}

func (r *jfEpisodeResponse) toEpisode() Episode {
	return Episode{
		ID:                r.ID,
		Name:              r.Name,
		IndexNumber:       r.IndexNumber,
		ParentIndexNumber: r.ParentIndexNumber,
		Overview:          r.Overview,
		RunTimeTicks:      r.RunTimeTicks,
		PrimaryImageTag:   r.ImageTags["Primary"],
		UserData: UserData{
			PlaybackPositionTicks: r.UserData.PlaybackPositionTicks,
			Played:                r.UserData.Played,
		},
	}
}
