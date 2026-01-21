package data

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/VJ-2303/code-runner/internal/validator"
)

type Snippet struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"-"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Language  string    `json:"language"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	Version   int32     `json:"version"`
}

func ValidateSnippet(v *validator.Validator, snippet *Snippet) {
	v.Check(snippet.Title != "", "title", "must be provided")
	v.Check(len(snippet.Title) <= 100, "title", "must not be more than 100 bytes")

	v.Check(snippet.Content != "", "content", "must be provided")
	v.Check(validator.PermittedValue(snippet.Language, "ruby", "python", "javascript"), "language", "must be either go, python, or javascript")
}

type SnippetMini struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	Language  string    `json:"language"`
	CreatedAt time.Time `json:"created_at"`
}

type SnippetModel struct {
	DB *sql.DB
}

func (m SnippetModel) Insert(snippet *Snippet) error {
	query := `
			INSERT INTO snippets (user_id,title, content, language, expires_at)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING id, created_at, version`

	args := []any{
		snippet.UserID,
		snippet.Title,
		snippet.Content,
		snippet.Language,
		snippet.ExpiresAt,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	return m.DB.QueryRowContext(ctx, query, args...).Scan(&snippet.ID, &snippet.Content, &snippet.Version)
}

func (m SnippetModel) Get(id int64) (*Snippet, error) {
	if id < 1 {
		return nil, ErrRecordNotFound
	}

	query := `
		SELECT id,user_id, title, content, language, created_at, expires_at, version
		FROM snippets
		WHERE id = $1`

	var snippet Snippet

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, id).Scan(
		&snippet.ID,
		&snippet.UserID,
		&snippet.Title,
		&snippet.Content,
		&snippet.Language,
		&snippet.CreatedAt,
		&snippet.ExpiresAt,
		&snippet.Version,
	)
	if err != nil {
		// If no rows were found, sql.ErrNoRows is returned.
		// We map this to our own ErrRecordNotFound to decouple the handler from SQL specifics.
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}

	return &snippet, nil
}

func (m SnippetModel) GetAllForUserID(userID int64) ([]*SnippetMini, error) {
	query := `
		SELECT id, title, language, created_at
		FROM snippets
		WHERE user_id = $1
			 `

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rows, err := m.DB.QueryContext(ctx, query, userID)
	if err != nil {
		return []*SnippetMini{}, err
	}

	var snippets []*SnippetMini

	for rows.Next() {

		var mini SnippetMini
		err := rows.Scan(
			&mini.ID,
			&mini.Title,
			&mini.Language,
			&mini.CreatedAt,
		)
		if err != nil {
			return []*SnippetMini{}, err
		}
		snippets = append(snippets, &mini)
	}
	if err := rows.Err(); err != nil {
		return []*SnippetMini{}, err
	}
	return snippets, nil
}

func (m SnippetModel) Update(snippet *Snippet) error {
	query := `
			UPDATE snippets SET title = $1, content = $2, language = $3, version = version + 1
			WHERE id = $4 and version = $5
			RETURNING version
	`
	args := []any{snippet.Title, snippet.Content, snippet.Language, snippet.ID, snippet.Version}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, args...).Scan(&snippet.Version)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrEditConflict
		}
		return err
	}
	return nil
}

func (m SnippetModel) Delete(ID int64) error {
	query := `
		DELETE FROM snippets WHERE id = $1
	`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := m.DB.ExecContext(ctx, query, ID)
	if err != nil {
		return err
	}
	return nil
}
