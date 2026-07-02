package jellyfin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
)

var ErrQuickConnectPending = errors.New("jellyfin: quick connect not yet approved")

func IsPending(err error) bool { return errors.Is(err, ErrQuickConnectPending) }

type QuickConnectInitiation struct {
	Secret string
	Code   string
}

func (c *Client) QuickConnectInitiate(ctx context.Context) (*QuickConnectInitiation, error) {
	raw, err := url.JoinPath(c.baseURL, "QuickConnect", "Initiate")
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, raw, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Emby-Authorization", authHeader("api-proxy-qc"))

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUpstreamUnavailable, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status %d", ErrUpstreamUnavailable, resp.StatusCode)
	}
	var parsed struct {
		Secret string `json:"Secret"`
		Code   string `json:"Code"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("%w: decode: %v", ErrUpstreamUnavailable, err)
	}
	return &QuickConnectInitiation{Secret: parsed.Secret, Code: parsed.Code}, nil
}

func (c *Client) QuickConnectAuthenticate(ctx context.Context, secret string) (*AuthResult, error) {
	raw, err := url.JoinPath(c.baseURL, "QuickConnect", "Authenticate")
	if err != nil {
		return nil, err
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("secret", secret)
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Emby-Authorization", authHeader("api-proxy-qc"))

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUpstreamUnavailable, err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusBadRequest:
		return nil, ErrQuickConnectPending
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
