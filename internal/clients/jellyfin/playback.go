package jellyfin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

const subtitleStreamType = "Subtitle"

type SubtitleTrack struct {
	Index      int
	Language   string
	Title      string
	Codec      string
	IsDefault  bool
	IsForced   bool
	IsExternal bool
}

func (c *Client) GetSubtitleTracks(ctx context.Context, userID, itemID string) ([]SubtitleTrack, error) {
	raw, err := url.JoinPath(c.baseURL, "Items", itemID, "PlaybackInfo")
	if err != nil {
		return nil, fmt.Errorf("jellyfin GetSubtitleTracks: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return nil, err
	}
	q := req.URL.Query()
	q.Set("UserId", userID)
	req.URL.RawQuery = q.Encode()
	req.Header.Set("X-Emby-Authorization", authHeader(userID))
	req.Header.Set("X-Emby-Token", c.apiKey)

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jellyfin GetSubtitleTracks: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrItemNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jellyfin GetSubtitleTracks: unexpected status %d", resp.StatusCode)
	}

	var body struct {
		MediaSources []struct {
			MediaStreams []jfMediaStream `json:"MediaStreams"`
		} `json:"MediaSources"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("jellyfin GetSubtitleTracks: decode: %w", err)
	}
	if len(body.MediaSources) == 0 {
		return nil, nil
	}

	tracks := make([]SubtitleTrack, 0, len(body.MediaSources[0].MediaStreams))
	for _, ms := range body.MediaSources[0].MediaStreams {
		if ms.Type != subtitleStreamType {
			continue
		}
		tracks = append(tracks, ms.toSubtitleTrack())
	}
	return tracks, nil
}

type jfMediaStream struct {
	Type         string `json:"Type"`
	Index        int    `json:"Index"`
	Language     string `json:"Language"`
	DisplayTitle string `json:"DisplayTitle"`
	Codec        string `json:"Codec"`
	IsDefault    bool   `json:"IsDefault"`
	IsForced     bool   `json:"IsForced"`
	IsExternal   bool   `json:"IsExternal"`
}

func (m jfMediaStream) toSubtitleTrack() SubtitleTrack {
	return SubtitleTrack{
		Index:      m.Index,
		Language:   m.Language,
		Title:      m.DisplayTitle,
		Codec:      m.Codec,
		IsDefault:  m.IsDefault,
		IsForced:   m.IsForced,
		IsExternal: m.IsExternal,
	}
}
