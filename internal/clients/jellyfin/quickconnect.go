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
	connectRaw, err := url.JoinPath(c.baseURL, "QuickConnect", "Connect")
	if err != nil {
		return nil, err
	}
	connectURL, err := url.Parse(connectRaw)
	if err != nil {
		return nil, err
	}
	q := connectURL.Query()
	q.Set("secret", secret)
	connectURL.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, connectURL.String(), nil)
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
		return nil, fmt.Errorf("%w: Connect status %d", ErrUpstreamUnavailable, resp.StatusCode)
	}

	var state struct {
		Authenticated bool `json:"Authenticated"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&state); err != nil {
		return nil, fmt.Errorf("%w: decode Connect: %v", ErrUpstreamUnavailable, err)
	}
	if !state.Authenticated {
		return nil, ErrQuickConnectPending
	}

	authRaw, err := url.JoinPath(c.baseURL, "Users", "AuthenticateWithQuickConnect")
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(map[string]string{"Secret": secret})
	if err != nil {
		return nil, err
	}

	req2, err := http.NewRequestWithContext(ctx, http.MethodPost, authRaw, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("X-Emby-Authorization", authHeader("api-proxy-qc"))

	resp2, err := c.hc.Do(req2)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUpstreamUnavailable, err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: AuthenticateWithQuickConnect status %d", ErrUpstreamUnavailable, resp2.StatusCode)
	}

	var parsed struct {
		AccessToken string `json:"AccessToken"`
		User        struct {
			ID   string `json:"Id"`
			Name string `json:"Name"`
		} `json:"User"`
	}
	if err := json.NewDecoder(resp2.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("%w: decode auth: %v", ErrUpstreamUnavailable, err)
	}
	return &AuthResult{
		AccessToken: parsed.AccessToken,
		UserID:      parsed.User.ID,
		Username:    parsed.User.Name,
	}, nil
}
