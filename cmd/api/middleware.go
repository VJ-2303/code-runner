package main

import (
	"errors"
	"net/http"
	"strings"

	"github.com/VJ-2303/code-runner/internal/data"
)

func (app *application) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Authorization")

		authorizationHeader := r.Header.Get("Authorization")

		if authorizationHeader == "" {
			r = contextSetUser(r, data.AnonymousUser)
			next.ServeHTTP(w, r)
			return
		}

		headerParts := strings.Split(authorizationHeader, " ")

		if len(headerParts) != 2 || headerParts[0] != "Bearer" {
			app.invalidAuthenticationTokenResponse(w, r)
			return
		}

		token := headerParts[1]

		user, err := app.models.Users.GetForToken(data.ScopeAuthentication, token)
		if err != nil {
			switch {
			case errors.Is(err, data.ErrRecordNotFound):
				app.invalidAuthenticationTokenResponse(w, r)
			default:
				app.serverErrorResponse(w, r, err)
			}
			return
		}
		r = contextSetUser(r, user)

		next.ServeHTTP(w, r)
	})
}

func (app *application) requireAPIKey(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("X-API-Key")
		if apiKey != "" {
			user, err := app.models.ApiKeys.GetUserForApiKey(apiKey)
			if err != nil {
				switch {
				case errors.Is(err, data.ErrRecordNotFound):
					app.invalidApiKeyResponse(w, r)
				default:
					app.serverErrorResponse(w, r, err)
				}
				return
			}
			r = contextSetUser(r, user)
			next.ServeHTTP(w, r)
			return
		}
		app.invalidApiKeyResponse(w, r)
		return
	})
}

func (app *application) requireAuthenticatedUser(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := contextGetUser(r)

		if user.IsAnonymous() {
			app.authenticationRequiredResponse(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (app *application) requireWithinLimit(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := contextGetUser(r)

		count, err := app.limiter.Increment(r.Context(), user.ID)
		if err != nil {
			app.serverErrorResponse(w, r, err)
			return
		}

		limit := user.DailyLimit

		if count > int64(limit) {
			app.usageLimitExceededResponse(w, r)
			return
		}

		next.ServeHTTP(w, r)
	})
}
