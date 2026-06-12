package main

import "net/http"

func (app *application) router() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /v1/healthcheck", app.healthcheckHandler)

	mux.HandleFunc("POST /v1/run", app.requireAPIKey(app.requireWithinLimit(app.runCodeHandler)))

	mux.HandleFunc("POST /v1/users", app.registerUserHandler)
	mux.HandleFunc("GET /v1/users/activate", app.activateUserHandler)

	mux.HandleFunc("POST /v1/tokens/authentication", app.createAuthenticationTokenHandler)
	mux.HandleFunc("DELETE /v1/tokens/authentication", app.requireAuthenticatedUser(app.deleteAuthenticationTokenHandler))

	mux.HandleFunc("POST /v1/apikeys", app.requireAuthenticatedUser(app.createApiKeyHandler))
	mux.HandleFunc("GET /v1/apikeys", app.requireAuthenticatedUser(app.listApiKeysHandler))
	mux.HandleFunc("DELETE /v1/apikeys", app.requireAuthenticatedUser(app.deleteApiKeyHandler))

	mux.HandleFunc("GET /v1/usage", app.requireAuthenticatedUser(app.getUsageHandler))

	return app.authenticate(mux)
}
