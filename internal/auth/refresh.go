package auth

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
)

func (s *Service) Refresh(ctx context.Context, plaintext string) (*TokenPair, error) {
	hash := sha256Hex(plaintext)
	now := s.clock().Unix()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var (
		id        string
		userID    string
		expiresAt int64
		usedAt    sql.NullInt64
		deviceLbl sql.NullString
	)
	err = tx.QueryRowContext(ctx,
		`SELECT id, user_id, expires_at, used_at, device_label
		 FROM refresh_tokens WHERE token_hash = ?`,
		hash,
	).Scan(&id, &userID, &expiresAt, &usedAt, &deviceLbl)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrTokenInvalid
	}
	if err != nil {
		return nil, err
	}
	if expiresAt < now {
		return nil, ErrTokenExpired
	}
	if usedAt.Valid {
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM refresh_tokens WHERE user_id = ?`, userID,
		); err != nil {
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return nil, ErrTokenReused
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE refresh_tokens SET used_at = ? WHERE id = ?`, now, id,
	); err != nil {
		return nil, err
	}

	var u User
	var jfUserID string
	err = tx.QueryRowContext(ctx,
		`SELECT id, email, display_name, jellyfin_user_id FROM users WHERE id = ?`, userID,
	).Scan(&u.ID, &u.Email, &u.DisplayName, &jfUserID)
	if err != nil {
		return nil, err
	}

	access, err := s.issueJWT(u.ID, u.Email, jfUserID)
	if err != nil {
		return nil, err
	}
	var devPtr *string
	if deviceLbl.Valid {
		v := deviceLbl.String
		devPtr = &v
	}
	plain := randomToken(32)
	newHash := sha256Hex(plain)
	newID := newUUID()
	exp := s.clock().Add(s.refreshTTL).Unix()
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO refresh_tokens(id, user_id, token_hash, device_label, created_at, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		newID, userID, newHash, devPtr, now, exp,
	); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &TokenPair{AccessToken: access, RefreshToken: plain, User: u}, nil
}

func newUUID() string { return uuid.NewString() }
