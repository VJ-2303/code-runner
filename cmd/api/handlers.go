package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
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

func (app *application) runCodeHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Code     string `json:"code"`
		Language string `json:"language"`
		Stdin    string `json:"stdin"`
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

	result, err := app.runner.Run(ctx, input.Code, input.Language, input.Stdin)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	err = app.writeJSON(w, http.StatusOK, envelope{"result": result}, nil)
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
