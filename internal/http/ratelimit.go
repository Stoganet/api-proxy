package http

import (
	"context"
	"net"
	stdhttp "net/http"
	"time"

	"github.com/Stoganet/api-proxy/internal/gen"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
)

func rateLimitedResponse(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	writeError(w, r, stdhttp.StatusTooManyRequests, gen.RateLimited, "rate limit exceeded")
}

func rateLimitKey(r *stdhttp.Request) (string, error) {
	ip := middleware.GetClientIP(r.Context())
	if ip == "" {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = r.RemoteAddr
		}
		ip = host
	}
	return httprate.CanonicalizeIP(ip), nil
}

func stripUntrustedForwardedFor(next stdhttp.Handler) stdhttp.Handler {
	return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if !fromTraefik(r) {
			r.Header.Del("X-Forwarded-For")
		}
		next.ServeHTTP(w, r)
	})
}

var lookupHost = net.LookupHost

func fromTraefik(r *stdhttp.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ips, err := lookupHost("traefik")
	if err != nil {
		return false
	}
	for _, ip := range ips {
		if ip == host {
			return true
		}
	}
	return false
}

func rateLimitStrictMiddleware() (gen.StrictMiddlewareFunc, func(stdhttp.Handler) stdhttp.Handler) {
	return newRateLimitStrictMiddleware(30, 15, 60, time.Minute)
}

func newRateLimitStrictMiddleware(pollN, unauthN, authedN int, window time.Duration) (gen.StrictMiddlewareFunc, func(stdhttp.Handler) stdhttp.Handler) {
	opts := []httprate.Option{httprate.WithLimitHandler(rateLimitedResponse)}
	pollLimit := httprate.LimitBy(pollN, window, rateLimitKey, opts...)
	unauthLimit := httprate.LimitBy(unauthN, window, rateLimitKey, opts...)
	authedLimit := httprate.LimitBy(authedN, window, rateLimitKey, opts...)

	exempt := map[string]bool{
		"GetHealthz": true,
	}
	pollOps := map[string]bool{
		"PostAuthQuickConnectPoll": true,
	}
	unauthOps := map[string]bool{
		"PostAuthLogin":             true,
		"PostAuthRefresh":           true,
		"PostAuthQuickConnectStart": true,
	}

	mw := func(f gen.StrictHandlerFunc, operationID string) gen.StrictHandlerFunc {
		if exempt[operationID] {
			return f
		}

		limit := authedLimit
		switch {
		case pollOps[operationID]:
			limit = pollLimit
		case unauthOps[operationID]:
			limit = unauthLimit
		}

		return func(ctx context.Context, w stdhttp.ResponseWriter, r *stdhttp.Request, req any) (any, error) {
			var res any
			var resErr error
			called := false

			inner := stdhttp.HandlerFunc(func(w2 stdhttp.ResponseWriter, r2 *stdhttp.Request) {
				called = true
				res, resErr = f(ctx, w2, r2, req)
			})
			limit(inner).ServeHTTP(w, r.WithContext(ctx))

			if !called {
				return nil, nil
			}
			return res, resErr
		}
	}

	return mw, authedLimit
}
