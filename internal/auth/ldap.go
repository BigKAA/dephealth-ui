package auth

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-ldap/ldap/v3"

	"github.com/BigKAA/dephealth-ui/internal/config"
)

const ldapDialTimeout = 10 * time.Second

// ldapAuth implements LDAP bind authentication.
type ldapAuth struct {
	cfg          config.LDAPConfig
	sessions     *SessionStore
	secureCookie bool
	logger       *slog.Logger
}

// NewLDAP creates an LDAP authenticator.
func NewLDAP(cfg config.AuthConfig, logger *slog.Logger) (Authenticator, error) {
	lcfg := cfg.LDAP

	// Validate compound filter in direct bind mode.
	if lcfg.BindDN == "" && (strings.Contains(lcfg.UserFilter, "&") || strings.Contains(lcfg.UserFilter, "|")) {
		return nil, fmt.Errorf("LDAP direct bind mode does not support compound filters (userFilter contains '&' or '|'); set bindDN for search bind mode")
	}

	return &ldapAuth{
		cfg:          lcfg,
		sessions:     NewSessionStore(sessionTTL),
		secureCookie: cfg.SecureCookie,
		logger:       logger,
	}, nil
}

// Middleware returns HTTP middleware that validates the session cookie.
func (a *ldapAuth) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(sessionCookieName)
			if err != nil {
				a.unauthorizedJSON(w)
				return
			}

			sess := a.sessions.Get(cookie.Value)
			if sess == nil {
				a.unauthorizedJSON(w)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Routes returns a chi.Router with LDAP auth endpoints.
func (a *ldapAuth) Routes() http.Handler {
	r := chi.NewRouter()
	r.Get("/login", a.handleLoginGET)
	r.Post("/login", a.handleLoginPOST)
	r.Get("/logout", a.handleLogout)
	r.Get("/userinfo", a.handleUserInfo)
	return r
}

// Stop releases resources.
func (a *ldapAuth) Stop() {
	a.sessions.Stop()
}

func (a *ldapAuth) connect() (*ldap.Conn, error) {
	tlsCfg := &tls.Config{
		InsecureSkipVerify: a.cfg.InsecureSkipVerify, //nolint:gosec // configurable for dev environments
	}
	dialer := &net.Dialer{Timeout: ldapDialTimeout}

	var opts []ldap.DialOpt
	opts = append(opts, ldap.DialWithDialer(dialer))
	if strings.HasPrefix(a.cfg.URL, "ldaps://") {
		opts = append(opts, ldap.DialWithTLSConfig(tlsCfg))
	}

	conn, err := ldap.DialURL(a.cfg.URL, opts...)
	if err != nil {
		return nil, fmt.Errorf("LDAP dial %q: %w", a.cfg.URL, err)
	}

	if a.cfg.StartTLS && strings.HasPrefix(a.cfg.URL, "ldap://") {
		if err := conn.StartTLS(tlsCfg); err != nil {
			conn.Close()
			return nil, fmt.Errorf("LDAP StartTLS: %w", err)
		}
	}
	return conn, nil
}

// authenticate performs LDAP authentication and returns user info.
func (a *ldapAuth) authenticate(username, password string) (*UserInfo, error) {
	conn, err := a.connect()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	escapedUsername := ldap.EscapeFilter(username)
	renderedFilter := strings.ReplaceAll(a.cfg.UserFilter, "{{.Username}}", escapedUsername)

	var userDN string
	var displayName, email string

	if a.cfg.BindDN != "" {
		// Search bind mode.
		if err := conn.Bind(a.cfg.BindDN, a.cfg.BindPassword); err != nil {
			return nil, fmt.Errorf("LDAP service account bind: %w", err)
		}

		searchReq := ldap.NewSearchRequest(
			a.cfg.BaseDN,
			ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
			renderedFilter,
			[]string{"dn", a.cfg.Attributes.DisplayName, a.cfg.Attributes.Email},
			nil,
		)

		result, err := conn.Search(searchReq)
		if err != nil {
			return nil, fmt.Errorf("LDAP user search: %w", err)
		}

		if len(result.Entries) == 0 {
			return nil, fmt.Errorf("LDAP user not found")
		}
		if len(result.Entries) > 1 {
			return nil, fmt.Errorf("LDAP search returned %d entries, expected 1", len(result.Entries))
		}

		entry := result.Entries[0]
		userDN = entry.DN
		displayName = entry.GetAttributeValue(a.cfg.Attributes.DisplayName)
		email = entry.GetAttributeValue(a.cfg.Attributes.Email)

		// Re-bind as user to verify password.
		if err := conn.Bind(userDN, password); err != nil {
			return nil, fmt.Errorf("LDAP user bind: %w", err)
		}
	} else {
		// Direct bind mode.
		stripped := strings.TrimPrefix(renderedFilter, "(")
		stripped = strings.TrimSuffix(stripped, ")")
		userDN = stripped + "," + a.cfg.BaseDN

		if err := conn.Bind(userDN, password); err != nil {
			return nil, fmt.Errorf("LDAP direct bind: %w", err)
		}

		// Search for own attributes.
		searchReq := ldap.NewSearchRequest(
			userDN,
			ldap.ScopeBaseObject, ldap.NeverDerefAliases, 0, 0, false,
			"(objectClass=*)",
			[]string{a.cfg.Attributes.DisplayName, a.cfg.Attributes.Email},
			nil,
		)

		result, err := conn.Search(searchReq)
		if err != nil {
			a.logger.Warn("LDAP attribute search failed (direct bind)", "error", err)
		} else if len(result.Entries) > 0 {
			displayName = result.Entries[0].GetAttributeValue(a.cfg.Attributes.DisplayName)
			email = result.Entries[0].GetAttributeValue(a.cfg.Attributes.Email)
		}
	}

	// Group membership check.
	if len(a.cfg.AllowedGroups) > 0 {
		if err := a.checkGroupMembership(conn, userDN); err != nil {
			return nil, err
		}
	}

	return &UserInfo{
		Subject: userDN,
		Name:    displayName,
		Email:   email,
	}, nil
}

func (a *ldapAuth) checkGroupMembership(conn *ldap.Conn, userDN string) error {
	// Re-bind as service account if available.
	if a.cfg.BindDN != "" {
		if err := conn.Bind(a.cfg.BindDN, a.cfg.BindPassword); err != nil {
			return fmt.Errorf("LDAP service account re-bind for group check: %w", err)
		}
	}

	escapedUserDN := ldap.EscapeFilter(userDN)
	renderedGroupFilter := strings.ReplaceAll(a.cfg.GroupFilter, "{{.UserDN}}", escapedUserDN)

	searchReq := ldap.NewSearchRequest(
		a.cfg.GroupBaseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		renderedGroupFilter,
		[]string{"dn"},
		nil,
	)

	result, err := conn.Search(searchReq)
	if err != nil {
		return fmt.Errorf("LDAP group search: %w", err)
	}

	allowed := make(map[string]bool, len(a.cfg.AllowedGroups))
	for _, g := range a.cfg.AllowedGroups {
		allowed[strings.ToLower(g)] = true
	}

	for _, entry := range result.Entries {
		if allowed[strings.ToLower(entry.DN)] {
			return nil
		}
	}

	return fmt.Errorf("user is not a member of any allowed group")
}

func (a *ldapAuth) handleLoginGET(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, `<!DOCTYPE html><html><body><h1>Login</h1>
<form method="POST" action="/auth/login">
<input name="username" placeholder="Username" autofocus><br>
<input name="password" type="password" placeholder="Password"><br>
<button type="submit">Login</button>
</form></body></html>`)
}

func (a *ldapAuth) handleLoginPOST(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")

	if username == "" || password == "" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `<!DOCTYPE html><html><body><h1>Login</h1>
<p style="color:red">Username and password are required</p>
<form method="POST" action="/auth/login">
<input name="username" placeholder="Username" autofocus><br>
<input name="password" type="password" placeholder="Password"><br>
<button type="submit">Login</button>
</form></body></html>`)
		return
	}

	user, err := a.authenticate(username, password)
	if err != nil {
		a.logger.Warn("LDAP authentication failed", "username", username, "error", err)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `<!DOCTYPE html><html><body><h1>Login</h1>
<p style="color:red">Invalid username or password</p>
<form method="POST" action="/auth/login">
<input name="username" placeholder="Username" autofocus><br>
<input name="password" type="password" placeholder="Password"><br>
<button type="submit">Login</button>
</form></body></html>`)
		return
	}

	sessionID, err := a.sessions.Create(*user)
	if err != nil {
		a.logger.Error("failed to create session", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		Secure:   a.secureCookie,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(sessionTTL.Seconds()),
	})

	http.Redirect(w, r, "/", http.StatusFound)
}

func (a *ldapAuth) handleLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sessionCookieName)
	if err == nil {
		a.sessions.Delete(cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   a.secureCookie,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})

	http.Redirect(w, r, "/", http.StatusFound)
}

func (a *ldapAuth) handleUserInfo(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		a.unauthorizedJSON(w)
		return
	}

	sess := a.sessions.Get(cookie.Value)
	if sess == nil {
		a.unauthorizedJSON(w)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(sess.User)
}

func (a *ldapAuth) unauthorizedJSON(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = fmt.Fprint(w, `{"error":"unauthorized"}`)
}
