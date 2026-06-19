package data

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
)

type Job struct {
	ID        uuid.UUID `json:"id"`
	UserID    int64     `json:"user_id"`
	Language  string    `json:"language"`
	Code      string    `json:"code"`
	Stdin     string    `json:"stdin"`
	Status    string    `json:"status"`
	Output    *string   `json:"output,omitempty"`
	Error     *string   `json:"error,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type JobsModel struct {
	DB *sql.DB
}

func (m JobsModel) Insert(job *Job) error {
	query := `
		INSERT INTO jobs(id, user_id, language, code, stdin, status)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING created_at, updated_at
	`
	args := []any{job.ID, job.UserID, job.Language, job.Code, job.Stdin, job.Status}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	return m.DB.QueryRowContext(ctx, query, args...).Scan(&job.CreatedAt, &job.UpdatedAt)
}

func (m JobsModel) Get(id uuid.UUID) (*Job, error) {
	query := `
		SELECT id, user_id, language, code, stdin, status, output, error, created_at, updated_at
		FROM jobs
		WHERE id = $1`

	var job Job

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, id).Scan(
		&job.ID,
		&job.UserID,
		&job.Language,
		&job.Code,
		&job.Stdin,
		&job.Status,
		&job.Output,
		&job.Error,
		&job.CreatedAt,
		&job.UpdatedAt,
	)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	return &job, nil
}

func (m JobsModel) Update(job *Job) error {
	query := `
		UPDATE jobs
		SET status = $1, output = $2, error = $3, updated_at = NOW()
		WHERE id = $4
		RETURNING updated_at`

	args := []any{
		job.Status,
		job.Output,
		job.Error,
		job.ID,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	return m.DB.QueryRowContext(ctx, query, args...).Scan(&job.UpdatedAt)
}
