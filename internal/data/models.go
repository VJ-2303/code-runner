package data

import "database/sql"

type Models struct {
	Snippets SnippetModel
}

func NewModels(db *sql.DB) Models {
	return Models{
		Snippets: SnippetModel{DB: db},
	}
}
