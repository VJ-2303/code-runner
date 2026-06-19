CREATE TABLE IF NOT EXISTS jobs (
    id uuid PRIMARY KEY,
    user_id bigint REFERENCES users(id) ON DELETE CASCADE,
    language text NOT NULL,
    code text NOT NULL,
    status text NOT NULL DEFAULT 'PENDING',
    output text,
    error text,
    created_at timestamp(0) with time zone NOT NULL DEFAULT NOW(),
    updated_at timestamp(0) with time zone NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_jobs_user_id ON jobs(user_id);
