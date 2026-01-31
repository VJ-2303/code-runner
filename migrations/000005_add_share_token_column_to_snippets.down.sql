ALTER TABLE snippets DROP CONSTRAINT IF EXISTS snippets_share_token_key;
ALTER TABLE snippets DROP COLUMN IF EXISTS share_token;
