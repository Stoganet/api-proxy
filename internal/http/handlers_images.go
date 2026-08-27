package http

import (
	"log/slog"
	stdhttp "net/http"
	"net/url"

	"github.com/Stoganet/api-proxy/internal/gen"
)

// newImageHandler proxies Jellyfin item images so the client never talks to Jellyfin directly
// (it only knows the internal-only Docker address, e.g. http://jellyfin:8096, which isn't
// reachable from outside the compose network).
func newImageHandler(authSvc authService, jellyfinBaseURL string, logger *slog.Logger) stdhttp.Handler {
	proxy := newProxyHandler("images", authSvc, logger, func(r *stdhttp.Request, jfToken string) (*url.URL, error) {
		jfID := r.PathValue("jfId")
		jfPath := imageJfPath(r.PathValue("kind"), jfID)

		raw, err := url.JoinPath(jellyfinBaseURL, jfPath...)
		if err != nil {
			return nil, err
		}
		target, err := url.Parse(raw)
		if err != nil {
			return nil, err
		}
		q := target.Query()
		q.Set("api_key", jfToken)
		target.RawQuery = q.Encode()
		return target, nil
	})

	return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		switch r.PathValue("kind") {
		case "primary", "backdrop":
			proxy.ServeHTTP(w, r)
		default:
			writeError(w, r, stdhttp.StatusNotFound, gen.ItemNotFound, "unknown image kind")
		}
	})
}

func imageJfPath(kind, jfID string) []string {
	if kind == "backdrop" {
		return []string{"Items", jfID, "Images", "Backdrop", "0"}
	}
	return []string{"Items", jfID, "Images", "Primary"}
}
