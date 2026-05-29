package http

import (
	"context"
	"errors"

	"github.com/Stoganet/api-proxy/internal/auth"
	"github.com/Stoganet/api-proxy/internal/gen"
)

func (s *Server) GetHealthz(ctx context.Context, _ gen.GetHealthzRequestObject) (gen.GetHealthzResponseObject, error) {
	return gen.GetHealthz200JSONResponse{Status: gen.Ok}, nil
}

func (s *Server) PostAuthLogin(ctx context.Context, request gen.PostAuthLoginRequestObject) (gen.PostAuthLoginResponseObject, error) {
	body := request.Body
	pair, err := s.auth.Login(ctx, body.Username, body.Password, body.DeviceLabel)
	switch {
	case errors.Is(err, auth.ErrAccountLocked):
		return gen.PostAuthLogin423JSONResponse(apiError(ctx, gen.AccountLocked, "too many failed attempts")), nil
	case errors.Is(err, auth.ErrInvalidCredentials):
		return gen.PostAuthLogin401JSONResponse(apiError(ctx, gen.InvalidCredentials, "username or password incorrect")), nil
	case errors.Is(err, auth.ErrJellyfinUnavailable):
		return gen.PostAuthLogin503JSONResponse(apiError(ctx, gen.BackendUnavailable, "jellyfin unavailable")), nil
	case err != nil:
		return nil, err
	}
	return gen.PostAuthLogin200JSONResponse(toGenTokenPair(pair)), nil
}

func (s *Server) PostAuthRefresh(ctx context.Context, request gen.PostAuthRefreshRequestObject) (gen.PostAuthRefreshResponseObject, error) {
	pair, err := s.auth.Refresh(ctx, request.Body.RefreshToken)
	switch {
	case errors.Is(err, auth.ErrTokenExpired):
		return gen.PostAuthRefresh401JSONResponse(apiError(ctx, gen.TokenExpired, "refresh token expired")), nil
	case errors.Is(err, auth.ErrTokenInvalid), errors.Is(err, auth.ErrTokenReused):
		return gen.PostAuthRefresh401JSONResponse(apiError(ctx, gen.TokenInvalid, "refresh token invalid")), nil
	case err != nil:
		return nil, err
	}
	return gen.PostAuthRefresh200JSONResponse(toGenTokenPair(pair)), nil
}

func (s *Server) PostAuthLogout(ctx context.Context, request gen.PostAuthLogoutRequestObject) (gen.PostAuthLogoutResponseObject, error) {
	err := s.auth.Logout(ctx, request.Body.RefreshToken)
	if err != nil && !errors.Is(err, auth.ErrTokenInvalid) {
		return nil, err
	}
	return gen.PostAuthLogout204Response{}, nil
}

func (s *Server) PostAuthLogoutAll(ctx context.Context, _ gen.PostAuthLogoutAllRequestObject) (gen.PostAuthLogoutAllResponseObject, error) {
	uid, _ := ctx.Value(ctxUserID).(string)
	if err := s.auth.LogoutAll(ctx, uid); err != nil {
		return nil, err
	}
	return gen.PostAuthLogoutAll204Response{}, nil
}

func (s *Server) PostAuthQuickConnectStart(ctx context.Context, _ gen.PostAuthQuickConnectStartRequestObject) (gen.PostAuthQuickConnectStartResponseObject, error) {
	out, err := s.auth.QuickConnectStart(ctx)
	if errors.Is(err, auth.ErrJellyfinUnavailable) {
		return gen.PostAuthQuickConnectStart503JSONResponse(apiError(ctx, gen.BackendUnavailable, "jellyfin unavailable")), nil
	}
	if err != nil {
		return nil, err
	}
	return gen.PostAuthQuickConnectStart200JSONResponse{Code: out.Code, PollToken: out.PollToken}, nil
}

func (s *Server) PostAuthQuickConnectPoll(ctx context.Context, request gen.PostAuthQuickConnectPollRequestObject) (gen.PostAuthQuickConnectPollResponseObject, error) {
	pair, err := s.auth.QuickConnectPoll(ctx, request.Body.PollToken)
	switch {
	case errors.Is(err, auth.ErrQuickConnectPending):
		return gen.PostAuthQuickConnectPoll202Response{}, nil
	case errors.Is(err, auth.ErrQuickConnectExpired), errors.Is(err, auth.ErrTokenInvalid):
		return gen.PostAuthQuickConnectPoll410JSONResponse(apiError(ctx, gen.TokenExpired, "quick connect expired")), nil
	case errors.Is(err, auth.ErrJellyfinUnavailable):
		return gen.PostAuthQuickConnectPoll410JSONResponse(apiError(ctx, gen.BackendUnavailable, "jellyfin unavailable")), nil
	case err != nil:
		return nil, err
	}
	return gen.PostAuthQuickConnectPoll200JSONResponse(toGenTokenPair(pair)), nil
}

// apiError builds a gen.Error with the request ID from context.
func apiError(ctx context.Context, code gen.ErrorErrorCode, message string) gen.Error {
	var e gen.Error
	e.Error.Code = code
	e.Error.Message = message
	e.RequestId = requestIDFromCtx(ctx)
	return e
}

// toGenTokenPair converts an auth.TokenPair to the generated API type.
func toGenTokenPair(p *auth.TokenPair) gen.TokenPair {
	return gen.TokenPair{
		AccessToken:  p.AccessToken,
		RefreshToken: p.RefreshToken,
		User: gen.User{
			Id:          p.User.ID,
			Email:       p.User.Email,
			DisplayName: p.User.DisplayName,
		},
	}
}
