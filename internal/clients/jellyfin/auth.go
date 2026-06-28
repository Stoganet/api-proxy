package jellyfin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
)

type AuthResult struct {
	AccessToken string
	UserID      string
	Username    string
}

var (
	ErrInvalidCredentials  = errors.New("jellyfin: invalid credentials")
	ErrUpstreamUnavailable = errors.New("jellyfin: upstream unavailable")
	ErrItemNotFound        = errors.New("jellyfin: item not found")
)

func IsInvalidCredentials(err error) bool  { return errors.Is(err, ErrInvalidCredentials) }
func IsUpstreamUnavailable(err error) bool { return errors.Is(err, ErrUpstreamUnavailable) }

func (c *Client) AuthenticateByName(ctx context.Context, username, password string) (*AuthResult, error) {
	body, _ := json.Marshal(map[string]string{"Username": username, "Pw": password})
	raw, err := url.JoinPath(c.baseURL, "Users", "AuthenticateByName")
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, raw, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Emby-Authorization", authHeader("api-proxy-login"))

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUpstreamUnavailable, err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, ErrInvalidCredentials
	default:
		return nil, fmt.Errorf("%w: status %d", ErrUpstreamUnavailable, resp.StatusCode)
	}

	var parsed struct {
		AccessToken string `json:"AccessToken"`
		User        struct {
			ID   string `json:"Id"`
			Name string `json:"Name"`
		} `json:"User"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("%w: decode: %v", ErrUpstreamUnavailable, err)
	}
	return &AuthResult{
		AccessToken: parsed.AccessToken,
		UserID:      parsed.User.ID,
		Username:    parsed.User.Name,
	}, nil
}
