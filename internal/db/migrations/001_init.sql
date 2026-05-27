PRAGMA journal_mode = WAL;
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS users (
  id                    TEXT PRIMARY KEY,
  email                 TEXT NOT NULL UNIQUE,
  display_name          TEXT NOT NULL,
  jellyfin_user_id      TEXT NOT NULL UNIQUE,
  jellyfin_access_token TEXT NOT NULL,
  created_at            INTEGER NOT NULL,
  updated_at            INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS refresh_tokens (
  id           TEXT PRIMARY KEY,
  user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash   TEXT NOT NULL,
  device_label TEXT,
  created_at   INTEGER NOT NULL,
  expires_at   INTEGER NOT NULL,
  used_at      INTEGER
);
CREATE INDEX IF NOT EXISTS idx_refresh_user ON refresh_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_refresh_hash ON refresh_tokens(token_hash);

CREATE TABLE IF NOT EXISTS login_attempts (
  username     TEXT NOT NULL,
  attempted_at INTEGER NOT NULL,
  success      INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_login_user_time ON login_attempts(username, attempted_at);

CREATE TABLE IF NOT EXISTS quick_connect_sessions (
  poll_token   TEXT PRIMARY KEY,
  jf_secret    TEXT NOT NULL,
  created_at   INTEGER NOT NULL,
  expires_at   INTEGER NOT NULL,
  approved     INTEGER NOT NULL DEFAULT 0
);
