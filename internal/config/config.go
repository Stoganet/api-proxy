package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
)

type envMap map[string]string

// Config holds all runtime configuration for the auth surface.
// Future phases will extend this with Sonarr/Radarr/qBit/Jellyseerr/TMDB fields.
type Config struct {
	JellyfinURL    string
	JellyfinAPIKey string
	JWTSigningKey  []byte
	DBPath         string
	ListenAddr     string
	ProxyBaseURL   string
}

func Load(override envMap) (*Config, error) {
	get := func(k string) string {
		if override != nil {
			if v, ok := override[k]; ok {
				return v
			}
		}
		return os.Getenv(k)
	}

	required := []string{
		"JELLYFIN_URL",
		"JELLYFIN_API_KEY",
		"JWT_SIGNING_KEY",
		"DB_PATH",
		"LISTEN_ADDR",
		"PROXY_BASE_URL",
	}
	var missing []string
	for _, k := range required {
		if get(k) == "" {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required env vars: %v", missing)
	}

	key := []byte(get("JWT_SIGNING_KEY"))
	if len(key) < 32 {
		return nil, errors.New("JWT_SIGNING_KEY must be at least 32 bytes")
	}

	if err := validateURL("JELLYFIN_URL", get("JELLYFIN_URL")); err != nil {
		return nil, err
	}
	if err := validateURL("PROXY_BASE_URL", get("PROXY_BASE_URL")); err != nil {
		return nil, err
	}

	return &Config{
		JellyfinURL:    get("JELLYFIN_URL"),
		JellyfinAPIKey: get("JELLYFIN_API_KEY"),
		JWTSigningKey:  key,
		DBPath:         get("DB_PATH"),
		ListenAddr:     get("LISTEN_ADDR"),
		ProxyBaseURL:   get("PROXY_BASE_URL"),
	}, nil
}

func validateURL(name, raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("%s must be an absolute URL, got %q", name, raw)
	}
	return nil
}

// LoadFromEnv reads the real process environment.
func LoadFromEnv() (*Config, error) { return Load(nil) }
