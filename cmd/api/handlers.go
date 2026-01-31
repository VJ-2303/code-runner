package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/VJ-2303/code-runner/internal/data"
	"github.com/VJ-2303/code-runner/internal/validator"
)

func (app *application) healthcheckHandler(w http.ResponseWriter, r *http.Request) {
	data := envelope{
		"system": "available",
		"system_info": map[string]string{
			"environment": app.config.env,
			"version":     "1.0.0",
		},
	}

	err := app.writeJSON(w, http.StatusOK, data, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) createSnippetHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Title    string `json:"title"`
		Content  string `json:"content"`
		Language string `json:"language"`
	}

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	user := contextGetUser(r)

	snippet := &data.Snippet{
		Title:     input.Title,
		UserID:    user.ID,
		Content:   input.Content,
		Language:  input.Language,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	}

	v := validator.New()
	data.ValidateSnippet(v, snippet)
	if !v.Valid() {
		app.failedValidationResponse(w, r, v.FieldErrors)
		return
	}

	err = app.models.Snippets.Insert(snippet)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	headers := make(http.Header)
	headers.Set("Location", fmt.Sprintf("/v1/snippets/%d", snippet.ID))

	err = app.writeJSON(w, http.StatusCreated, envelope{"snippet": snippet}, headers)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) runCodeHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Code     string `json:"code"`
		Language string `json:"language"`
	}

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	v := validator.New()

	v.Check(input.Code != "", "code", "must be provided")
	v.Check(validator.PermittedValue(input.Language, "ruby", "python", "javascript"), "language", "must be either go, python or javascript")

	if !v.Valid() {
		app.failedValidationResponse(w, r, v.FieldErrors)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := app.runner.Run(ctx, input.Code, input.Language)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	err = app.writeJSON(w, http.StatusOK, envelope{"result": result}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) GetAllSnippetHandler(w http.ResponseWriter, r *http.Request) {
	user := contextGetUser(r)

	v := validator.New()
	qs := r.URL.Query()
	f := data.Filters{
		Language: app.readString(qs, "lang", ""),
		PageSize: app.readInt(qs, "page_size", 5, v),
		Page:     app.readInt(qs, "page", 1, v),
	}
	data.ValidateFilters(v, f)

	if !v.Valid() {
		app.failedValidationResponse(w, r, v.FieldErrors)
		return
	}

	snippets, metaData, err := app.models.Snippets.GetAllForUserID(user.ID, f)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	err = app.writeJSON(w, http.StatusOK, envelope{"snippets": snippets, "meta_data": metaData}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
}

func (app *application) getSnippetHandler(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id < 1 {
		app.notFoundResponse(w, r)
		return
	}
	user := contextGetUser(r)

	snippet, err := app.models.Snippets.Get(id)
	if err != nil {
		if errors.Is(err, data.ErrRecordNotFound) {
			app.notFoundResponse(w, r)
		} else {
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	if snippet.UserID != user.ID {
		app.notFoundResponse(w, r)
		return
	}
	err = app.writeJSON(w, http.StatusOK, envelope{"snippet": snippet}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) updateSnippetHandler(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id < 1 {
		app.notFoundResponse(w, r)
		return
	}
	snippet, err := app.models.Snippets.Get(id)
	if err != nil {
		if errors.Is(err, data.ErrRecordNotFound) {
			app.notFoundResponse(w, r)
		} else {
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	user := contextGetUser(r)

	if user.ID != snippet.UserID {
		app.notFoundResponse(w, r)
		return
	}
	var input struct {
		Title    *string `json:"title"`
		Language *string `json:"language"`
		Content  *string `json:"content"`
	}

	err = app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	if input.Title != nil {
		snippet.Title = *input.Title
	}
	if input.Content != nil {
		snippet.Content = *input.Content
	}
	if input.Language != nil {
		snippet.Language = *input.Language
	}
	v := validator.New()
	data.ValidateSnippet(v, snippet)
	if !v.Valid() {
		app.failedValidationResponse(w, r, v.FieldErrors)
		return
	}

	err = app.models.Snippets.Update(snippet)
	if err != nil {
		if errors.Is(err, data.ErrEditConflict) {
			app.editConflictResponse(w, r)
			return
		}
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"snippet": snippet}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) deleteSnippetHandler(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id < 1 {
		app.notFoundResponse(w, r)
		return
	}
	snippet, err := app.models.Snippets.Get(id)
	if err != nil {
		if errors.Is(err, data.ErrRecordNotFound) {
			app.notFoundResponse(w, r)
		} else {
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	user := contextGetUser(r)

	if user.ID != snippet.UserID {
		app.notFoundResponse(w, r)
		return
	}

	err = app.models.Snippets.Delete(snippet.ID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	err = app.writeJSON(w, http.StatusGone, envelope{"message": "Snippet deleted successfully"}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) registerUserHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	user := &data.User{
		Name:      input.Name,
		Email:     input.Email,
		Activated: false,
	}

	err = user.Password.Set(input.Password)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	v := validator.New()
	data.ValidateUser(v, user)

	if !v.Valid() {
		app.failedValidationResponse(w, r, v.FieldErrors)
		return
	}

	err = app.models.Users.Insert(user)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrDuplicateEmail):
			v.AddError("email", "a user with this email address already exists")
			app.failedValidationResponse(w, r, v.FieldErrors)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	token, err := app.models.Tokens.New(user.ID, 72*time.Hour, data.ScopeActivation)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	app.background(func() {
		data := map[string]string{
			"VerifyURL": fmt.Sprintf("http://localhost:4000/v1/users/activate?token=%s", token.Plaintext),
		}
		err := app.mailer.Send(user.Email, "user_welcome.tmpl", data)
		if err != nil {
			app.logger.Error(err.Error())
		}
	})

	err = app.writeJSON(w, http.StatusCreated, envelope{"user": user}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) getProfileHandler(w http.ResponseWriter, r *http.Request) {
	user := contextGetUser(r)

	userStats, err := app.models.Users.GetUserStats(user.ID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	err = app.writeJSON(w, http.StatusOK, envelope{
		"user":  user,
		"stats": userStats,
	}, nil)
}

func (app *application) createShareTokenHandler(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id < 0 {
		app.notFoundResponse(w, r)
		return
	}

	token, err := app.generateRandomToken()
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	user := contextGetUser(r)

	err = app.models.Snippets.SetShareToken(id, user.ID, token)
	if err != nil {
		if errors.Is(err, data.ErrRecordNotFound) {
			app.notFoundResponse(w, r)
			return
		}
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.writeJSON(w, http.StatusCreated, envelope{"share_link": fmt.Sprintf("http://localhost:4000/v1/snippets/share/%s", token)}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
}

func (app *application) getSharedSnippetHandler(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if token == "" {
		app.notFoundResponse(w, r)
		return
	}

	snippet, err := app.models.Snippets.GetByShareToken(token)
	if err != nil {
		if errors.Is(err, data.ErrRecordNotFound) {
			app.notFoundResponse(w, r)
		} else {
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"snippet": snippet}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
}
