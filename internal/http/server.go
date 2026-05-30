package http

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	stdhttp "net/http"
	"strings"

	"github.com/Stoganet/api-proxy/internal/auth"
	"github.com/Stoganet/api-proxy/internal/gen"
	"github.com/golang-jwt/jwt/v5"
)

type authService interface {
	Login(ctx context.Context, username, password string, deviceLabel *string) (*auth.TokenPair, error)
	Refresh(ctx context.Context, plaintext string) (*auth.TokenPair, error)
	Logout(ctx context.Context, plaintext string) error
	LogoutAll(ctx context.Context, userID string) error
	QuickConnectStart(ctx context.Context) (*auth.QuickConnectStartOut, error)
	QuickConnectPoll(ctx context.Context, pollToken string) (*auth.TokenPair, error)
	VerifyJWT(token string) (*auth.Claims, error)
}

type Server struct {
	auth   authService
	logger *slog.Logger
}

func NewServer(authSvc *auth.Service, logger *slog.Logger) stdhttp.Handler {
	s := &Server{auth: authSvc, logger: logger}

	strict := gen.NewStrictHandlerWithOptions(s, []gen.StrictMiddlewareFunc{
		jwtStrictMiddleware(authSvc),
	}, gen.StrictHTTPServerOptions{
		ResponseErrorHandlerFunc: func(w stdhttp.ResponseWriter, r *stdhttp.Request, err error) {
			s.logger.ErrorContext(r.Context(), "handler error", "err", err, "request_id", requestIDFromCtx(r.Context()))
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
// Writes 401 directly and returns nil to prevent the strict handler from
// writing a second response.
func jwtStrictMiddleware(svc authService) gen.StrictMiddlewareFunc {
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
				writeError(w, r, stdhttp.StatusUnauthorized, gen.TokenInvalid, "missing bearer token")
				return nil, nil
			}
			tok := strings.TrimPrefix(h, "Bearer ")
			claims, err := svc.VerifyJWT(tok)
			if err != nil {
				code := gen.TokenInvalid
				if errors.Is(err, jwt.ErrTokenExpired) {
					code = gen.TokenExpired
				}
				writeError(w, r, stdhttp.StatusUnauthorized, code, "invalid or expired token")
				return nil, nil //nolint:nilerr // error handled by writing 401 directly
			}
			ctx = context.WithValue(ctx, ctxUserID, claims.UserID)
			ctx = context.WithValue(ctx, ctxJFUserID, claims.JFUserID)
			return f(ctx, w, r, req)
		}
	}
}
