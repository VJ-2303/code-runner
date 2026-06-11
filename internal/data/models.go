package data

import (
	"database/sql"
	"errors"
)

var ErrRecordNotFound = errors.New("record not found")

type Models struct {
	Users   UserModel
	Tokens  TokenModel
	ApiKeys ApiKeyModel
}

func NewModels(db *sql.DB) Models {
	return Models{
		Users:   UserModel{DB: db},
		Tokens:  TokenModel{DB: db},
		ApiKeys: ApiKeyModel{DB: db},
	}
}
