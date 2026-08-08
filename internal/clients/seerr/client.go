package seerr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	baseURL string
	apiKey  string
	hc      *http.Client
}

func New(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		hc:      &http.Client{Timeout: 10 * time.Second},
	}
}

// Status mirrors Seerr's MediaInfo.status codes.
type Status int

const (
	StatusUnknown            Status = 1
	StatusPending            Status = 2
	StatusProcessing         Status = 3
	StatusPartiallyAvailable Status = 4
	StatusAvailable          Status = 5
	StatusDeleted            Status = 6
)

type MediaInfo struct {
	Status Status
}

type SearchResult struct {
	TmdbID       int
	MediaType    string
	Title        string
	Overview     string
	PosterPath   string
	BackdropPath string
	ReleaseDate  string
	MediaInfo    *MediaInfo
}

func (c *Client) Search(ctx context.Context, query string) ([]SearchResult, error) {
	raw, err := url.JoinPath(c.baseURL, "api", "v1", "search")
	if err != nil {
		return nil, fmt.Errorf("seerr Search: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return nil, err
	}
	q := req.URL.Query()
	q.Set("query", query)
	// Seerr rejects '+' for space, but Encode() always uses it. A literal '+' in the input
	// is escaped to '%2B' by Encode(), so the remaining '+' only ever means space.
	req.URL.RawQuery = strings.ReplaceAll(q.Encode(), "+", "%20")
	req.Header.Set("X-Api-Key", c.apiKey)

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("seerr Search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("seerr Search: unexpected status %d", resp.StatusCode)
	}

	var decoded struct {
		Results []seerrSearchResult `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("seerr Search: decode: %w", err)
	}

	results := make([]SearchResult, 0, len(decoded.Results))
	for _, r := range decoded.Results {
		if r.MediaType != "movie" {
			continue
		}
		results = append(results, r.toSearchResult())
	}
	return results, nil
}

func (c *Client) RequestMovie(ctx context.Context, tmdbID int) error {
	raw, err := url.JoinPath(c.baseURL, "api", "v1", "request")
	if err != nil {
		return fmt.Errorf("seerr RequestMovie: %w", err)
	}
	body, err := json.Marshal(struct {
		MediaType string `json:"mediaType"`
		MediaID   int    `json:"mediaId"`
	}{
		MediaType: "movie",
		MediaID:   tmdbID,
	})
	if err != nil {
		return fmt.Errorf("seerr RequestMovie: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, raw, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", c.apiKey)

	resp, err := c.hc.Do(req)
	if err != nil {
		return fmt.Errorf("seerr RequestMovie: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("seerr RequestMovie: unexpected status %d", resp.StatusCode)
	}
	return nil
}

type seerrSearchResult struct {
	ID           int    `json:"id"`
	MediaType    string `json:"mediaType"`
	Title        string `json:"title"`
	Overview     string `json:"overview"`
	PosterPath   string `json:"posterPath"`
	BackdropPath string `json:"backdropPath"`
	ReleaseDate  string `json:"releaseDate"`
	MediaInfo    *struct {
		Status Status `json:"status"`
	} `json:"mediaInfo"`
}

func (r seerrSearchResult) toSearchResult() SearchResult {
	sr := SearchResult{
		TmdbID:       r.ID,
		MediaType:    r.MediaType,
		Title:        r.Title,
		Overview:     r.Overview,
		PosterPath:   r.PosterPath,
		BackdropPath: r.BackdropPath,
		ReleaseDate:  r.ReleaseDate,
	}
	if r.MediaInfo != nil {
		sr.MediaInfo = &MediaInfo{Status: r.MediaInfo.Status}
	}
	return sr
}
