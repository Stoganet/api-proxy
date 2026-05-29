package http

import (
	"context"
	"encoding/json"
	"log/slog"
	stdhttp "net/http"
	"strings"

	"github.com/Stoganet/api-proxy/internal/auth"
	"github.com/Stoganet/api-proxy/internal/gen"
)

type Server struct {
	auth   *auth.Service
	logger *slog.Logger
}

func NewServer(authSvc *auth.Service, logger *slog.Logger) stdhttp.Handler {
	s := &Server{auth: authSvc, logger: logger}

	strict := gen.NewStrictHandlerWithOptions(s, []gen.StrictMiddlewareFunc{
		jwtStrictMiddleware(authSvc),
	}, gen.StrictHTTPServerOptions{
		ResponseErrorHandlerFunc: func(w stdhttp.ResponseWriter, r *stdhttp.Request, err error) {
			var e gen.Error
			e.Error.Code = gen.Internal
			e.Error.Message = "internal error"
			e.RequestId = requestIDFromCtx(r.Context())
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(stdhttp.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(e)
		},
	})

	return RequestID(Logging(logger)(gen.Handler(strict)))
}

// jwtStrictMiddleware enforces Bearer JWT auth on logout operations only.
// It runs before the handler and writes a 401 directly if the token is
// missing or invalid, then returns nil to stop the strict handler from
// writing a second response.
func jwtStrictMiddleware(authSvc *auth.Service) gen.StrictMiddlewareFunc {
	protected := map[string]bool{
		"postAuthLogout":    true,
		"postAuthLogoutAll": true,
	}
	return func(f gen.StrictHandlerFunc, operationID string) gen.StrictHandlerFunc {
		if !protected[operationID] {
			return f
		}
		return func(ctx context.Context, w stdhttp.ResponseWriter, r *stdhttp.Request, req any) (any, error) {
			h := r.Header.Get("Authorization")
			if !strings.HasPrefix(h, "Bearer ") {
				writeError(w, r, stdhttp.StatusUnauthorized, gen.TokenExpired, "missing bearer token")
				return nil, nil
			}
			tok := strings.TrimPrefix(h, "Bearer ")
			claims, err := authSvc.VerifyJWT(tok)
			if err != nil {
				writeError(w, r, stdhttp.StatusUnauthorized, gen.TokenExpired, "invalid or expired token")
				return nil, nil
			}
			ctx = context.WithValue(ctx, ctxUserID, claims.UserID)
			ctx = context.WithValue(ctx, ctxJFUserID, claims.JFUserID)
			return f(ctx, w, r, req)
		}
	}
}
