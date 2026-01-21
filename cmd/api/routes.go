package main

import "net/http"

func (app *application) router() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /v1/healthcheck", app.healthcheckHandler)

	mux.HandleFunc("POST /v1/snippets", app.requireAuthenticatedUser(app.createSnippetHandler))

	mux.HandleFunc("POST /v1/run", app.requireAuthenticatedUser(app.runCodeHandler))

	mux.HandleFunc("POST /v1/users", app.registerUserHandler)
	mux.HandleFunc("POST /v1/tokens/authentication", app.createAuthenticationTokenHandler)
	mux.HandleFunc("GET /v1/users/activate", app.activateUserHandler)

	return app.authenticate(mux)
}
