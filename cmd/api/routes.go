package main

import "net/http"

func (app *application) router() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /v1/healthcheck", app.healthcheckHandler)

	mux.HandleFunc("POST /v1/snippets", app.createSnippetHandler)
	mux.HandleFunc("GET /v1/snippets/{id}", app.showSnippetHandler)

	mux.HandleFunc("POST /v1/run", app.runCodeHandler)

	return mux
}
