package http

import (
	"context"
	"log/slog"
	stdhttp "net/http"
	"net/http/httputil"
	"net/url"

	"github.com/Stoganet/api-proxy/internal/gen"
)

type ctxProxyTargetKey struct{}

// newProxyHandler is the shared reverse-proxy skeleton behind stream, subtitles, and images.
func newProxyHandler(
	name string,
	authSvc authService,
	logger *slog.Logger,
	buildTarget func(r *stdhttp.Request, jfToken string) (*url.URL, error),
) stdhttp.Handler {
	proxy := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			target, _ := pr.In.Context().Value(ctxProxyTargetKey{}).(*url.URL)
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
			logger.ErrorContext(r.Context(), name+": jellyfin unreachable", "err", err)
			w.WriteHeader(stdhttp.StatusServiceUnavailable)
		},
	}

	return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		userID := userIDFromCtx(r.Context())

		jfToken, err := authSvc.GetJellyfinToken(r.Context(), userID)
		if err != nil {
			logger.ErrorContext(r.Context(), name+": GetJellyfinToken failed", "err", err)
			writeError(w, r, stdhttp.StatusServiceUnavailable, gen.BackendUnavailable, "upstream error")
			return
		}

		target, err := buildTarget(r, jfToken)
		if err != nil {
			logger.ErrorContext(r.Context(), name+": malformed jellyfin base URL", "err", err)
			writeError(w, r, stdhttp.StatusServiceUnavailable, gen.BackendUnavailable, "upstream error")
			return
		}

		ctx := context.WithValue(r.Context(), ctxProxyTargetKey{}, target)
		proxy.ServeHTTP(w, r.WithContext(ctx))
	})
}
