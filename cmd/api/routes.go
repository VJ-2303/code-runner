package main

import "net/http"

func (app *application) router() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /v1/healthcheck", app.healthcheckHandler)

	mux.HandleFunc("POST /v1/snippets", app.requireAuthenticatedUser(app.createSnippetHandler))
	mux.HandleFunc("GET /v1/snippets", app.requireAuthenticatedUser(app.GetAllSnippetHandler))
	mux.HandleFunc("GET /v1/snippets/{id}", app.requireAuthenticatedUser(app.getSnippetHandler))
	mux.HandleFunc("PATCH /v1/snippets/{id}", app.requireAuthenticatedUser(app.updateSnippetHandler))
	mux.HandleFunc("DELETE /v1/snippets/{id}", app.requireAuthenticatedUser(app.deleteSnippetHandler))

	mux.HandleFunc("POST /v1/run", app.requireAuthenticatedUser(app.runCodeHandler))

	mux.HandleFunc("POST /v1/users", app.registerUserHandler)
	mux.HandleFunc("DELETE /v1/tokens/authentication", app.requireAuthenticatedUser(app.deleteAuthenticationTokenHandler))
	mux.HandleFunc("POST /v1/tokens/authentication", app.createAuthenticationTokenHandler)
	mux.HandleFunc("GET /v1/users/activate", app.activateUserHandler)
	mux.HandleFunc("GET /v1/users/profile", app.requireAuthenticatedUser(app.getProfileHandler))

	return app.rateLimit(app.enableCORS(app.authenticate(mux)))
}
