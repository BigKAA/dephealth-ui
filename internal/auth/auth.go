package auth

import (
	"fmt"
	"net/http"

	"github.com/BigKAA/dephealth-ui/internal/config"
)

// Authenticator provides HTTP middleware for authentication.
type Authenticator interface {
	Middleware() func(http.Handler) http.Handler
}

// NewFromConfig creates an Authenticator based on the configuration.
func NewFromConfig(cfg config.AuthConfig) (Authenticator, error) {
	switch cfg.Type {
	case "none", "":
		return &noneAuth{}, nil
	case "basic":
		if len(cfg.Basic.Users) == 0 {
			return nil, fmt.Errorf("auth type \"basic\" requires at least one user")
		}
		users := make([]User, len(cfg.Basic.Users))
		for i, u := range cfg.Basic.Users {
			if u.Username == "" {
				return nil, fmt.Errorf("auth.basic.users[%d]: username is required", i)
			}
			if u.PasswordHash == "" {
				return nil, fmt.Errorf("auth.basic.users[%d]: passwordHash is required", i)
			}
			users[i] = User{
				Username:     u.Username,
				PasswordHash: u.PasswordHash,
			}
		}
		return NewBasic(users), nil
	default:
		return nil, fmt.Errorf("unknown auth type: %q", cfg.Type)
	}
}
