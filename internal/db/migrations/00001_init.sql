-- +goose Up
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE users (
  id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  email               TEXT NOT NULL UNIQUE,
  password_hash       TEXT NOT NULL,
  display_name        TEXT NOT NULL,
  timezone            TEXT NOT NULL DEFAULT 'UTC',
  archive_after_days  INT NOT NULL DEFAULT 30,
  calendar_token      TEXT NOT NULL UNIQUE,
  created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE refresh_tokens (
  id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash  TEXT NOT NULL UNIQUE,
  expires_at  TIMESTAMPTZ NOT NULL,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens (user_id);

CREATE TABLE statuses (
  id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name        TEXT NOT NULL,
  color       TEXT NOT NULL DEFAULT '#6b7280',
  position    INT NOT NULL DEFAULT 0,
  is_system   BOOLEAN NOT NULL DEFAULT false,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (user_id, name)
);

CREATE INDEX idx_statuses_user_id ON statuses (user_id);

CREATE TABLE applications (
  id                UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id           UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  company           TEXT NOT NULL,
  position          TEXT,
  link              TEXT,
  description       TEXT,
  current_status_id UUID NOT NULL REFERENCES statuses(id),
  applied_at        TIMESTAMPTZ NOT NULL,
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_applications_user_id ON applications (user_id);
CREATE INDEX idx_applications_current_status_id ON applications (current_status_id);

CREATE TABLE application_status_history (
  id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  application_id UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
  status_id      UUID REFERENCES statuses(id) ON DELETE SET NULL,
  status_name    TEXT NOT NULL,
  changed_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_application_status_history_application_id
  ON application_status_history (application_id);

CREATE TABLE calendar_events (
  id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  title       VARCHAR(40) NOT NULL,
  description VARCHAR(512),
  starts_at   TIMESTAMPTZ NOT NULL,
  ends_at     TIMESTAMPTZ NOT NULL,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_calendar_events_user_range ON calendar_events (user_id, starts_at, ends_at);

CREATE TABLE availability_windows (
  id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  is_recurring  BOOLEAN NOT NULL DEFAULT true,
  day_of_week   SMALLINT,
  specific_date DATE,
  start_time    TIME NOT NULL,
  end_time      TIME NOT NULL,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (
    (is_recurring = true AND day_of_week IS NOT NULL AND specific_date IS NULL) OR
    (is_recurring = false AND day_of_week IS NULL AND specific_date IS NOT NULL)
  )
);

CREATE INDEX idx_availability_windows_user_id ON availability_windows (user_id);

CREATE TABLE invitations (
  id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  title           VARCHAR(40) NOT NULL,
  description     VARCHAR(512),
  proposed_starts TIMESTAMPTZ NOT NULL,
  proposed_ends   TIMESTAMPTZ NOT NULL,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_invitations_user_range ON invitations (user_id, proposed_starts, proposed_ends);

-- +goose Down
DROP TABLE invitations;
DROP TABLE availability_windows;
DROP TABLE calendar_events;
DROP TABLE application_status_history;
DROP TABLE applications;
DROP TABLE statuses;
DROP TABLE refresh_tokens;
DROP TABLE users;
DROP EXTENSION IF EXISTS "uuid-ossp";
