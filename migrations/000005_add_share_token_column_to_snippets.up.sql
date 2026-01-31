ALTER TABLE snippets ADD COLUMN share_token text;
ALTER TABLE snippets ADD CONSTRAINT snippets_share_token_key UNIQUE(share_token);
