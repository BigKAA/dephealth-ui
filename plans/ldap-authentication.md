# Plan: LDAP Authentication

## Metadata

- **Version**: 1.4.0
- **Created**: 2026-03-10
- **Updated**: 2026-03-10
- **Status**: Pending

---

## Version History

- **v1.4.0** (2026-03-10): Phase 4 review — fixed deployment.yml `if or` condition, added secureCookie to values.yaml, corrected doc files (docs/API.md + Helm README instead of docs/README.md), removed fragile line number, `includes()` instead of `||`, added limitations documentation, helm template dry-run step, lint in completion criteria
- **v1.3.0** (2026-03-10): Phase 3 review — fixed IP extraction for rate limiter (X-Forwarded-For support), added CSRF max tokens cap, username preservation on error, explicit struct field additions for loginTmpl/limiter, rate limiter in separate file, clientIP helper with proxy header support, throttled stale entry cleanup, CSRF/username e2e checks, OpenLDAP in dephealth-infra
- **v1.2.0** (2026-03-10): Phase 2 review — fixed secureCookie logic (was based on LDAP URL, now shared config flag), moved session constants to session.go (new item 2.2), fixed EscapeDN→EscapeFilter for group filter, documented direct bind limitations, added dial timeout, full TLS config spec, removed placeholder struct fields, added missing tests (connection timeout, multiple results, compound filter rejection)
- **v1.1.0** (2026-03-10): Phase 1 review — added missing Attributes env vars, URL scheme validation, bool parsing pattern, AllowedGroups trim, YAML test details, factory positive test, NewLDAP stub signature
- **v1.0.0** (2026-03-10): Initial plan

---

## Current Status

- **Active phase**: Phase 4
- **Active item**: 4.4 (manual verification)
- **Updated**: 2026-03-10
- **Note**: Phases 1–3 completed, Phase 4 items 4.1–4.3 completed

---

## Table of Contents

- [x] [Phase 1: Configuration and Types](#phase-1-configuration-and-types)
- [x] [Phase 2: LDAP Authenticator Core](#phase-2-ldap-authenticator-core)
- [x] [Phase 3: Login Template and Rate Limiting](#phase-3-login-template-and-rate-limiting)
- [ ] [Phase 4: Frontend, Helm Chart and Documentation](#phase-4-frontend-helm-chart-and-documentation)

---

## Phase 1: Configuration and Types

**Dependencies**: None
**Status**: Done

### Description

Add `LDAPConfig` type to the config package, wire it into `AuthConfig`, add validation
and environment variable overrides. Update the auth factory to recognize `type: ldap`.
All existing tests must continue to pass; new tests cover LDAP config loading and validation.

### Items

- [x] **1.1 Add LDAPConfig types to config package**
  - **Dependencies**: None
  - **Description**: Add `LDAPConfig` and `LDAPAttributes` structs to `internal/config/config.go`.
    Add `LDAP LDAPConfig` field to `AuthConfig`. Set default `UserFilter` value
    `(uid={{.Username}})` in `defaultConfig()`.
  - **Modifies**:
    - `internal/config/config.go`
  - **Details**:
    ```go
    // LDAPConfig holds LDAP authentication settings.
    type LDAPConfig struct {
        URL                string         `yaml:"url"`
        StartTLS           bool           `yaml:"startTLS"`
        InsecureSkipVerify bool           `yaml:"insecureSkipVerify"`
        BindDN             string         `yaml:"bindDN"`
        BindPassword       string         `yaml:"bindPassword"`
        BaseDN             string         `yaml:"baseDN"`
        UserFilter         string         `yaml:"userFilter"`
        Attributes         LDAPAttributes `yaml:"attributes"`
        GroupBaseDN        string         `yaml:"groupBaseDN"`
        GroupFilter        string         `yaml:"groupFilter"`
        AllowedGroups      []string       `yaml:"allowedGroups"`
    }

    // LDAPAttributes maps LDAP entry attributes to UserInfo fields.
    type LDAPAttributes struct {
        DisplayName string `yaml:"displayName"`
        Email       string `yaml:"email"`
    }
    ```
    Default in `defaultConfig()`:
    ```go
    Auth: AuthConfig{
        Type: "none",
        LDAP: LDAPConfig{
            UserFilter: "(uid={{.Username}})",
            Attributes: LDAPAttributes{
                DisplayName: "displayName",
                Email:       "mail",
            },
        },
    },
    ```

- [x] **1.2 Add LDAP validation to Validate()**
  - **Dependencies**: 1.1
  - **Description**: Add `case "ldap"` to `Validate()` switch. Required fields: `url`, `baseDN`.
    Update the `default` case error message to include `ldap` in supported types list.
    Update existing test case `"unknown auth type"` — change its type from `"ldap"` to
    `"kerberos"` (since `ldap` is now valid). Add new test cases for LDAP validation.
  - **Modifies**:
    - `internal/config/config.go` — `Validate()` method
    - `internal/config/config_test.go` — update `"unknown auth type"` test, add LDAP tests
  - **Details**:
    Validation rules:
    ```go
    case "ldap":
        if c.Auth.LDAP.URL == "" {
            return fmt.Errorf("auth.ldap.url is required when auth.type is \"ldap\"")
        }
        if !strings.HasPrefix(c.Auth.LDAP.URL, "ldap://") && !strings.HasPrefix(c.Auth.LDAP.URL, "ldaps://") {
            return fmt.Errorf("auth.ldap.url must start with \"ldap://\" or \"ldaps://\"")
        }
        if c.Auth.LDAP.BaseDN == "" {
            return fmt.Errorf("auth.ldap.baseDN is required when auth.type is \"ldap\"")
        }
    ```
    New test cases (add to table-driven `TestValidate`):
    - `"auth type ldap valid"` — url=`ldap://ldap:389` + baseDN set, wantErr=false
    - `"auth type ldap valid ldaps"` — url=`ldaps://ldap:636` + baseDN set, wantErr=false
    - `"auth type ldap missing url"` — wantErr=true
    - `"auth type ldap invalid url scheme"` — url=`http://ldap:389`, wantErr=true
    - `"auth type ldap missing baseDN"` — wantErr=true

- [x] **1.3 Add LDAP environment variable overrides**
  - **Dependencies**: 1.1
  - **Description**: Add env overrides in `applyEnvOverrides()` for all LDAP config fields.
    Add test `TestLDAPEnvOverrides`.
  - **Modifies**:
    - `internal/config/config.go` — `applyEnvOverrides()` function
    - `internal/config/config_test.go` — new test
  - **Details**:
    Environment variables:
    | Variable | Field |
    |----------|-------|
    | `DEPHEALTH_AUTH_LDAP_URL` | `LDAP.URL` |
    | `DEPHEALTH_AUTH_LDAP_STARTTLS` | `LDAP.StartTLS` (use `strings.EqualFold(v, "true") \|\| v == "1"`, same as `LOG_ADD_SOURCE`) |
    | `DEPHEALTH_AUTH_LDAP_INSECURE_SKIP_VERIFY` | `LDAP.InsecureSkipVerify` (same bool pattern) |
    | `DEPHEALTH_AUTH_LDAP_BIND_DN` | `LDAP.BindDN` |
    | `DEPHEALTH_AUTH_LDAP_BIND_PASSWORD` | `LDAP.BindPassword` |
    | `DEPHEALTH_AUTH_LDAP_BASE_DN` | `LDAP.BaseDN` |
    | `DEPHEALTH_AUTH_LDAP_USER_FILTER` | `LDAP.UserFilter` |
    | `DEPHEALTH_AUTH_LDAP_ATTRIBUTES_DISPLAYNAME` | `LDAP.Attributes.DisplayName` |
    | `DEPHEALTH_AUTH_LDAP_ATTRIBUTES_EMAIL` | `LDAP.Attributes.Email` |
    | `DEPHEALTH_AUTH_LDAP_GROUP_BASE_DN` | `LDAP.GroupBaseDN` |
    | `DEPHEALTH_AUTH_LDAP_GROUP_FILTER` | `LDAP.GroupFilter` |
    | `DEPHEALTH_AUTH_LDAP_ALLOWED_GROUPS` | `LDAP.AllowedGroups` (comma-separated, `strings.Split` + `strings.TrimSpace` each element, skip empty) |

- [x] **1.4 Add LDAP config YAML loading test**
  - **Dependencies**: 1.1
  - **Description**: Add `TestLoadLDAPConfig` — load LDAP config from YAML file, verify all
    fields are parsed correctly (including nested `attributes` and `allowedGroups` list).
  - **Modifies**:
    - `internal/config/config_test.go`
  - **Details**:
    Test YAML content:
    ```yaml
    server:
      listen: ":8080"
    datasources:
      prometheus:
        url: "http://vm:8428"
    auth:
      type: "ldap"
      ldap:
        url: "ldaps://ldap.example.com:636"
        startTLS: false
        insecureSkipVerify: true
        bindDN: "cn=readonly,dc=example,dc=com"
        bindPassword: "readonly-pass"
        baseDN: "ou=people,dc=example,dc=com"
        userFilter: "(uid={{.Username}})"
        attributes:
          displayName: "cn"
          email: "userEmail"
        groupBaseDN: "ou=groups,dc=example,dc=com"
        groupFilter: "(member={{.UserDN}})"
        allowedGroups:
          - "cn=dephealth-users,ou=groups,dc=example,dc=com"
          - "cn=admins,ou=groups,dc=example,dc=com"
    ```
    Verify: all fields including nested `attributes.displayName`, `attributes.email`,
    and `allowedGroups` slice length and values.

- [x] **1.5 Add LDAP case to auth factory**
  - **Dependencies**: 1.1
  - **Description**: Add `case "ldap"` to `NewFromConfigWithContext()` in `internal/auth/auth.go`.
    This will call `NewLDAP(cfg.LDAP, logger)` which doesn't exist yet — create a minimal
    stub in `internal/auth/ldap.go` that returns an error (`"not implemented"`) so the code
    compiles. Update `TestFactoryUnknownType` to use `"kerberos"` instead of `"ldap"`.
    Add `TestFactoryLDAP` — verify factory recognizes `type: "ldap"` and returns the
    stub's `"not implemented"` error (not `"unknown auth type"`).
  - **Modifies**:
    - `internal/auth/auth.go` — add case
    - `internal/auth/basic_test.go` — update `TestFactoryUnknownType`, add `TestFactoryLDAP`
  - **Creates**:
    - `internal/auth/ldap.go` — stub with `NewLDAP()` returning error
  - **Details**:
    Stub signature in `internal/auth/ldap.go`:
    ```go
    // NewLDAP creates an LDAP authenticator.
    func NewLDAP(cfg config.LDAPConfig, logger *slog.Logger) (Authenticator, error) {
        return nil, fmt.Errorf("LDAP authenticator is not implemented")
    }
    ```
    Factory test `TestFactoryLDAP`:
    ```go
    func TestFactoryLDAP(t *testing.T) {
        _, err := NewFromConfig(config.AuthConfig{
            Type: "ldap",
            LDAP: config.LDAPConfig{
                URL:    "ldap://localhost:389",
                BaseDN: "dc=example,dc=com",
            },
        })
        if err == nil {
            t.Fatal("expected error from LDAP stub")
        }
        if strings.Contains(err.Error(), "unknown auth type") {
            t.Errorf("factory should recognize ldap type, got: %v", err)
        }
    }
    ```

### Completion Criteria Phase 1

- [x] All items completed (1.1–1.5)
- [x] `go build ./...` succeeds
- [x] `go test ./internal/config/...` passes (including new LDAP tests)
- [x] `go test ./internal/auth/...` passes (factory test updated)

---

## Phase 2: LDAP Authenticator Core

**Dependencies**: Phase 1
**Status**: Done

### Description

Implement the `ldapAuth` struct that performs LDAP bind authentication, user attribute
retrieval, and optional group membership checks. Add the Go dependency `github.com/go-ldap/ldap/v3`.
Use a mock LDAP server in tests (package `github.com/jimlambrt/gldap`).

### Items

- [x] **2.1 Add go-ldap dependency**
  - **Dependencies**: None
  - **Description**: Run `go get github.com/go-ldap/ldap/v3` and
    `go get github.com/jimlambrt/gldap` (test dependency) to add them to `go.mod`.
  - **Modifies**:
    - `go.mod`
    - `go.sum`

- [x] **2.2 Move shared session constants to session.go**
  - **Dependencies**: None
  - **Description**: Move `sessionCookieName` and `sessionTTL` constants from
    `internal/auth/oidc.go` to `internal/auth/session.go` where they logically belong.
    Both OIDC and LDAP authenticators need them. Remove the constants from `oidc.go`.
    All existing tests must continue to pass without changes (same package).
  - **Modifies**:
    - `internal/auth/oidc.go` — remove `sessionCookieName` and `sessionTTL`
    - `internal/auth/session.go` — add `sessionCookieName` and `sessionTTL`

- [x] **2.3 Implement ldapAuth struct and NewLDAP constructor**
  - **Dependencies**: 2.1, 2.2
  - **Description**: Replace the stub in `internal/auth/ldap.go` with the full
    implementation. The `ldapAuth` struct holds config, `SessionStore`,
    `secureCookie` flag, and logger. Fields for login template and rate limiter
    will be added in Phase 3 when they are needed.
    `NewLDAP()` validates config, creates `SessionStore`, returns `*ldapAuth`.
  - **Modifies**:
    - `internal/auth/ldap.go`
  - **Details**:
    ```go
    type ldapAuth struct {
        cfg          config.LDAPConfig
        sessions     *SessionStore
        secureCookie bool
        logger       *slog.Logger
    }
    ```
    Determine `secureCookie`: add `SecureCookie bool` field to `config.AuthConfig`
    (shared by all auth types). In `NewLDAP()`: `secureCookie: cfg.SecureCookie`
    (where `cfg` is the parent `AuthConfig`). This requires updating the `NewLDAP`
    signature to accept the full `AuthConfig` or the `SecureCookie` flag separately.
    Preferred: `NewLDAP(cfg config.AuthConfig, logger *slog.Logger)` — consistent
    with how OIDC accesses `cfg.OIDC` and could access `cfg.SecureCookie`.
    Also update `NewOIDC` to use this shared flag instead of deriving from `RedirectURL`.

    `Middleware()` — check session cookie (same pattern as OIDC `oidcAuth.Middleware()`),
    use shared `sessionCookieName` from `session.go`.
    `Routes()` — return `chi.Router` with GET/POST `/login`, GET `/logout`, GET `/userinfo`.
    `Stop()` — call `sessions.Stop()`.

- [x] **2.4 Implement LDAP connect function**
  - **Dependencies**: 2.1
  - **Description**: Implement `connect()` method that dials the LDAP server. Handle
    `ldaps://` (TLS from start) and `ldap://` with optional StartTLS. Support
    `InsecureSkipVerify` for dev environments. Add dial timeout to prevent hanging
    on unreachable servers. Return `*ldap.Conn`.
  - **Modifies**:
    - `internal/auth/ldap.go`
  - **Details**:
    Three connection scenarios:
    1. **`ldaps://`** — TLS from start, pass `tlsCfg` to `DialURL`
    2. **`ldap://` + StartTLS** — plain dial, then upgrade via `conn.StartTLS(tlsCfg)`
    3. **`ldap://` without StartTLS** — plain dial, no TLS config needed

    ```go
    const ldapDialTimeout = 10 * time.Second

    func (a *ldapAuth) connect() (*ldap.Conn, error) {
        tlsCfg := &tls.Config{
            InsecureSkipVerify: a.cfg.InsecureSkipVerify,
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
    ```

- [x] **2.5 Implement authenticate method**
  - **Dependencies**: 2.4
  - **Description**: Core authentication logic — search bind or direct bind mode,
    user attribute extraction, optional group membership check.
  - **Modifies**:
    - `internal/auth/ldap.go`
  - **Details**:
    Template rendering for `userFilter`: replace `{{.Username}}` with
    `ldap.EscapeFilter(username)` to prevent LDAP injection.

    **Search bind** (when `bindDN` is set):
    1. Bind as service account (`bindDN` / `bindPassword`)
    2. Search for user: base=`baseDN`, filter=rendered `userFilter`,
       attrs=`[displayName, email]` (from config)
    3. Verify exactly 1 result found
    4. Re-bind as found `userDN` with user's password

    **Direct bind** (when `bindDN` is empty):
    1. Render filter, strip outer parentheses, construct
       `userDN` = `<stripped-filter>,<baseDN>` (e.g. `uid=john,ou=people,dc=example,dc=com`)
    2. Bind as user with `userDN` and password
    3. Search for own attributes (bind as self, search own DN)

    **Limitation**: Direct bind only works with simple single-attribute filters
    like `(uid={{.Username}})`. Compound filters (e.g. `(&(uid={{.Username}})(objectClass=person))`)
    are not supported in direct bind mode because DN cannot be extracted from them.
    Validate in `NewLDAP()`: if `bindDN` is empty and `userFilter` contains `&` or `|`,
    return a config error.

    **Group check** (when `allowedGroups` is non-empty):
    1. Re-bind as service account (if `bindDN` is set) or use user's bind
    2. Render `groupFilter` replacing `{{.UserDN}}` with `ldap.EscapeFilter(userDN)`
       (**not** `EscapeDN` — the value is inside an LDAP filter, not a DN context)
    3. Search under `groupBaseDN`
    4. Check if any returned group DN is in `allowedGroups`
    5. Return error if no match

    **Note**: In direct bind mode (no service account), group check relies on the
    user having LDAP read access to `groupBaseDN`. This depends on the LDAP server's
    ACL configuration. Document this limitation in Phase 4 docs.

    Return `*UserInfo{Subject: userDN, Name: displayName, Email: email}`.

- [x] **2.6 Implement HTTP handlers (login, logout, userinfo)**
  - **Dependencies**: 2.5
  - **Description**: Implement route handlers for LDAP auth flow.
  - **Modifies**:
    - `internal/auth/ldap.go`
  - **Details**:
    **GET /login**: Render login template with empty error. For now, use a minimal
    inline HTML string (template file comes in Phase 3).

    **POST /login**:
    1. Parse form: `username`, `password`
    2. Validate non-empty
    3. Call `authenticate(username, password)`
    4. On success: create session, set cookie, redirect to `/`
    5. On failure: re-render login template with error message

    Cookie settings (same as OIDC):
    - Name: `sessionCookieName` (from `session.go`)
    - HttpOnly: true
    - Secure: `a.secureCookie` (from `config.AuthConfig.SecureCookie`, see 2.3)
    - SameSite: Lax
    - MaxAge: `sessionTTL` (from `session.go`)

    **GET /logout**: Delete session, clear cookie, redirect to `/`.
    **GET /userinfo**: Return session user as JSON (same as OIDC).

- [x] **2.7 Unit tests with mock LDAP server**
  - **Dependencies**: 2.6
  - **Description**: Create `internal/auth/ldap_test.go` with tests using `gldap` mock server.
  - **Creates**:
    - `internal/auth/ldap_test.go`
  - **Details**:
    Mock LDAP server setup: create `gldap.Server` with bind handler and search handler.
    Pre-populate with test users and groups.

    Test cases:
    | Test | Description |
    |------|-------------|
    | `TestNewLDAP` | Constructor with valid config |
    | `TestNewLDAP_DirectBindCompoundFilter` | Constructor rejects compound filter without bindDN |
    | `TestLDAP_SearchBind_Success` | Service account search + user bind OK |
    | `TestLDAP_SearchBind_WrongPassword` | User bind fails |
    | `TestLDAP_SearchBind_UserNotFound` | Search returns 0 entries |
    | `TestLDAP_SearchBind_MultipleResults` | Search returns >1 entry → error |
    | `TestLDAP_DirectBind_Success` | Direct bind without service account |
    | `TestLDAP_DirectBind_WrongPassword` | Direct bind fails |
    | `TestLDAP_GroupCheck_Allowed` | User in allowed group |
    | `TestLDAP_GroupCheck_Denied` | User not in allowed group |
    | `TestLDAP_GroupCheck_Disabled` | Empty allowedGroups — no check |
    | `TestLDAP_ConnectTimeout` | Dial to unreachable host returns error within timeout |
    | `TestLDAP_LoginFlow` | GET /login → POST /login → cookie set → redirect |
    | `TestLDAP_LoginWrongCreds` | POST /login with bad password → re-render form |
    | `TestLDAP_Logout` | Session deleted, cookie cleared |
    | `TestLDAP_UserInfo` | Returns user data from session |
    | `TestLDAP_UserInfoUnauthorized` | No session → 401 |
    | `TestLDAP_MiddlewareValidSession` | Request with valid cookie passes |
    | `TestLDAP_MiddlewareNoSession` | Request without cookie → 401 |
    | `TestLDAP_FilterEscaping` | Username with special chars is escaped |
    | `TestLDAP_Factory` | Update Phase 1 `TestFactoryLDAP` in `basic_test.go` — expect success (no error) now that `NewLDAP` returns a real authenticator |
  - **Links**:
    - [go-ldap/ldap](https://github.com/go-ldap/ldap)
    - [jimlambrt/gldap](https://github.com/jimlambrt/gldap)

### Completion Criteria Phase 2

- [x] All items completed (2.1–2.7)
- [x] `go build ./...` succeeds
- [x] `go test ./internal/auth/...` passes (all LDAP tests green)
- [x] LDAP injection is prevented via `ldap.EscapeFilter()`
- [x] Both search bind and direct bind modes work

---

## Phase 3: Login Template and Rate Limiting

**Dependencies**: Phase 2
**Status**: Done

### Description

Create the HTML login form template file, embed it via `//go:embed`, and implement
per-IP rate limiting on POST /login. Build Docker image, deploy to test cluster,
verify against a real LDAP server (OpenLDAP in test infra).

### Items

- [x] **3.1 Create login.html template**
  - **Dependencies**: None
  - **Description**: Create `internal/auth/templates/login.html` — an `html/template`
    file with a login form. The form POSTs to `/auth/login` with fields `username`
    and `password`. Includes a hidden CSRF token field. Displays `{{.Error}}` when
    authentication fails. CSS styles support dark/light theme via `prefers-color-scheme`.
    Visual style matches the main application (similar fonts, colors).
  - **Creates**:
    - `internal/auth/templates/login.html`
  - **Details**:
    Template data struct:
    ```go
    type loginPageData struct {
        Error     string
        CSRFToken string
        Username  string // preserve entered username on auth failure
    }
    ```
    Template features:
    - Responsive, centered form
    - Application title "dephealth-ui" at top
    - Username field (autofocus, `value="{{.Username}}"` — preserves input on error)
    - Password field
    - Submit button
    - Error message area (red text, shown when `.Error` is non-empty)
    - Dark/light theme via `@media (prefers-color-scheme: dark)`
    - Hidden CSRF token: `<input type="hidden" name="_csrf" value="{{.CSRFToken}}">`
    - No external dependencies (no CDN links)

- [x] **3.2 Embed template and wire into ldapAuth**
  - **Dependencies**: 3.1
  - **Description**: Add `//go:embed templates/login.html` in `ldap.go`. Add
    `loginTmpl *template.Template` field to `ldapAuth` struct (was deferred from Phase 2).
    Parse template in `NewLDAP()`. Update `handleLogin` GET to render template.
    Update `handleLoginPost` to render template with error on failure, passing back
    `Username` to preserve the entered value.
  - **Modifies**:
    - `internal/auth/ldap.go`
  - **Details**:
    ```go
    //go:embed templates/login.html
    var loginTemplateHTML string
    ```
    Add field to `ldapAuth`:
    ```go
    loginTmpl *template.Template
    ```
    In `NewLDAP()`:
    ```go
    tmpl, err := template.New("login").Parse(loginTemplateHTML)
    ```
    In `handleLoginPost` on failure:
    ```go
    a.loginTmpl.Execute(w, loginPageData{
        Error:     "Invalid username or password",
        CSRFToken: newToken,
        Username:  username,
    })
    ```

- [x] **3.3 Implement CSRF token protection**
  - **Dependencies**: 3.2
  - **Description**: Generate a random CSRF token per login page render. Store it in
    a short-lived map (similar to OIDC states). Validate on POST /login before
    processing credentials. Limit max stored tokens to prevent memory exhaustion
    from mass GET /login requests.
  - **Modifies**:
    - `internal/auth/ldap.go`
  - **Details**:
    Add to `ldapAuth`:
    ```go
    csrfTokens   map[string]time.Time  // token → expiry
    csrfMu       sync.Mutex
    ```
    Token TTL: 10 minutes. Cleanup expired tokens on each new token generation.
    Max tokens cap: 10000 — if exceeded after cleanup, reject with 503
    (Service Unavailable) to prevent memory exhaustion from automated GET /login spam.

- [x] **3.4 Implement per-IP rate limiter**
  - **Dependencies**: None
  - **Description**: Implement a simple in-memory rate limiter that tracks login attempts
    per client IP address. Hardcoded: 5 attempts per 1-minute sliding window. Returns
    HTTP 429 when exceeded. Place in a separate file `internal/auth/ratelimit.go` since
    it is a generic component reusable by any form-based auth.
    Add `limiter *rateLimiter` field to `ldapAuth` struct (was deferred from Phase 2).
  - **Creates**:
    - `internal/auth/ratelimit.go`
  - **Modifies**:
    - `internal/auth/ldap.go` — add `limiter` field, integrate into `handleLoginPost`
  - **Details**:
    ```go
    type rateLimiter struct {
        mu          sync.Mutex
        attempts    map[string][]time.Time
        window      time.Duration  // 1 minute
        max         int            // 5
        lastCleanup time.Time
    }

    func newRateLimiter(window time.Duration, max int) *rateLimiter
    func (rl *rateLimiter) Allow(ip string) bool
    ```

    **IP extraction**: `r.RemoteAddr` is unreliable behind reverse proxy (returns
    proxy IP, not client IP). Add helper function `clientIP(r *http.Request) string`:
    1. Check `X-Forwarded-For` header — take the first (leftmost) IP
    2. Fall back to `X-Real-IP` header
    3. Fall back to `r.RemoteAddr`
    4. Strip port via `net.SplitHostPort` in all cases

    Integrate into `handleLoginPost`: check `limiter.Allow(clientIP(r))` before
    processing credentials. On 429, render login template with rate limit error message.

    **Stale entry cleanup**: Instead of cleaning on every `Allow()` call, throttle
    cleanup to run at most once per `window` duration (track `lastCleanup` timestamp).
    This prevents both excessive cleanup overhead and unbounded memory growth.

- [x] **3.5 Rate limiter and CSRF tests**
  - **Dependencies**: 3.3, 3.4
  - **Description**: Add tests for rate limiter and CSRF validation.
  - **Modifies**:
    - `internal/auth/ldap_test.go`
  - **Creates**:
    - `internal/auth/ratelimit_test.go` — rate limiter unit tests (separate file)
  - **Details**:
    Rate limiter tests (in `ratelimit_test.go`):
    | Test | Description |
    |------|-------------|
    | `TestRateLimiter_AllowUnderLimit` | 5 requests within window → all allowed |
    | `TestRateLimiter_BlockOverLimit` | 6th request within window → blocked |
    | `TestRateLimiter_WindowExpiry` | After window passes, requests allowed again |
    | `TestRateLimiter_DifferentIPs` | Each IP has independent counter |
    | `TestRateLimiter_CleanupThrottle` | Stale entries cleaned only once per window |
    | `TestClientIP_XForwardedFor` | Extracts first IP from `X-Forwarded-For` |
    | `TestClientIP_XRealIP` | Falls back to `X-Real-IP` |
    | `TestClientIP_RemoteAddr` | Falls back to `r.RemoteAddr`, strips port |

    CSRF and integration tests (in `ldap_test.go`):
    | Test | Description |
    |------|-------------|
    | `TestLDAP_CSRF_ValidToken` | POST with correct token → processed |
    | `TestLDAP_CSRF_InvalidToken` | POST with wrong token → 403 |
    | `TestLDAP_CSRF_MissingToken` | POST without token → 403 |
    | `TestLDAP_CSRF_MaxTokensCap` | After 10000 tokens, GET /login returns 503 |
    | `TestLDAP_RateLimit_POST` | 6 POSTs → 429 on 6th |
    | `TestLDAP_LoginPreservesUsername` | Failed login re-renders form with username filled |

- [ ] **3.6 Build and test in cluster**
  - **Dependencies**: 3.5
  - **Description**: Build Docker dev image, deploy to test Kubernetes cluster. Deploy
    OpenLDAP as a test dependency in `deploy/helm/dephealth-infra/` chart (add OpenLDAP
    templates alongside existing PostgreSQL/Redis). Verify end-to-end login flow in browser.
  - **Modifies**:
    - `deploy/helm/dephealth-infra/` — add OpenLDAP deployment, service, configmap with seed data
  - **Details**:
    Steps:
    1. `make docker-build TAG=v<next>-1` — build dev image
    2. `docker push` to Harbor
    3. Add OpenLDAP to `dephealth-infra` chart with test users:
       - `uid=testuser,ou=people,dc=example,dc=com` (password: `testpass`)
       - `uid=admin,ou=people,dc=example,dc=com` (password: `adminpass`)
       - Group `cn=dephealth-users,ou=groups,dc=example,dc=com` containing `testuser`
    4. Update dephealth-ui deployment config to use `auth.type: ldap`
    5. Verify in browser:
       - Unauthenticated request → redirect to login form
       - Wrong credentials → error message on form, username preserved in field
       - Correct credentials → redirect to main app, user info shown
       - Logout → session cleared, redirect to login
       - Rate limiting: rapid login attempts → 429
       - CSRF: direct POST to `/auth/login` without token → 403

### Completion Criteria Phase 3

- [ ] All items completed (3.1–3.6)
- [ ] `go test ./internal/auth/...` passes (rate limiter + CSRF tests)
- [ ] Login form renders correctly in browser (dark and light theme)
- [ ] CSRF protection works
- [ ] Rate limiting blocks brute-force attempts
- [ ] End-to-end login flow verified in test cluster

---

## Phase 4: Frontend, Helm Chart and Documentation

**Dependencies**: Phase 3
**Status**: Pending

### Description

Update frontend to show user info for LDAP auth type. Update Helm chart values and
templates for LDAP configuration. Update project documentation.

### Items

- [x] **4.1 Frontend: support LDAP auth type**
  - **Dependencies**: None
  - **Description**: Update `frontend/src/main.js` to show user info panel for LDAP
    auth type (same as OIDC). Single line change in the `init()` function where
    `config.auth.type === 'oidc'` is checked. Use `includes()` for extensibility.
  - **Modifies**:
    - `frontend/src/main.js` — search for `config.auth.type === 'oidc'` in `init()`
  - **Details**:
    ```javascript
    // Before:
    if (config.auth && config.auth.type === 'oidc') {
    // After:
    if (config.auth && ['oidc', 'ldap'].includes(config.auth.type)) {
    ```

- [x] **4.2 Helm chart: add LDAP config values**
  - **Dependencies**: None
  - **Description**: Add LDAP configuration section to `deploy/helm/dephealth-ui/values.yaml`.
    Add `secureCookie` field (shared by all auth types, introduced in Phase 2).
    Add `ldapSecret` section for sensitive fields (bindPassword) from Kubernetes Secret.
    Update `deployment.yml` to inject LDAP secret env vars — extend the existing
    `if or` condition to include `ldapSecret.enabled`.
  - **Modifies**:
    - `deploy/helm/dephealth-ui/values.yaml`
    - `deploy/helm/dephealth-ui/templates/deployment.yml`
  - **Details**:
    Add to `values.yaml` under `config.auth`:
    ```yaml
    auth:
      type: "none"
      secureCookie: false  # set to true when app is behind HTTPS
      ldap:
        url: ""
        startTLS: false
        insecureSkipVerify: false
        bindDN: ""
        # bindPassword: set via ldapSecret or env
        baseDN: ""
        userFilter: "(uid={{.Username}})"
        attributes:
          displayName: "displayName"
          email: "mail"
        groupBaseDN: ""
        groupFilter: ""
        allowedGroups: []
    ```
    Add top-level secret config:
    ```yaml
    ldapSecret:
      enabled: false
      secretName: ""
      bindPasswordKey: "bindPassword"
    ```
    In `deployment.yml`, update the existing `if or` condition to include `ldapSecret`:
    ```yaml
    # Before:
    {{- if or .Values.customCA.enabled .Values.grafanaSecret.enabled }}
    # After:
    {{- if or .Values.customCA.enabled .Values.grafanaSecret.enabled .Values.ldapSecret.enabled }}
    ```
    Add LDAP env var inside the env block:
    ```yaml
    {{- if .Values.ldapSecret.enabled }}
    - name: DEPHEALTH_AUTH_LDAP_BIND_PASSWORD
      valueFrom:
        secretKeyRef:
          name: {{ .Values.ldapSecret.secretName }}
          key: {{ .Values.ldapSecret.bindPasswordKey }}
    {{- end }}
    ```
    Note: `customCA` already exists and works for LDAP TLS too (SSL_CERT_FILE is
    process-wide), so no additional CA handling needed.

- [x] **4.3 Update project documentation**
  - **Dependencies**: 4.1, 4.2
  - **Description**: Document LDAP authentication configuration. Update existing
    auth documentation files (not `docs/README.md` — auth docs are elsewhere).
    Add LDAP section with config examples for common scenarios: simple LDAP,
    LDAP with groups, LDAPS. Document known limitations from Phase 2.
  - **Modifies**:
    - `docs/API.md` — add `ldap` to the Authentication section (alongside `none`, `basic`, `oidc`)
    - `deploy/helm/dephealth-ui/README.md` — add LDAP Helm config example (alongside existing OIDC example)
  - **Details**:
    Documentation must cover:
    1. **Config examples**: simple LDAP (direct bind), LDAP with service account (search bind),
       LDAPS, LDAP with group restrictions
    2. **Helm example**: values.yaml with `ldapSecret` for bind password
    3. **Known limitations** (from Phase 2 review):
       - Direct bind mode only supports simple single-attribute filters
         (compound filters like `(&(uid=...)(objectClass=...))` are not supported)
       - Group check in direct bind mode requires the user to have LDAP read access
         to `groupBaseDN` (depends on LDAP server ACL configuration)
    4. **secureCookie field**: explain when to set `true` (behind HTTPS terminating proxy)
    5. **customCA**: note that existing `customCA` works for LDAP TLS verification too

- [ ] **4.4 Verify Helm deployment with LDAP**
  - **Dependencies**: 4.2
  - **Description**: Deploy using Helm chart with LDAP auth configured. Verify that
    ConfigMap contains correct LDAP config, Secret env vars are injected, and
    authentication works end-to-end.
  - **Details**:
    Steps:
    1. Create test values file with LDAP config pointing to OpenLDAP from Phase 3
    2. **Dry-run validation**: `helm template dephealth-ui deploy/helm/dephealth-ui/ -f test-values.yaml`
       — verify rendered YAML: ConfigMap has LDAP config, deployment has env block
       with `DEPHEALTH_AUTH_LDAP_BIND_PASSWORD` secretKeyRef, `if or` condition
       correctly includes `ldapSecret.enabled`
    3. `helm upgrade --install dephealth-ui deploy/helm/dephealth-ui/ -f test-values.yaml`
    4. Verify ConfigMap contents: `kubectl get cm dephealth-ui-config -o yaml`
    5. Verify env injection: `kubectl describe pod <pod>` — check env vars
    6. Verify login flow in browser
    7. Clean up test deployment

### Completion Criteria Phase 4

- [ ] All items completed (4.1–4.4)
- [ ] Frontend shows user info for LDAP auth
- [ ] Helm chart renders correct ConfigMap and injects Secret env vars
- [ ] `helm template` dry-run produces valid YAML with LDAP config
- [ ] Documentation covers LDAP configuration with examples and limitations
- [ ] Full end-to-end flow verified via Helm deployment
- [ ] `make lint` passes (frontend JS + markdown files)

---

## Notes

- **go-ldap/ldap v3** — the standard Go LDAP client. Supports bind, search, StartTLS,
  connection timeouts. No connection pooling (new connection per auth request, acceptable
  for this use case).
- **jimlambrt/gldap** — lightweight mock LDAP server for unit tests. Allows defining
  custom bind/search handlers without running a real LDAP server.
- **CSRF** — the login form includes a CSRF token to prevent cross-site form submission.
  Token is stored in-memory with 10-minute TTL (similar to OIDC state entries).
- **Session reuse** — `SessionStore` and `UserInfo` from `session.go` are shared between
  OIDC and LDAP authenticators. No changes needed to session management.
- **customCA** — the existing `customCA` Helm value (sets `SSL_CERT_FILE`) works for
  LDAP TLS verification too, since Go's TLS uses the system cert pool.
