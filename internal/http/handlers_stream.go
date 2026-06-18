package http

import (
	"fmt"
	"log/slog"
	stdhttp "net/http"
	"net/http/httputil"
	"net/url"

	"github.com/Stoganet/api-proxy/internal/gen"
)

func newStreamHandler(authSvc authService, jellyfinBaseURL string, logger *slog.Logger) stdhttp.Handler {
	return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		jfID := r.PathValue("jfId")
		userID := userIDFromCtx(r.Context())

		jfToken, err := authSvc.GetJellyfinToken(r.Context(), userID)
		if err != nil {
			logger.ErrorContext(r.Context(), "stream: GetJellyfinToken failed", "err", err)
			writeError(w, r, stdhttp.StatusServiceUnavailable, gen.BackendUnavailable, "upstream error")
			return
		}

		target, _ := url.Parse(fmt.Sprintf("%s/Videos/%s/stream", jellyfinBaseURL, jfID))
		q := target.Query()
		q.Set("Static", "true")
		q.Set("api_key", jfToken)
		target.RawQuery = q.Encode()

		proxy := &httputil.ReverseProxy{
			Director: func(req *stdhttp.Request) {
				req.URL = target
				req.Host = target.Host
				req.Header.Del("Authorization") // do not forward client JWT to Jellyfin
			},
			ErrorHandler: func(w stdhttp.ResponseWriter, r *stdhttp.Request, err error) {
				logger.ErrorContext(r.Context(), "stream: jellyfin unreachable", "err", err)
				w.WriteHeader(stdhttp.StatusServiceUnavailable)
			},
		}

		proxy.ServeHTTP(w, r)
	})
}
