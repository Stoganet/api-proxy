package auth

import "context"

const (
	lockoutWindowSec = 15 * 60
	lockoutThreshold = 5
)

func (s *Service) RecordAttempt(ctx context.Context, username string, success bool) error {
	now := s.clock().Unix()
	succInt := 0
	if success {
		succInt = 1
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO login_attempts(username, attempted_at, success) VALUES (?, ?, ?)`,
		username, now, succInt,
	)
	return err
}

func (s *Service) IsLocked(ctx context.Context, username string) (bool, error) {
	cutoff := s.clock().Unix() - int64(lockoutWindowSec)
	var fails int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM login_attempts
		 WHERE username = ? AND success = 0 AND attempted_at >= ?`,
		username, cutoff,
	).Scan(&fails)
	if err != nil {
		return false, err
	}
	return fails >= lockoutThreshold, nil
}
