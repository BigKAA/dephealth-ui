package auth

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/BigKAA/dephealth-ui/internal/config"
)

// Authenticator provides HTTP middleware for authentication.
type Authenticator interface {
	Middleware() func(http.Handler) http.Handler
	// Routes returns an http.Handler with auth-specific routes (e.g. /login, /callback).
	// Returns nil if the auth type does not require additional routes.
	Routes() http.Handler
}

// NewFromConfig creates an Authenticator based on the configuration.
// For OIDC, use NewFromConfigWithContext which performs provider discovery.
func NewFromConfig(cfg config.AuthConfig) (Authenticator, error) {
	return NewFromConfigWithContext(context.Background(), cfg, nil)
}

// NewFromConfigWithContext creates an Authenticator with context and logger.
// The context is used for OIDC provider discovery (fail-fast on startup).
func NewFromConfigWithContext(ctx context.Context, cfg config.AuthConfig, logger *slog.Logger) (Authenticator, error) {
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
	case "oidc":
		if logger == nil {
			logger = slog.Default()
		}
		return NewOIDC(ctx, cfg.OIDC, logger)
	default:
		return nil, fmt.Errorf("unknown auth type: %q", cfg.Type)
	}
}
