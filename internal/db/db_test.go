package db

import (
	"context"
	"testing"
)

func TestOpenAppliesMigrations(t *testing.T) {
	d, err := Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.Close()
	var count int
	if err := d.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='users'`,
	).Scan(&count); err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 1 {
		t.Fatalf("users table missing")
	}
}
