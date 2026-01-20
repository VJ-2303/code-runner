package main

import (
	"context"
	"net/http"

	"github.com/VJ-2303/code-runner/internal/data"
)

type contextKey string

const userContextKey = contextKey("user")

func contextSetUser(r *http.Request, user *data.User) *http.Request {
	ctx := context.WithValue(r.Context(), userContextKey, user)
	return r.WithContext(ctx)
}

func contextGetUser(r *http.Request) *data.User {
	user, ok := r.Context().Value(userContextKey).(*data.User)
	if !ok {
		panic("missing user value in the request context")
	}
	return user
}
