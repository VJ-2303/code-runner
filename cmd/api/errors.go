package main

import (
	"net/http"
)

func (app *application) logError(r *http.Request, err error) {
	app.logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
}

func (app *application) errorResponse(w http.ResponseWriter, r *http.Request, status int, message any) {
	env := envelope{"error": message}

	err := app.writeJSON(w, status, env, nil)
	if err != nil {
		app.logError(r, err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (app *application) serverErrorResponse(w http.ResponseWriter, r *http.Request, err error) {
	app.logError(r, err)
	message := "the server encountered a problem and could not process your request"
	app.errorResponse(w, r, http.StatusInternalServerError, message)
}

func (app *application) notFoundResponse(w http.ResponseWriter, r *http.Request) {
	message := "the requested resource could not be found"
	app.errorResponse(w, r, http.StatusNotFound, message)
}

func (app *application) badRequestResponse(w http.ResponseWriter, r *http.Request, err error) {
	app.errorResponse(w, r, http.StatusBadRequest, err.Error())
}

func (app *application) failedValidationResponse(w http.ResponseWriter, r *http.Request, errros map[string]string) {
	app.errorResponse(w, r, http.StatusUnprocessableEntity, errros)
}

func (app *application) invalidCredentialsResponse(w http.ResponseWriter, r *http.Request) {
	message := "invalid authentication credentials"
	app.errorResponse(w, r, http.StatusUnauthorized, message)
}

func (app *application) invalidAuthenticationTokenResponse(w http.ResponseWriter, r *http.Request) {
	message := "invalid or missing authentication token"
	app.errorResponse(w, r, http.StatusUnauthorized, message)
}

func (app *application) authenticationRequiredResponse(w http.ResponseWriter, r *http.Request) {
	message := "only authenticated users can access this resource"
	app.errorResponse(w, r, http.StatusUnauthorized, message)
}

func (app *application) inactiveAccoutResponse(w http.ResponseWriter, r *http.Request) {
	message := "your account must be authenticated"
	app.errorResponse(w, r, http.StatusForbidden, message)
}

func (app *application) invalidActivationTokenResponse(w http.ResponseWriter, r *http.Request) {
	message := "your activation token is invalid or expired"
	app.errorResponse(w, r, http.StatusUnprocessableEntity, message)
}

func (app *application) editConflictResponse(w http.ResponseWriter, r *http.Request) {
	message := "edit conflict occured, please try again later"
	app.errorResponse(w, r, http.StatusConflict, message)
}
