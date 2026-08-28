package http

import (
	"log/slog"
	stdhttp "net/http"
	"net/url"
)

func newStreamHandler(authSvc authService, jellyfinBaseURL string, logger *slog.Logger) stdhttp.Handler {
	return newProxyHandler("stream", authSvc, logger, func(r *stdhttp.Request, jfToken string) (*url.URL, error) {
		jfID := r.PathValue("jfId")
		raw, err := url.JoinPath(jellyfinBaseURL, "Videos", jfID, "stream")
		if err != nil {
			return nil, err
		}
		target, err := url.Parse(raw)
		if err != nil {
			return nil, err
		}
		q := target.Query()
		q.Set("Static", "true")
		q.Set("api_key", jfToken)
		target.RawQuery = q.Encode()
		return target, nil
	})
}
