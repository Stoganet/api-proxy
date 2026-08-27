package http

import (
	"log/slog"
	stdhttp "net/http"
	"net/url"
)

func newSubtitleHandler(authSvc authService, jellyfinBaseURL string, logger *slog.Logger) stdhttp.Handler {
	return newProxyHandler("subtitles", authSvc, logger, func(r *stdhttp.Request, jfToken string) (*url.URL, error) {
		jfID := r.PathValue("jfId")
		index := r.PathValue("index")

		// Jellyfin addresses subtitles as /Videos/{itemId}/{mediaSourceId}/Subtitles/{index}/Stream.vtt.
		// For single-file items the item ID doubles as the media source ID.
		raw, err := url.JoinPath(jellyfinBaseURL, "Videos", jfID, jfID, "Subtitles", index, "Stream.vtt")
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
}
