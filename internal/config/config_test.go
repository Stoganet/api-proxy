package config

import (
	"testing"
)

func TestLoad_MissingRequiredVarFatal(t *testing.T) {
	t.Setenv("JELLYFIN_URL", "")
	_, err := Load(envMap{})
	if err == nil {
		t.Fatal("expected error for missing JELLYFIN_URL")
	}
}

func TestLoad_AllRequiredPresent(t *testing.T) {
	env := envMap{
		"JELLYFIN_URL":     "http://jellyfin:8096",
		"JELLYFIN_API_KEY": "abc",
		"JWT_SIGNING_KEY":  "0123456789abcdef0123456789abcdef",
		"DB_PATH":          "/tmp/api-proxy.sqlite",
		"LISTEN_ADDR":      ":8080",
		"PROXY_BASE_URL":   "https://api.stoganet.com",
	}
	c, err := Load(env)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if c.JellyfinURL != "http://jellyfin:8096" {
		t.Errorf("got %q", c.JellyfinURL)
	}
	if c.ProxyBaseURL != "https://api.stoganet.com" {
		t.Errorf("got %q", c.ProxyBaseURL)
	}
}

func TestLoad_MissingProxyBaseURL_ReturnsError(t *testing.T) {
	env := envMap{
		"JELLYFIN_URL":     "http://jellyfin:8096",
		"JELLYFIN_API_KEY": "abc",
		"JWT_SIGNING_KEY":  "0123456789abcdef0123456789abcdef",
		"DB_PATH":          "/tmp/api-proxy.sqlite",
		"LISTEN_ADDR":      ":8080",
		// PROXY_BASE_URL intentionally absent
	}
	if _, err := Load(env); err == nil {
		t.Fatal("expected error for missing PROXY_BASE_URL")
	}
}

func TestLoad_JWTKeyTooShort(t *testing.T) {
	env := envMap{
		"JELLYFIN_URL":     "http://jellyfin:8096",
		"JELLYFIN_API_KEY": "abc",
		"JWT_SIGNING_KEY":  "short",
		"DB_PATH":          "/tmp/api-proxy.sqlite",
		"LISTEN_ADDR":      ":8080",
		"PROXY_BASE_URL":   "https://api.stoganet.com",
	}
	if _, err := Load(env); err == nil {
		t.Fatal("expected error for short JWT_SIGNING_KEY")
	}
}

func TestLoad_InvalidURLs(t *testing.T) {
	base := envMap{
		"JELLYFIN_API_KEY": "abc",
		"JWT_SIGNING_KEY":  "0123456789abcdef0123456789abcdef",
		"DB_PATH":          "/tmp/api-proxy.sqlite",
		"LISTEN_ADDR":      ":8080",
	}

	cases := []struct {
		name string
		key  string
		val  string
	}{
		{"jellyfin relative path", "JELLYFIN_URL", "/not/absolute"},
		{"jellyfin no scheme", "JELLYFIN_URL", "jellyfin:8096"},
		{"proxy relative path", "PROXY_BASE_URL", "/not/absolute"},
		{"proxy no scheme", "PROXY_BASE_URL", "api.stoganet.com"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := envMap{}
			for k, v := range base {
				env[k] = v
			}
			env["JELLYFIN_URL"] = "http://jellyfin:8096"
			env["PROXY_BASE_URL"] = "https://api.stoganet.com"
			env[tc.key] = tc.val

			if _, err := Load(env); err == nil {
				t.Fatalf("expected error for %s=%q", tc.key, tc.val)
			}
		})
	}
}
