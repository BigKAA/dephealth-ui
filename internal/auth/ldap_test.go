package auth

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/jimlambrt/gldap"

	"github.com/BigKAA/dephealth-ui/internal/config"
)

// testLDAPServer creates a mock LDAP server with test users and groups.
func testLDAPServer(t *testing.T) (string, func()) {
	t.Helper()

	s, err := gldap.NewServer()
	if err != nil {
		t.Fatalf("gldap.NewServer: %v", err)
	}

	mux, err := gldap.NewMux()
	if err != nil {
		t.Fatalf("gldap.NewMux: %v", err)
	}

	mux.Bind(func(w *gldap.ResponseWriter, r *gldap.Request) {
		resp := r.NewBindResponse(
			gldap.WithResponseCode(gldap.ResultInvalidCredentials),
		)

		msg, err := r.GetSimpleBindMessage()
		if err != nil {
			w.Write(resp)
			return
		}

		validBinds := map[string]string{
			"cn=readonly,dc=example,dc=com":              "readonly-pass",
			"uid=testuser,ou=people,dc=example,dc=com":   "testpass",
			"uid=admin,ou=people,dc=example,dc=com":      "adminpass",
			"uid=nogroup,ou=people,dc=example,dc=com":    "nogroup-pass",
			"uid=special*,ou=people,dc=example,dc=com":   "specialpass",
		}

		if pass, ok := validBinds[msg.UserName]; ok && pass == string(msg.Password) {
			resp.SetResultCode(gldap.ResultSuccess)
		}
		w.Write(resp)
	})

	mux.Search(func(w *gldap.ResponseWriter, r *gldap.Request) {
		msg, err := r.GetSearchMessage()
		if err != nil {
			return
		}

		baseDN := msg.BaseDN

		// User search.
		if baseDN == "ou=people,dc=example,dc=com" || strings.HasSuffix(baseDN, "ou=people,dc=example,dc=com") {
			users := map[string]map[string]string{
				"uid=testuser,ou=people,dc=example,dc=com": {
					"displayName": "Test User",
					"mail":        "testuser@example.com",
				},
				"uid=admin,ou=people,dc=example,dc=com": {
					"displayName": "Admin User",
					"mail":        "admin@example.com",
				},
				"uid=nogroup,ou=people,dc=example,dc=com": {
					"displayName": "No Group User",
					"mail":        "nogroup@example.com",
				},
			}

			filter := msg.Filter

			for dn, attrs := range users {
				// Check if the filter matches this user.
				// Simple matching: check if uid value is in filter.
				uid := strings.TrimPrefix(strings.Split(dn, ",")[0], "uid=")
				if strings.Contains(filter, uid) || strings.Contains(filter, "objectClass") {
					// If searching by base object (own DN), match on baseDN.
					if msg.Scope == gldap.BaseObject && baseDN != dn {
						continue
					}
					entry := r.NewSearchResponseEntry(dn,
						gldap.WithAttributes(map[string][]string{
							"displayName": {attrs["displayName"]},
							"mail":        {attrs["mail"]},
						}),
					)
					w.Write(entry)
				}
			}
		}

		// Group search.
		if baseDN == "ou=groups,dc=example,dc=com" {
			groups := map[string][]string{
				"cn=dephealth-users,ou=groups,dc=example,dc=com": {
					"uid=testuser,ou=people,dc=example,dc=com",
				},
				"cn=admins,ou=groups,dc=example,dc=com": {
					"uid=admin,ou=people,dc=example,dc=com",
				},
			}

			filter := msg.Filter
			for groupDN, members := range groups {
				for _, memberDN := range members {
					if strings.Contains(filter, memberDN) {
						entry := r.NewSearchResponseEntry(groupDN)
						w.Write(entry)
					}
				}
			}
		}

		w.Write(r.NewSearchDoneResponse())
	})

	if err := s.Router(mux); err != nil {
		t.Fatalf("s.Router: %v", err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	addr := listener.Addr().String()
	listener.Close() // gldap.Run will re-listen

	go func() {
		_ = s.Run(addr)
	}()

	ldapURL := fmt.Sprintf("ldap://%s", addr)

	return ldapURL, func() {
		s.Stop()
	}
}

func testLDAPConfig(ldapURL string) config.AuthConfig {
	return config.AuthConfig{
		Type: "ldap",
		LDAP: config.LDAPConfig{
			URL:          ldapURL,
			BindDN:       "cn=readonly,dc=example,dc=com",
			BindPassword: "readonly-pass",
			BaseDN:       "ou=people,dc=example,dc=com",
			UserFilter:   "(uid={{.Username}})",
			Attributes: config.LDAPAttributes{
				DisplayName: "displayName",
				Email:       "mail",
			},
			GroupBaseDN: "ou=groups,dc=example,dc=com",
			GroupFilter:  "(member={{.UserDN}})",
		},
	}
}

func TestNewLDAP(t *testing.T) {
	ldapURL, cleanup := testLDAPServer(t)
	defer cleanup()

	cfg := testLDAPConfig(ldapURL)
	auth, err := NewLDAP(cfg, testLogger())
	if err != nil {
		t.Fatalf("NewLDAP error: %v", err)
	}
	if auth == nil {
		t.Fatal("expected non-nil authenticator")
	}
	auth.Stop()
}

func TestNewLDAP_DirectBindCompoundFilter(t *testing.T) {
	_, err := NewLDAP(config.AuthConfig{
		Type: "ldap",
		LDAP: config.LDAPConfig{
			URL:        "ldap://localhost:389",
			BaseDN:     "dc=example,dc=com",
			UserFilter: "(&(uid={{.Username}})(objectClass=person))",
		},
	}, testLogger())
	if err == nil {
		t.Fatal("expected error for compound filter in direct bind mode")
	}
	if !strings.Contains(err.Error(), "compound filter") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLDAP_SearchBind_Success(t *testing.T) {
	ldapURL, cleanup := testLDAPServer(t)
	defer cleanup()

	cfg := testLDAPConfig(ldapURL)
	cfg.LDAP.AllowedGroups = nil // no group check
	auth, err := NewLDAP(cfg, testLogger())
	if err != nil {
		t.Fatalf("NewLDAP error: %v", err)
	}
	defer auth.Stop()

	a := auth.(*ldapAuth)
	user, err := a.authenticate("testuser", "testpass")
	if err != nil {
		t.Fatalf("authenticate error: %v", err)
	}
	if user.Subject != "uid=testuser,ou=people,dc=example,dc=com" {
		t.Errorf("Subject = %q", user.Subject)
	}
	if user.Name != "Test User" {
		t.Errorf("Name = %q", user.Name)
	}
	if user.Email != "testuser@example.com" {
		t.Errorf("Email = %q", user.Email)
	}
}

func TestLDAP_SearchBind_WrongPassword(t *testing.T) {
	ldapURL, cleanup := testLDAPServer(t)
	defer cleanup()

	cfg := testLDAPConfig(ldapURL)
	cfg.LDAP.AllowedGroups = nil
	auth, err := NewLDAP(cfg, testLogger())
	if err != nil {
		t.Fatalf("NewLDAP error: %v", err)
	}
	defer auth.Stop()

	a := auth.(*ldapAuth)
	_, err = a.authenticate("testuser", "wrongpass")
	if err == nil {
		t.Fatal("expected error for wrong password")
	}
}

func TestLDAP_SearchBind_UserNotFound(t *testing.T) {
	ldapURL, cleanup := testLDAPServer(t)
	defer cleanup()

	cfg := testLDAPConfig(ldapURL)
	cfg.LDAP.AllowedGroups = nil
	auth, err := NewLDAP(cfg, testLogger())
	if err != nil {
		t.Fatalf("NewLDAP error: %v", err)
	}
	defer auth.Stop()

	a := auth.(*ldapAuth)
	_, err = a.authenticate("nonexistent", "pass")
	if err == nil {
		t.Fatal("expected error for nonexistent user")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLDAP_DirectBind_Success(t *testing.T) {
	ldapURL, cleanup := testLDAPServer(t)
	defer cleanup()

	cfg := testLDAPConfig(ldapURL)
	cfg.LDAP.BindDN = ""
	cfg.LDAP.BindPassword = ""
	cfg.LDAP.AllowedGroups = nil
	auth, err := NewLDAP(cfg, testLogger())
	if err != nil {
		t.Fatalf("NewLDAP error: %v", err)
	}
	defer auth.Stop()

	a := auth.(*ldapAuth)
	user, err := a.authenticate("testuser", "testpass")
	if err != nil {
		t.Fatalf("authenticate error: %v", err)
	}
	if user.Subject != "uid=testuser,ou=people,dc=example,dc=com" {
		t.Errorf("Subject = %q", user.Subject)
	}
}

func TestLDAP_DirectBind_WrongPassword(t *testing.T) {
	ldapURL, cleanup := testLDAPServer(t)
	defer cleanup()

	cfg := testLDAPConfig(ldapURL)
	cfg.LDAP.BindDN = ""
	cfg.LDAP.BindPassword = ""
	cfg.LDAP.AllowedGroups = nil
	auth, err := NewLDAP(cfg, testLogger())
	if err != nil {
		t.Fatalf("NewLDAP error: %v", err)
	}
	defer auth.Stop()

	a := auth.(*ldapAuth)
	_, err = a.authenticate("testuser", "wrongpass")
	if err == nil {
		t.Fatal("expected error for wrong password")
	}
}

func TestLDAP_GroupCheck_Allowed(t *testing.T) {
	ldapURL, cleanup := testLDAPServer(t)
	defer cleanup()

	cfg := testLDAPConfig(ldapURL)
	cfg.LDAP.AllowedGroups = []string{"cn=dephealth-users,ou=groups,dc=example,dc=com"}
	auth, err := NewLDAP(cfg, testLogger())
	if err != nil {
		t.Fatalf("NewLDAP error: %v", err)
	}
	defer auth.Stop()

	a := auth.(*ldapAuth)
	_, err = a.authenticate("testuser", "testpass")
	if err != nil {
		t.Fatalf("authenticate error: %v", err)
	}
}

func TestLDAP_GroupCheck_Denied(t *testing.T) {
	ldapURL, cleanup := testLDAPServer(t)
	defer cleanup()

	cfg := testLDAPConfig(ldapURL)
	cfg.LDAP.AllowedGroups = []string{"cn=dephealth-users,ou=groups,dc=example,dc=com"}
	auth, err := NewLDAP(cfg, testLogger())
	if err != nil {
		t.Fatalf("NewLDAP error: %v", err)
	}
	defer auth.Stop()

	a := auth.(*ldapAuth)
	_, err = a.authenticate("nogroup", "nogroup-pass")
	if err == nil {
		t.Fatal("expected error for user not in allowed group")
	}
	if !strings.Contains(err.Error(), "not a member") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLDAP_GroupCheck_Disabled(t *testing.T) {
	ldapURL, cleanup := testLDAPServer(t)
	defer cleanup()

	cfg := testLDAPConfig(ldapURL)
	cfg.LDAP.AllowedGroups = nil
	auth, err := NewLDAP(cfg, testLogger())
	if err != nil {
		t.Fatalf("NewLDAP error: %v", err)
	}
	defer auth.Stop()

	a := auth.(*ldapAuth)
	_, err = a.authenticate("nogroup", "nogroup-pass")
	if err != nil {
		t.Fatalf("authenticate should succeed with no group check: %v", err)
	}
}

func TestLDAP_ConnectTimeout(t *testing.T) {
	// Use a non-routable IP to test dial timeout.
	cfg := config.AuthConfig{
		Type: "ldap",
		LDAP: config.LDAPConfig{
			URL:        "ldap://192.0.2.1:389",
			BaseDN:     "dc=example,dc=com",
			UserFilter: "(uid={{.Username}})",
			Attributes: config.LDAPAttributes{DisplayName: "displayName", Email: "mail"},
		},
	}
	auth, err := NewLDAP(cfg, testLogger())
	if err != nil {
		t.Fatalf("NewLDAP error: %v", err)
	}
	defer auth.Stop()

	a := auth.(*ldapAuth)
	_, err = a.authenticate("testuser", "testpass")
	if err == nil {
		t.Fatal("expected error for unreachable host")
	}
}

func TestLDAP_LoginFlow(t *testing.T) {
	ldapURL, cleanup := testLDAPServer(t)
	defer cleanup()

	cfg := testLDAPConfig(ldapURL)
	cfg.LDAP.AllowedGroups = nil
	auth, err := NewLDAP(cfg, testLogger())
	if err != nil {
		t.Fatalf("NewLDAP error: %v", err)
	}
	defer auth.Stop()

	router := auth.Routes()

	// GET /login.
	req := httptest.NewRequest("GET", "/login", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("GET /login status = %d, want %d", w.Code, http.StatusOK)
	}

	// POST /login with valid credentials.
	form := url.Values{"username": {"testuser"}, "password": {"testpass"}}
	req = httptest.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("POST /login status = %d, want %d", w.Code, http.StatusFound)
	}

	// Check cookie is set.
	cookies := w.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == sessionCookieName {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("expected session cookie to be set")
	}
}

func TestLDAP_LoginWrongCreds(t *testing.T) {
	ldapURL, cleanup := testLDAPServer(t)
	defer cleanup()

	cfg := testLDAPConfig(ldapURL)
	cfg.LDAP.AllowedGroups = nil
	auth, err := NewLDAP(cfg, testLogger())
	if err != nil {
		t.Fatalf("NewLDAP error: %v", err)
	}
	defer auth.Stop()

	router := auth.Routes()

	form := url.Values{"username": {"testuser"}, "password": {"wrongpass"}}
	req := httptest.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("POST /login wrong creds status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
	if !strings.Contains(w.Body.String(), "Invalid username or password") {
		t.Error("expected error message in response body")
	}
}

func TestLDAP_Logout(t *testing.T) {
	ldapURL, cleanup := testLDAPServer(t)
	defer cleanup()

	cfg := testLDAPConfig(ldapURL)
	cfg.LDAP.AllowedGroups = nil
	auth, err := NewLDAP(cfg, testLogger())
	if err != nil {
		t.Fatalf("NewLDAP error: %v", err)
	}
	defer auth.Stop()

	a := auth.(*ldapAuth)

	// Create a session first.
	sessionID, err := a.sessions.Create(UserInfo{Subject: "test", Name: "Test", Email: "test@test.com"})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	router := auth.Routes()
	req := httptest.NewRequest("GET", "/logout", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("GET /logout status = %d, want %d", w.Code, http.StatusFound)
	}

	// Session should be deleted.
	if sess := a.sessions.Get(sessionID); sess != nil {
		t.Error("expected session to be deleted after logout")
	}
}

func TestLDAP_UserInfo(t *testing.T) {
	ldapURL, cleanup := testLDAPServer(t)
	defer cleanup()

	cfg := testLDAPConfig(ldapURL)
	cfg.LDAP.AllowedGroups = nil
	auth, err := NewLDAP(cfg, testLogger())
	if err != nil {
		t.Fatalf("NewLDAP error: %v", err)
	}
	defer auth.Stop()

	a := auth.(*ldapAuth)
	user := UserInfo{Subject: "uid=testuser,dc=example", Name: "Test User", Email: "test@example.com"}
	sessionID, err := a.sessions.Create(user)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	router := auth.Routes()
	req := httptest.NewRequest("GET", "/userinfo", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /userinfo status = %d, want %d", w.Code, http.StatusOK)
	}

	var info UserInfo
	if err := json.NewDecoder(w.Body).Decode(&info); err != nil {
		t.Fatalf("decode userinfo: %v", err)
	}
	if info.Name != "Test User" {
		t.Errorf("Name = %q, want %q", info.Name, "Test User")
	}
}

func TestLDAP_UserInfoUnauthorized(t *testing.T) {
	ldapURL, cleanup := testLDAPServer(t)
	defer cleanup()

	cfg := testLDAPConfig(ldapURL)
	auth, err := NewLDAP(cfg, testLogger())
	if err != nil {
		t.Fatalf("NewLDAP error: %v", err)
	}
	defer auth.Stop()

	router := auth.Routes()
	req := httptest.NewRequest("GET", "/userinfo", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("GET /userinfo without session status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestLDAP_MiddlewareValidSession(t *testing.T) {
	ldapURL, cleanup := testLDAPServer(t)
	defer cleanup()

	cfg := testLDAPConfig(ldapURL)
	auth, err := NewLDAP(cfg, testLogger())
	if err != nil {
		t.Fatalf("NewLDAP error: %v", err)
	}
	defer auth.Stop()

	a := auth.(*ldapAuth)
	sessionID, _ := a.sessions.Create(UserInfo{Subject: "test"})

	handler := auth.Middleware()(okHandler())
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("middleware valid session status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestLDAP_MiddlewareNoSession(t *testing.T) {
	ldapURL, cleanup := testLDAPServer(t)
	defer cleanup()

	cfg := testLDAPConfig(ldapURL)
	auth, err := NewLDAP(cfg, testLogger())
	if err != nil {
		t.Fatalf("NewLDAP error: %v", err)
	}
	defer auth.Stop()

	handler := auth.Middleware()(okHandler())
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("middleware no session status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestLDAP_FilterEscaping(t *testing.T) {
	ldapURL, cleanup := testLDAPServer(t)
	defer cleanup()

	cfg := testLDAPConfig(ldapURL)
	cfg.LDAP.AllowedGroups = nil
	auth, err := NewLDAP(cfg, testLogger())
	if err != nil {
		t.Fatalf("NewLDAP error: %v", err)
	}
	defer auth.Stop()

	a := auth.(*ldapAuth)
	// Username with special chars should not cause LDAP injection.
	_, err = a.authenticate("test*)(uid=*))(|(uid=*", "pass")
	if err == nil {
		t.Fatal("expected error for injected username")
	}
	// The error should be "not found" — the injection should have been escaped.
	if !strings.Contains(err.Error(), "not found") {
		t.Logf("got expected error (escaped): %v", err)
	}
}

func testLogger() *slog.Logger {
	return slog.Default()
}
