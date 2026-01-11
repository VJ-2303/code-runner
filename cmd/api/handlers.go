package main

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/VJ-2303/code-runner/internal/data"
)

func (app *application) createSnippetHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Title    string `json:"title"`
		Content  string `json:"content"`
		Language string `json:"language"`
	}

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.logger.Error(err.Error())
		http.Error(w, "Bad Request: "+err.Error(), http.StatusBadRequest)
		return
	}

	snippet := &data.Snippet{
		Title:     input.Title,
		Content:   input.Content,
		Language:  input.Language,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	}

	err = app.models.Snippets.Insert(snippet)
	if err != nil {
		app.logger.Error(err.Error())
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	headers := make(http.Header)
	headers.Set("Location", fmt.Sprintf("/v1/snippets/%d", snippet.ID))

	err = app.writeJSON(w, http.StatusCreated, envelope{"snippet": snippet}, headers)
	if err != nil {
		app.logger.Error(err.Error())
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (app *application) showSnippetHandler(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id < 1 {
		http.NotFound(w, r)
		return
	}

	snippet, err := app.models.Snippets.Get(id)
	if err != nil {
		if errors.Is(err, data.ErrRecordNotFound) {
			http.NotFound(w, r)
		} else {
			app.logger.Error(err.Error())
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"snippet": snippet}, nil)
	if err != nil {
		app.logger.Error(err.Error())
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
