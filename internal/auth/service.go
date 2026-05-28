package auth

import (
	"context"
	"database/sql"
	"time"
)

type JellyfinAuthenticator interface {
	AuthenticateByName(ctx context.Context, username, password string) (*JFAuthResult, error)
	QuickConnectInitiate(ctx context.Context) (*JFQuickConnectInit, error)
	QuickConnectAuthenticate(ctx context.Context, secret string) (*JFAuthResult, error)
}

type JFAuthResult struct {
	AccessToken string
	UserID      string
	Username    string
}

type JFQuickConnectInit struct {
	Secret string
	Code   string
}

type Clock func() time.Time

type Service struct {
	db         *sql.DB
	jf         JellyfinAuthenticator
	signKey    []byte
	clock      Clock
	accessTTL  time.Duration
	refreshTTL time.Duration
}

type Options struct {
	DB         *sql.DB
	Jellyfin   JellyfinAuthenticator
	SignKey     []byte
	Clock       Clock
	AccessTTL   time.Duration
	RefreshTTL  time.Duration
}

func NewService(opts Options) *Service {
	clock := opts.Clock
	if clock == nil {
		clock = time.Now
	}
	at := opts.AccessTTL
	if at == 0 {
		at = 24 * time.Hour
	}
	rt := opts.RefreshTTL
	if rt == 0 {
		rt = 90 * 24 * time.Hour
	}
	return &Service{
		db:         opts.DB,
		jf:         opts.Jellyfin,
		signKey:    opts.SignKey,
		clock:      clock,
		accessTTL:  at,
		refreshTTL: rt,
	}
}
