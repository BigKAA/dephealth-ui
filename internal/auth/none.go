package auth

import "net/http"

// noneAuth is a pass-through authenticator that allows all requests.
type noneAuth struct{}

func (a *noneAuth) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return next
	}
}

func (a *noneAuth) Routes() http.Handler {
	return nil
}
