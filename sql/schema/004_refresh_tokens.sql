-- +goose Up
CREATE TABLE refresh_tokens (
  token string PRIMARY KEY,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  expires_at TIMESTAMP NOT NULL,
  revoked_at TIMESTAMP
);

-- +goose Down
DROP TABLE refresh_tokens;