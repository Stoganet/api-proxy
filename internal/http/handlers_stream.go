package http

import (
	"context"
	"log/slog"
	stdhttp "net/http"
	"net/http/httputil"
	"net/url"

	"github.com/Stoganet/api-proxy/internal/gen"
)

type ctxStreamKey struct{}

func newStreamHandler(authSvc authService, jellyfinBaseURL string, logger *slog.Logger) stdhttp.Handler {
	proxy := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			target, _ := pr.In.Context().Value(ctxStreamKey{}).(*url.URL)
			pr.Out.URL = target
			pr.Out.Host = target.Host
			pr.Out.Header.Del("Authorization") // do not forward client JWT to Jellyfin
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
			logger.ErrorContext(r.Context(), "stream: jellyfin unreachable", "err", err)
			w.WriteHeader(stdhttp.StatusServiceUnavailable)
		},
	}

	return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		jfID := r.PathValue("jfId")
		userID := userIDFromCtx(r.Context())

		jfToken, err := authSvc.GetJellyfinToken(r.Context(), userID)
		if err != nil {
			logger.ErrorContext(r.Context(), "stream: GetJellyfinToken failed", "err", err)
			writeError(w, r, stdhttp.StatusServiceUnavailable, gen.BackendUnavailable, "upstream error")
			return
		}

		raw, err := url.JoinPath(jellyfinBaseURL, "Videos", jfID, "stream")
		if err != nil {
			logger.ErrorContext(r.Context(), "stream: malformed jellyfin base URL", "err", err)
			writeError(w, r, stdhttp.StatusServiceUnavailable, gen.BackendUnavailable, "upstream error")
			return
		}
		target, err := url.Parse(raw)
		if err != nil {
			logger.ErrorContext(r.Context(), "stream: malformed jellyfin base URL", "err", err)
			writeError(w, r, stdhttp.StatusServiceUnavailable, gen.BackendUnavailable, "upstream error")
			return
		}
		q := target.Query()
		q.Set("Static", "true")
		q.Set("api_key", jfToken)
		target.RawQuery = q.Encode()

		ctx := context.WithValue(r.Context(), ctxStreamKey{}, target)
		proxy.ServeHTTP(w, r.WithContext(ctx))
	})
}
