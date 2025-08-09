-- Temporary login tokens associated with users

CREATE TABLE IF NOT EXISTS login_tokens (
  id          uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token       uuid NOT NULL UNIQUE,
  added_at    timestamptz NOT NULL DEFAULT now(),
  expires_at  timestamptz NOT NULL,
  revoked     boolean NOT NULL DEFAULT false
);

CREATE INDEX IF NOT EXISTS idx_login_tokens_user_id ON login_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_login_tokens_expires_at ON login_tokens(expires_at);


