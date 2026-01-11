CREATE TABLE snippets (
    id bigserial PRIMARY KEY,
    title text NOT NULL,
    content text NOT NULL,
    language text NOT NULL,
    created_at timestamp(0) with time zone NOT NULL DEFAULT NOW(),
    expires_at timestamp(0) with time zone NOT NULL,
    version integer NOT NULL DEFAULT 1
);

-- Add an index on created_at because we'll likely sort by date later
CREATE INDEX snippets_created_at_idx ON snippets (created_at);