package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
)

type User struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
}

type TokenPair struct {
	AccessToken  string
	RefreshToken string
	User         User
}

func (s *Service) Login(ctx context.Context, username, password string, deviceLabel *string) (*TokenPair, error) {
	locked, err := s.IsLocked(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("lockout check: %w", err)
	}
	if locked {
		return nil, ErrAccountLocked
	}

	jfRes, err := s.jf.AuthenticateByName(ctx, username, password)
	if err != nil {
		_ = s.RecordAttempt(ctx, username, false)
		if errors.Is(err, ErrInvalidCredentials) {
			return nil, ErrInvalidCredentials
		}
		if errors.Is(err, ErrJellyfinUnavailable) {
			return nil, ErrJellyfinUnavailable
		}
		return nil, fmt.Errorf("jellyfin auth: %w", err)
	}
	_ = s.RecordAttempt(ctx, username, true)

	u, err := s.upsertUser(ctx, jfRes)
	if err != nil {
		return nil, fmt.Errorf("upsert: %w", err)
	}

	access, err := s.issueJWT(u.ID, u.Email, jfRes.UserID)
	if err != nil {
		return nil, err
	}
	refresh, err := s.issueRefreshToken(ctx, u.ID, deviceLabel)
	if err != nil {
		return nil, err
	}
	return &TokenPair{AccessToken: access, RefreshToken: refresh, User: *u}, nil
}

func (s *Service) upsertUser(ctx context.Context, jf *JFAuthResult) (*User, error) {
	now := s.clock().Unix()
	email := jf.Username + "@jellyfin.local"

	var u User
	err := s.db.QueryRowContext(ctx,
		`SELECT id, email, display_name FROM users WHERE jellyfin_user_id = ?`,
		jf.UserID,
	).Scan(&u.ID, &u.Email, &u.DisplayName)
	if err == nil {
		_, err = s.db.ExecContext(ctx,
			`UPDATE users SET jellyfin_access_token = ?, updated_at = ? WHERE id = ?`,
			jf.AccessToken, now, u.ID,
		)
		return &u, err
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	id := newUUID()
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO users(id, email, display_name, jellyfin_user_id, jellyfin_access_token, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, email, jf.Username, jf.UserID, jf.AccessToken, now, now,
	)
	if err != nil {
		return nil, err
	}
	return &User{ID: id, Email: email, DisplayName: jf.Username}, nil
}

func (s *Service) issueRefreshToken(ctx context.Context, userID string, deviceLabel *string) (string, error) {
	plain := randomToken(32)
	hash := sha256Hex(plain)
	id := newUUID()
	now := s.clock().Unix()
	exp := s.clock().Add(s.refreshTTL).Unix()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO refresh_tokens(id, user_id, token_hash, device_label, created_at, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		id, userID, hash, deviceLabel, now, exp,
	)
	if err != nil {
		return "", err
	}
	return plain, nil
}

func randomToken(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

