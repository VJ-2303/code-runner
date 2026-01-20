DELETE FROM snippets;

ALTER TABLE snippets
ADD COLUMN user_id bigint NOT NULL REFERENCES users ON DELETE CASCADE;

CREATE INDEX IF NOT EXISTS snippets_user_id_idx ON snippets(user_id);
