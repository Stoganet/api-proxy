package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type QuickConnectStartOut struct {
	Code      string
	PollToken string
}

func (s *Service) QuickConnectStart(ctx context.Context) (*QuickConnectStartOut, error) {
	init, err := s.jf.QuickConnectInitiate(ctx)
	if err != nil {
		return nil, ErrJellyfinUnavailable
	}
	poll := randomToken(24)
	now := s.clock().Unix()
	exp := s.clock().Add(10 * time.Minute).Unix()
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO quick_connect_sessions(poll_token, jf_secret, created_at, expires_at)
		 VALUES (?, ?, ?, ?)`,
		poll, init.Secret, now, exp,
	)
	if err != nil {
		return nil, fmt.Errorf("qc insert: %w", err)
	}
	return &QuickConnectStartOut{Code: init.Code, PollToken: poll}, nil
}

func (s *Service) QuickConnectPoll(ctx context.Context, pollToken string) (*TokenPair, error) {
	var (
		secret    string
		expiresAt int64
	)
	err := s.db.QueryRowContext(ctx,
		`SELECT jf_secret, expires_at FROM quick_connect_sessions WHERE poll_token = ?`,
		pollToken,
	).Scan(&secret, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrTokenInvalid
	}
	if err != nil {
		return nil, err
	}
	if expiresAt < s.clock().Unix() {
		return nil, ErrQuickConnectExpired
	}

	jfRes, err := s.jf.QuickConnectAuthenticate(ctx, secret)
	if err != nil {
		if errors.Is(err, ErrQuickConnectPending) {
			return nil, ErrQuickConnectPending
		}
		return nil, ErrJellyfinUnavailable
	}

	u, err := s.upsertUser(ctx, jfRes)
	if err != nil {
		return nil, err
	}
	access, err := s.issueJWT(u.ID, u.Email, jfRes.UserID)
	if err != nil {
		return nil, err
	}
	refresh, err := s.issueRefreshToken(ctx, u.ID, nil)
	if err != nil {
		return nil, err
	}
	_, _ = s.db.ExecContext(ctx, `DELETE FROM quick_connect_sessions WHERE poll_token = ?`, pollToken)
	return &TokenPair{AccessToken: access, RefreshToken: refresh, User: *u}, nil
}
