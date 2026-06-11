CREATE TABLE IF NOT EXISTS api_keys (
    id bigserial PRIMARY KEY,
    user_id bigint NOT NULL REFERENCES users ON DELETE CASCADE,
    name text NOT NULL,
    prefix text NOT NULL,
    hash bytea NOT NULL UNIQUE,
    created_at timestamp(0) with time zone NOT NULL DEFAULT NOW(),
    last_used_at timestamp(0) with time zone
);

CREATE INDEX idx_api_keys_user_id ON api_keys(user_id);

ALTER TABLE users ADD COLUMN daily_limit integer NOT NULL DEFAULT 100;
