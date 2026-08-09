CREATE TABLE users (
  id BIGSERIAL PRIMARY KEY,
  email TEXT NOT NULL,
  display_name TEXT NOT NULL DEFAULT '',
  password_hash TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  last_login_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX users_normalized_email_key ON users (LOWER(BTRIM(email)));

CREATE TABLE sessions (
  id BIGSERIAL PRIMARY KEY,
  token_hash CHAR(64) NOT NULL UNIQUE,
  csrf_hash CHAR(64) NOT NULL,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  idle_expires_at TIMESTAMPTZ NOT NULL,
  absolute_expires_at TIMESTAMPTZ NOT NULL,
  revoked_at TIMESTAMPTZ,
  device_metadata JSONB NOT NULL DEFAULT '{}'::JSONB
);

CREATE INDEX sessions_user_active_idx ON sessions (user_id, absolute_expires_at) WHERE revoked_at IS NULL;

CREATE TABLE security_events (
  id BIGSERIAL PRIMARY KEY,
  request_id TEXT NOT NULL,
  user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
  event_type TEXT NOT NULL,
  outcome TEXT NOT NULL,
  source_address TEXT,
  occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX security_events_user_time_idx ON security_events (user_id, occurred_at DESC);
CREATE INDEX security_events_type_time_idx ON security_events (event_type, occurred_at DESC);

ALTER TABLE squad_plans ADD COLUMN user_id BIGINT REFERENCES users(id) ON DELETE CASCADE;
DROP INDEX IF EXISTS squad_plans_season_idx;
CREATE UNIQUE INDEX squad_plans_user_season_key ON squad_plans (user_id, season_id) WHERE user_id IS NOT NULL;

COMMENT ON COLUMN squad_plans.user_id IS 'Staged ownership column. Legacy unowned rows remain inaccessible until explicitly assigned to a bootstrap user.';
COMMENT ON TABLE security_events IS 'Redacted authentication audit events. Passwords, session tokens, and CSRF tokens are forbidden.';
