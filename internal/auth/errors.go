package auth

import "errors"

var (
	ErrInvalidCredentials  = errors.New("auth: invalid credentials")
	ErrAccountLocked       = errors.New("auth: account locked")
	ErrTokenExpired        = errors.New("auth: token expired")
	ErrTokenInvalid        = errors.New("auth: token invalid")
	ErrTokenReused         = errors.New("auth: refresh token reused")
	ErrJellyfinUnavailable = errors.New("auth: jellyfin unavailable")
	ErrQuickConnectPending = errors.New("auth: quick connect pending")
	ErrQuickConnectExpired = errors.New("auth: quick connect expired")
)
