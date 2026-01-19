package data

import (
	"database/sql"
	"errors"
)

var ErrRecordNotFound = errors.New("record not found")

type Models struct {
	Snippets SnippetModel
	Users    UserModel
}

func NewModels(db *sql.DB) Models {
	return Models{
		Snippets: SnippetModel{DB: db},
		Users:    UserModel{DB: db},
	}
}
