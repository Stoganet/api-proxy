package jellyfin

import (
	"net/http"
	"time"
)

// clientVersion is the version string reported to Jellyfin in the
// X-Emby-Authorization header. Update here when the client contract changes.
const clientVersion = "0.1.0"

type Client struct {
	baseURL string
	apiKey  string
	hc      *http.Client
}

func New(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		hc:      &http.Client{Timeout: 5 * time.Second},
	}
}

func authHeader(deviceID string) string {
	return `MediaBrowser Client="api-proxy", Device="api-proxy", DeviceId="` +
		deviceID + `", Version="` + clientVersion + `"`
}
