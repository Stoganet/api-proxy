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
	"github.com/Stoganet/api-proxy/internal/media"
	"github.com/go-chi/chi/v5/middleware"
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
	GetJellyfinToken(ctx context.Context, userID string) (string, error)
}

type libraryService interface {
	GetItem(ctx context.Context, jfUserID, itemID string) (*media.Detail, error)
	List(ctx context.Context, jfUserID string, opts media.ListOpts) (*media.ListResult, error)
	Home(ctx context.Context, jfUserID string) (*media.HomeResult, error)
	GetEpisodes(ctx context.Context, jfUserID, showID string, seasonNumber int) ([]media.Episode, error)
	ReportProgress(ctx context.Context, jfUserID, itemID string, positionMS int64, played bool) error
	Search(ctx context.Context, query string) ([]media.Item, error)
	RequestMovie(ctx context.Context, catalogID string) error
}

type Server struct {
	auth    authService
	library libraryService
	logger  *slog.Logger
}

func NewServer(authSvc *auth.Service, libSvc *media.Service, jellyfinBaseURL string, logger *slog.Logger) stdhttp.Handler {
	s := &Server{auth: authSvc, library: libSvc, logger: logger}

	rateLimitMW, authedRateLimit := rateLimitStrictMiddleware()

	strict := gen.NewStrictHandlerWithOptions(s, []gen.StrictMiddlewareFunc{
		jwtStrictMiddleware(authSvc),
		rateLimitMW,
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

	mux := stdhttp.NewServeMux()
	mux.Handle("GET /stream/{jfId}", authedRateLimit(requireJWT(authSvc, newStreamHandler(authSvc, jellyfinBaseURL, logger))))
	mux.Handle("GET /stream/{jfId}/subtitles/{index}", authedRateLimit(requireJWT(authSvc, newSubtitleHandler(authSvc, jellyfinBaseURL, logger))))
	mux.Handle("/", gen.Handler(strict))

	return stripUntrustedForwardedFor(middleware.ClientIPFromXFF()(RequestID(Logging(logger)(mux))))
}

func jwtStrictMiddleware(svc authService) gen.StrictMiddlewareFunc {
	public := map[string]bool{
		"GetHealthz":                true,
		"PostAuthLogin":             true,
		"PostAuthRefresh":           true,
		"PostAuthQuickConnectStart": true,
		"PostAuthQuickConnectPoll":  true,
	}
	return func(f gen.StrictHandlerFunc, operationID string) gen.StrictHandlerFunc {
		if public[operationID] {
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
				return nil, nil //nolint:nilerr
			}
			ctx = context.WithValue(ctx, ctxUserID, claims.UserID)
			ctx = context.WithValue(ctx, ctxJFUserID, claims.JFUserID)
			return f(ctx, w, r, req)
		}
	}
}
