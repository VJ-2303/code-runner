package main

import "net/http"

func (app *application) router() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /v1/healthcheck", app.healthcheckHandler)

	mux.HandleFunc("POST /v1/snippets", app.createSnippetHandler)
	mux.HandleFunc("GET /v1/snippets/{id}", app.showSnippetHandler)

	mux.HandleFunc("POST /v1/run", app.runCodeHandler)

	mux.HandleFunc("POST /v1/users", app.registerUserHandler)
	mux.HandleFunc("POST /v1/tokens/authentication", app.createAuthenticationTokenHandler)

	return mux
}
