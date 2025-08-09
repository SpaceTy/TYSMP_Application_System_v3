-- Core tables: users, applications (1:1 optional), roles (optional later)

CREATE TABLE users (
  id                uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
  discord_user_id   bigint UNIQUE NOT NULL,     -- Discord snowflake
  discord_username  text   NOT NULL,
  minecraft_name    citext UNIQUE,              -- case-insensitive unique
  age               smallint,
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now()
);

-- Application is an object a user can have (0..1)
CREATE TABLE applications (
  id                uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id           uuid NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
  answers           jsonb NOT NULL DEFAULT '{}'::jsonb,
  status            text  NOT NULL CHECK (status IN ('applicant','interview_pending','member','banned')),
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_users_discord_user_id ON users(discord_user_id);
CREATE INDEX idx_users_created_at ON users(created_at);
CREATE INDEX idx_applications_status ON applications(status);
CREATE INDEX idx_applications_created_at ON applications(created_at);

-- Audit log is generic across tables
CREATE TABLE audit_log (
  id            bigserial PRIMARY KEY,
  table_name    text NOT NULL,
  row_id        uuid,
  action        text NOT NULL,  -- 'insert' | 'update' | 'delete'
  before_data   jsonb,
  after_data    jsonb,
  actor         text,
  created_at    timestamptz NOT NULL DEFAULT now()
);

