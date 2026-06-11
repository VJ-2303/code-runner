package data

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"
)

type ApiKey struct {
	ID         int64      `json:"id"`
	UserID     int64      `json:"-"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
}

type ApiKeyModel struct {
	DB *sql.DB
}

func (m ApiKeyModel) GenerateApiKey(userID int64, name string) (*ApiKey, string, error) {
	randomBytes := make([]byte, 32)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return nil, "", err
	}

	plaintext := hex.EncodeToString(randomBytes)
	prefix := plaintext[:8]

	hash := sha256.Sum256([]byte(plaintext))

	apiKey := &ApiKey{
		UserID: userID,
		Name:   name,
		Prefix: prefix,
	}

	query := `
		INSERT INTO api_keys (user_id, name, prefix, hash)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at
	`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err = m.DB.QueryRowContext(ctx, query, userID, name, prefix, hash).Scan(
		&apiKey.ID, &apiKey.CreatedAt,
	)

	if err != nil {
		return nil, "", err
	}

	return apiKey, plaintext, nil
}

func (m ApiKeyModel) GetUserForApiKey(plaintext string) (*User, error) {
	hash := sha256.Sum256([]byte(plaintext))

	query := `
		SELECT users.id, users.created_at, users.name, users.email,
		       users.password_hash, users.activated, users.version
		FROM users
		INNER JOIN api_keys ON users.id = api_keys.user_id
		WHERE api_keys.hash = $1`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var user User

	err := m.DB.QueryRowContext(ctx, query, hash).Scan(
		&user.ID,
		&user.CreatedAt,
		&user.Name,
		&user.Email,
		&user.Password.hash,
		&user.Activated,
		&user.Version,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}

	go func() {
		updateQuery := `UPDATE api_keys SET last_used_at = NOW() WHERE hash = $1`
		updateCtx, updateCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer updateCancel()
		m.DB.ExecContext(updateCtx, updateQuery, hash[:])
	}()

	return &user, nil
}

func (m ApiKeyModel) GetAllForUser(userID int64) ([]*ApiKey, error) {
	query := `
		SELECT id, name, prefix, created_at, last_used_at
		FROM api_keys
		WHERE user_id = $1
		ORDER BY created_at DESC`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rows, err := m.DB.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []*ApiKey

	for rows.Next() {
		var key ApiKey
		err := rows.Scan(&key.ID, &key.Name, &key.Prefix, &key.CreatedAt, &key.LastUsedAt)
		if err != nil {
			return nil, err
		}
		keys = append(keys, &key)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return keys, nil
}

func (m ApiKeyModel) Delete(id, userID int64) error {
	query := `DELETE FROM api_keys WHERE id = $1 AND user_id = $2`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result, err := m.DB.ExecContext(ctx, query, id, userID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrRecordNotFound
	}

	return nil
}

func (m ApiKeyModel) GetDailyLimit(userID int64) (int, error) {
	query := `SELECT daily_limit FROM users WHERE id = $1`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var limit int
	err := m.DB.QueryRowContext(ctx, query, userID).Scan(&limit)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrRecordNotFound
		}
		return 0, err
	}

	return limit, nil
}
