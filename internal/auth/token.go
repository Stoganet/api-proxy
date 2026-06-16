package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

var ErrUserNotFound = errors.New("auth: user not found")

func (s *Service) GetJellyfinToken(ctx context.Context, userID string) (string, error) {
	var tok string
	err := s.db.QueryRowContext(ctx,
		`SELECT jellyfin_access_token FROM users WHERE id = ?`, userID,
	).Scan(&tok)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrUserNotFound
	}
	if err != nil {
		return "", fmt.Errorf("GetJellyfinToken: %w", err)
	}
	return tok, nil
}
