package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/VJ-2303/code-runner/internal/broker"
	"github.com/VJ-2303/code-runner/internal/data"
	"github.com/VJ-2303/code-runner/internal/validator"
	"github.com/google/uuid"
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

	jobID := uuid.New()
	user := contextGetUser(r)
	job := &data.Job{
		ID:       jobID,
		UserID:   user.ID,
		Language: input.Language,
		Code:     input.Code,
		Stdin:    input.Stdin,
		Status:   "PENDING",
	}

	err = app.models.Jobs.Insert(job)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	executionJob := broker.ExecutionPayload{
		JobID:    job.ID,
		Language: input.Language,
		Code:     input.Code,
		Stdin:    input.Stdin,
	}

	err = app.rabbitmq.Publish(ctx, executionJob)

	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"job_id": jobID, "status": "PENDING", "message": "code submitted successfully"}, nil)
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

func (app *application) createApiKeyHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name string `json:"name"`
	}

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	v := validator.New()

	v.Check(input.Name != "", "name", "must be provided")
	v.Check(len(input.Name) <= 100, "name", "must not be more than 100 characters")

	if !v.Valid() {
		app.failedValidationResponse(w, r, v.FieldErrors)
		return
	}

	user := contextGetUser(r)

	apiKey, plaintext, err := app.models.ApiKeys.GenerateApiKey(user.ID, input.Name)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	err = app.writeJSON(w, http.StatusCreated, envelope{
		"api_key": apiKey,
		"key":     plaintext,
	}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) listApiKeysHandler(w http.ResponseWriter, r *http.Request) {
	user := contextGetUser(r)

	keys, err := app.models.ApiKeys.GetAllForUser(user.ID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"api_keys": keys}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) deleteApiKeyHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ID int64 `json:"id"`
	}

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	v := validator.New()

	v.Check(input.ID > 0, "id", "must be a positive integer")

	if !v.Valid() {
		app.failedValidationResponse(w, r, v.FieldErrors)
		return
	}

	user := contextGetUser(r)

	err = app.models.ApiKeys.Delete(input.ID, user.ID)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"message": "api key deleted successfully"}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) getUsageHandler(w http.ResponseWriter, r *http.Request) {
	user := contextGetUser(r)

	usage, err := app.limiter.GetUsage(r.Context(), user.ID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	limit := user.DailyLimit

	err = app.writeJSON(w, http.StatusOK, envelope{
		"usage": map[string]any{
			"used":      usage,
			"limit":     limit,
			"remaining": int64(limit) - usage,
		},
	}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
