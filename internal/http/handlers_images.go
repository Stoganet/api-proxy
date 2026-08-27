package http

import (
	"context"
	"log/slog"
	stdhttp "net/http"
	"net/http/httputil"
	"net/url"

	"github.com/Stoganet/api-proxy/internal/gen"
)

type ctxImageKey struct{}

// newImageHandler proxies Jellyfin item images so the client never talks to Jellyfin directly
func newImageHandler(authSvc authService, jellyfinBaseURL string, logger *slog.Logger) stdhttp.Handler {
	proxy := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			target, _ := pr.In.Context().Value(ctxImageKey{}).(*url.URL)
			pr.Out.URL = target
			pr.Out.Host = target.Host
			pr.Out.Header.Del("Authorization")
		},
		ModifyResponse: func(resp *stdhttp.Response) error {
			if resp.StatusCode >= 400 {
				resp.Body.Close()
				resp.Body = stdhttp.NoBody
				resp.ContentLength = 0
				resp.Header.Del("Content-Type")
			}
			return nil
		},
		ErrorHandler: func(w stdhttp.ResponseWriter, r *stdhttp.Request, err error) {
			logger.ErrorContext(r.Context(), "images: jellyfin unreachable", "err", err)
			w.WriteHeader(stdhttp.StatusServiceUnavailable)
		},
	}

	return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		jfID := r.PathValue("jfId")
		kind := r.PathValue("kind")
		userID := userIDFromCtx(r.Context())

		var jfPath []string
		switch kind {
		case "primary":
			jfPath = []string{"Items", jfID, "Images", "Primary"}
		case "backdrop":
			jfPath = []string{"Items", jfID, "Images", "Backdrop", "0"}
		default:
			writeError(w, r, stdhttp.StatusNotFound, gen.ItemNotFound, "unknown image kind")
			return
		}

		jfToken, err := authSvc.GetJellyfinToken(r.Context(), userID)
		if err != nil {
			logger.ErrorContext(r.Context(), "images: GetJellyfinToken failed", "err", err)
			writeError(w, r, stdhttp.StatusServiceUnavailable, gen.BackendUnavailable, "upstream error")
			return
		}

		raw, err := url.JoinPath(jellyfinBaseURL, jfPath...)
		if err != nil {
			logger.ErrorContext(r.Context(), "images: malformed jellyfin base URL", "err", err)
			writeError(w, r, stdhttp.StatusServiceUnavailable, gen.BackendUnavailable, "upstream error")
			return
		}
		target, err := url.Parse(raw)
		if err != nil {
			logger.ErrorContext(r.Context(), "images: malformed jellyfin base URL", "err", err)
			writeError(w, r, stdhttp.StatusServiceUnavailable, gen.BackendUnavailable, "upstream error")
			return
		}
		q := target.Query()
		q.Set("api_key", jfToken)
		target.RawQuery = q.Encode()

		ctx := context.WithValue(r.Context(), ctxImageKey{}, target)
		proxy.ServeHTTP(w, r.WithContext(ctx))
	})
}
