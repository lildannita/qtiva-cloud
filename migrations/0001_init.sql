BEGIN;

CREATE TABLE IF NOT EXISTS clients (
  id                TEXT PRIMARY KEY,
  name              TEXT NOT NULL,
  network_allowed   BOOLEAN NOT NULL DEFAULT FALSE,
  concurrency_limit INTEGER NOT NULL DEFAULT 1 CHECK (concurrency_limit >= 1),
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS users (
  id            TEXT PRIMARY KEY,
  client_id     TEXT NOT NULL REFERENCES clients(id),
  email         TEXT NOT NULL,
  password_hash TEXT NOT NULL,
  role          TEXT NOT NULL CHECK (role IN ('admin','user')),
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at    TIMESTAMPTZ NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS users_email_unique_active
  ON users (lower(email))
  WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS users_client_id_idx ON users (client_id);

CREATE TABLE IF NOT EXISTS invites (
  code        TEXT PRIMARY KEY,
  client_id   TEXT NOT NULL REFERENCES clients(id),
  max_uses    INTEGER NOT NULL CHECK (max_uses >= 1),
  used_count  INTEGER NOT NULL DEFAULT 0 CHECK (used_count >= 0),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  revoked_at  TIMESTAMPTZ NULL,
  expires_at  TIMESTAMPTZ NULL,
  CONSTRAINT invites_used_not_exceed_max CHECK (used_count <= max_uses)
);

CREATE INDEX IF NOT EXISTS invites_client_id_idx ON invites (client_id);

CREATE TABLE IF NOT EXISTS artifacts (
  id          TEXT PRIMARY KEY,
  client_id   TEXT NOT NULL REFERENCES clients(id),
  user_id     TEXT NOT NULL REFERENCES users(id),
  sha256      TEXT NOT NULL,
  size_bytes  BIGINT NOT NULL CHECK (size_bytes >= 0),
  stored_path TEXT NOT NULL,
  manifest    JSONB NOT NULL,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  consumed_at TIMESTAMPTZ NULL,
  deleted_at  TIMESTAMPTZ NULL
);

CREATE INDEX IF NOT EXISTS artifacts_client_id_idx ON artifacts (client_id);
CREATE INDEX IF NOT EXISTS artifacts_user_id_idx ON artifacts (user_id);

CREATE TABLE IF NOT EXISTS runs (
  id           TEXT PRIMARY KEY,
  client_id    TEXT NOT NULL REFERENCES clients(id),
  user_id      TEXT NOT NULL REFERENCES users(id),
  artifact_id  TEXT NOT NULL REFERENCES artifacts(id),

  status       TEXT NOT NULL CHECK (status IN ('pending','running','passed','failed','timeout','error')),
  ack_mode     TEXT NOT NULL CHECK (ack_mode IN ('auto')),

  os           TEXT NOT NULL,
  display      TEXT NOT NULL,
  qt_version   TEXT NOT NULL,

  exit_code    INTEGER NULL,

  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  started_at   TIMESTAMPTZ NULL,
  finished_at  TIMESTAMPTZ NULL,

  received_at  TIMESTAMPTZ NULL,
  delete_after TIMESTAMPTZ NULL,

  log_path     TEXT NULL,
  deleted_at   TIMESTAMPTZ NULL
);

CREATE INDEX IF NOT EXISTS runs_client_id_idx ON runs (client_id);
CREATE INDEX IF NOT EXISTS runs_user_id_idx ON runs (user_id);
CREATE INDEX IF NOT EXISTS runs_status_idx ON runs (status);

CREATE TABLE IF NOT EXISTS jobs (
  id           TEXT PRIMARY KEY,
  run_id       TEXT NOT NULL UNIQUE REFERENCES runs(id),

  status       TEXT NOT NULL CHECK (status IN ('queued','running','done','failed')),
  attempts     INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
  max_attempts INTEGER NOT NULL DEFAULT 3 CHECK (max_attempts >= 1),

  lease_until  TIMESTAMPTZ NULL,
  leased_by    TEXT NULL,

  available_at TIMESTAMPTZ NOT NULL DEFAULT now(),

  last_error   TEXT NULL,

  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS jobs_status_available_idx ON jobs (status, available_at);
CREATE INDEX IF NOT EXISTS jobs_lease_until_idx ON jobs (lease_until);

COMMIT;
