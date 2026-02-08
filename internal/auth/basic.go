package auth

import (
	"crypto/subtle"
	"net/http"

	"golang.org/x/crypto/bcrypt"
)

// User represents a Basic auth user with a bcrypt-hashed password.
type User struct {
	Username     string
	PasswordHash string
}

// basicAuth implements HTTP Basic authentication with bcrypt password verification.
type basicAuth struct {
	users []User
}

// NewBasic creates a new Basic auth authenticator.
func NewBasic(users []User) Authenticator {
	return &basicAuth{users: users}
}

func (a *basicAuth) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			username, password, ok := r.BasicAuth()
			if !ok {
				a.unauthorized(w)
				return
			}

			if !a.validate(username, password) {
				a.unauthorized(w)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func (a *basicAuth) validate(username, password string) bool {
	for _, u := range a.users {
		if subtle.ConstantTimeCompare([]byte(u.Username), []byte(username)) == 1 {
			if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err == nil {
				return true
			}
			return false
		}
	}
	return false
}

func (a *basicAuth) Routes() http.Handler {
	return nil
}

func (a *basicAuth) unauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Basic realm="dephealth-ui"`)
	http.Error(w, "Unauthorized", http.StatusUnauthorized)
}
