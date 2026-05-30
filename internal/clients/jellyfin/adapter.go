package jellyfin

import (
	"context"
	"errors"

	"github.com/Stoganet/api-proxy/internal/auth"
)

var _ auth.JellyfinAuthenticator = (*authAdapter)(nil)

type authAdapter struct{ c *Client }

// AsAuthAdapter wraps a *Client so it can be passed to auth.NewService.
// Translates jellyfin.Err* sentinels into auth.Err*.
func AsAuthAdapter(c *Client) auth.JellyfinAuthenticator { return &authAdapter{c} }

func (a *authAdapter) AuthenticateByName(ctx context.Context, u, p string) (*auth.JFAuthResult, error) {
	r, err := a.c.AuthenticateByName(ctx, u, p)
	if err != nil {
		return nil, translateErr(err)
	}
	return &auth.JFAuthResult{AccessToken: r.AccessToken, UserID: r.UserID, Username: r.Username}, nil
}

func (a *authAdapter) QuickConnectInitiate(ctx context.Context) (*auth.JFQuickConnectInit, error) {
	r, err := a.c.QuickConnectInitiate(ctx)
	if err != nil {
		return nil, translateErr(err)
	}
	return &auth.JFQuickConnectInit{Secret: r.Secret, Code: r.Code}, nil
}

func (a *authAdapter) QuickConnectAuthenticate(ctx context.Context, secret string) (*auth.JFAuthResult, error) {
	r, err := a.c.QuickConnectAuthenticate(ctx, secret)
	if err != nil {
		return nil, translateErr(err)
	}
	return &auth.JFAuthResult{AccessToken: r.AccessToken, UserID: r.UserID, Username: r.Username}, nil
}

func translateErr(err error) error {
	switch {
	case errors.Is(err, ErrInvalidCredentials):
		return auth.ErrInvalidCredentials
	case errors.Is(err, ErrUpstreamUnavailable):
		return auth.ErrJellyfinUnavailable
	case errors.Is(err, ErrQuickConnectPending):
		return auth.ErrQuickConnectPending
	default:
		return err
	}
}
