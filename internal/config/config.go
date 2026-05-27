package config

import (
	"errors"
	"fmt"
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

	return &Config{
		JellyfinURL:    get("JELLYFIN_URL"),
		JellyfinAPIKey: get("JELLYFIN_API_KEY"),
		JWTSigningKey:  key,
		DBPath:         get("DB_PATH"),
		ListenAddr:     get("LISTEN_ADDR"),
	}, nil
}

// LoadFromEnv reads the real process environment.
func LoadFromEnv() (*Config, error) { return Load(nil) }
