package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type memoryAuthRepository struct {
	mu       sync.Mutex
	nextUser int64
	nextSess int64
	users    map[int64]User
	emails   map[string]int64
	sessions map[int64]Session
	events   []SecurityEvent
}

func newMemoryAuthRepository() *memoryAuthRepository {
	return &memoryAuthRepository{users: map[int64]User{}, emails: map[string]int64{}, sessions: map[int64]Session{}}
}

func (r *memoryAuthRepository) CreateUser(_ context.Context, user User) (User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, found := r.emails[user.Email]; found {
		return User{}, ErrDuplicateEmail
	}
	r.nextUser++
	now := time.Now().UTC()
	user.ID, user.CreatedAt, user.UpdatedAt = r.nextUser, now, now
	r.users[user.ID], r.emails[user.Email] = user, user.ID
	return user, nil
}

func (r *memoryAuthRepository) FindUserByEmail(_ context.Context, email string) (User, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, found := r.emails[email]
	return r.users[id], found, nil
}

func (r *memoryAuthRepository) FindUserByID(_ context.Context, id int64) (User, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	user, found := r.users[id]
	return user, found, nil
}

func (r *memoryAuthRepository) UpdateUserPassword(_ context.Context, id int64, hash string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	user := r.users[id]
	user.PasswordHash = hash
	r.users[id] = user
	return nil
}

func (r *memoryAuthRepository) MarkUserLogin(_ context.Context, id int64, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	user := r.users[id]
	user.LastLoginAt = &at
	r.users[id] = user
	return nil
}

func (r *memoryAuthRepository) CreateSession(_ context.Context, session Session) (Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextSess++
	session.ID = r.nextSess
	r.sessions[session.ID] = session
	return session, nil
}

func (r *memoryAuthRepository) FindSessionByTokenHash(_ context.Context, hash string) (Session, User, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, session := range r.sessions {
		if session.TokenHash == hash {
			return session, r.users[session.UserID], true, nil
		}
	}
	return Session{}, User{}, false, nil
}

func (r *memoryAuthRepository) RefreshSession(_ context.Context, id int64, csrf string, seen, idle time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	session := r.sessions[id]
	session.CSRFHash, session.LastSeenAt, session.IdleExpiresAt = csrf, seen, idle
	r.sessions[id] = session
	return nil
}

func (r *memoryAuthRepository) RevokeSession(_ context.Context, id int64, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	session := r.sessions[id]
	if session.ID != 0 && session.RevokedAt == nil {
		session.RevokedAt = &at
		r.sessions[id] = session
	}
	return nil
}

func (r *memoryAuthRepository) RevokeOtherSessions(_ context.Context, userID, keepID int64, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, session := range r.sessions {
		if session.UserID == userID && id != keepID && session.RevokedAt == nil {
			session.RevokedAt = &at
			r.sessions[id] = session
		}
	}
	return nil
}

func (r *memoryAuthRepository) RecordSecurityEvent(_ context.Context, event SecurityEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
	return nil
}
func (r *memoryAuthRepository) AssignLegacySquads(context.Context, int64) error { return nil }

func newTestAuth(t *testing.T, registration bool) (*AuthService, *memoryAuthRepository) {
	t.Helper()
	repository := newMemoryAuthRepository()
	service, err := NewAuthService(repository, AuthRuntimeConfig{RegistrationEnabled: registration, AllowedOrigin: "http://localhost:5173", IdleTimeout: time.Hour, AbsoluteTimeout: 24 * time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	return service, repository
}

func TestArgon2idPasswordPolicyAndUpgrade(t *testing.T) {
	hasher := DefaultPasswordHasher()
	if _, err := hasher.Hash("short"); err == nil {
		t.Fatal("expected password policy failure")
	}
	hash, err := hasher.Hash("Correct-horse-battery-42")
	if err != nil {
		t.Fatal(err)
	}
	if valid, upgrade := hasher.Verify(hash, "Correct-horse-battery-42"); !valid || upgrade {
		t.Fatalf("valid=%v upgrade=%v", valid, upgrade)
	}
	if valid, _ := hasher.Verify(hash, "wrong-password-value"); valid {
		t.Fatal("wrong password verified")
	}
	older := hasher
	older.Iterations = 2
	oldHash, err := older.Hash("Correct-horse-battery-42")
	if err != nil {
		t.Fatal(err)
	}
	if valid, upgrade := hasher.Verify(oldHash, "Correct-horse-battery-42"); !valid || !upgrade {
		t.Fatalf("expected hash upgrade: valid=%v upgrade=%v", valid, upgrade)
	}
}

func TestAuthServiceSessionLifecycleAndIsolation(t *testing.T) {
	service, repository := newTestAuth(t, true)
	registered, err := service.Register(context.Background(), " User@Example.COM ", "Correct-horse-battery-42", "User", "127.0.0.1", "req-register")
	if err != nil {
		t.Fatal(err)
	}
	if registered.User.Email != "user@example.com" || registered.Token == "" || registered.CSRFToken == "" {
		t.Fatalf("unsafe registration result: %#v", registered)
	}
	if registered.User.PasswordHash != "" {
		t.Fatal("password hash was exposed")
	}
	if _, err := service.Register(context.Background(), "user@example.com", "Another-secure-password-42", "Duplicate", "127.0.0.1", "req-duplicate"); !errors.Is(err, ErrDuplicateEmail) {
		t.Fatalf("duplicate error=%v", err)
	}
	authenticated, err := service.Authenticate(context.Background(), registered.Token)
	if err != nil || authenticated.User.ID != registered.User.ID {
		t.Fatalf("authenticate=%#v err=%v", authenticated, err)
	}
	second, err := service.Login(context.Background(), "user@example.com", "Correct-horse-battery-42", "127.0.0.1", "req-login")
	if err != nil {
		t.Fatal(err)
	}
	changed, err := service.ChangePassword(context.Background(), authenticated, "Correct-horse-battery-42", "New-correct-horse-credential-43", "127.0.0.1", "req-password")
	if err != nil || changed.Token == registered.Token {
		t.Fatalf("change=%#v err=%v", changed, err)
	}
	if _, err := service.Authenticate(context.Background(), registered.Token); !errors.Is(err, ErrSessionInvalid) {
		t.Fatalf("old current token error=%v", err)
	}
	if _, err := service.Authenticate(context.Background(), second.Token); !errors.Is(err, ErrSessionInvalid) {
		t.Fatalf("other token error=%v", err)
	}
	if len(repository.events) < 3 {
		t.Fatalf("security events=%#v", repository.events)
	}
}

func TestCredentialRateLimitUsesGenericFailure(t *testing.T) {
	service, _ := newTestAuth(t, true)
	for attempt := 0; attempt < 5; attempt++ {
		if _, err := service.Login(context.Background(), "missing@example.com", "Wrong-password-value-42", "127.0.0.1", "req"); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("attempt %d error=%v", attempt, err)
		}
	}
	if _, err := service.Login(context.Background(), "missing@example.com", "Wrong-password-value-42", "127.0.0.1", "req"); err == nil {
		t.Fatal("expected rate limit")
	} else {
		var limited RateLimitError
		if !errors.As(err, &limited) || limited.RetryAfter <= 0 {
			t.Fatalf("rate error=%v", err)
		}
	}
}

func TestConcurrentSessionChecksKeepOneSessionBoundCSRFToken(t *testing.T) {
	service, _ := newTestAuth(t, true)
	registered, err := service.Register(context.Background(), "tabs@example.com", "Correct-horse-battery-42", "Tabs", "127.0.0.1", "req-register")
	if err != nil {
		t.Fatal(err)
	}
	first, err := service.Authenticate(context.Background(), registered.Token)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Authenticate(context.Background(), registered.Token)
	if err != nil {
		t.Fatal(err)
	}
	first, err = service.RefreshCSRF(context.Background(), first)
	if err != nil {
		t.Fatal(err)
	}
	second, err = service.RefreshCSRF(context.Background(), second)
	if err != nil {
		t.Fatal(err)
	}
	if first.CSRFToken == "" || first.CSRFToken != second.CSRFToken || !service.VerifyCSRF(second.SessionID, first.CSRFToken, second.CSRFHash) {
		t.Fatalf("session-bound CSRF tokens diverged: first=%q second=%q", first.CSRFToken, second.CSRFToken)
	}
}

func TestAuthAPICookiesCSRFRateLimitsAndDisabledRegistration(t *testing.T) {
	service, repository := newTestAuth(t, true)
	api := NewAPI(NewStore(), nil, nil, nil)
	api.EnableAuth(service)
	handler := api.Handler()

	register := authRequest(handler, http.MethodPost, "/api/v1/auth/register", `{"email":"api@example.com","password":"Correct-horse-battery-42","displayName":"API User"}`, "", "")
	if register.Code != http.StatusCreated {
		t.Fatalf("register=%d %s", register.Code, register.Body.String())
	}
	cookies := register.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteLaxMode || cookies[0].Value == "" {
		t.Fatalf("cookie=%#v", cookies)
	}
	if strings.Contains(register.Body.String(), "Correct-horse") || strings.Contains(register.Body.String(), cookies[0].Value) || strings.Contains(register.Body.String(), "passwordHash") {
		t.Fatalf("secret leaked: %s", register.Body.String())
	}
	if register.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("auth response cache policy=%q", register.Header().Get("Cache-Control"))
	}
	invalidEmail := authRequest(handler, http.MethodPost, "/api/v1/auth/register", `{"email":"not-an-email","password":"Correct-horse-battery-42"}`, "", "")
	if invalidEmail.Code != http.StatusUnprocessableEntity || !strings.Contains(invalidEmail.Body.String(), `"code":"invalid_email"`) {
		t.Fatalf("invalid email=%d %s", invalidEmail.Code, invalidEmail.Body.String())
	}

	me := authRequest(handler, http.MethodGet, "/api/v1/auth/me", "", cookies[0].Value, "")
	if me.Code != http.StatusOK {
		t.Fatalf("me=%d %s", me.Code, me.Body.String())
	}
	var meBody struct {
		Data AuthSessionResult `json:"data"`
	}
	if err := json.NewDecoder(me.Body).Decode(&meBody); err != nil {
		t.Fatal(err)
	}

	// authRequest supplies the valid origin; an explicit cross-site request must fail.
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", bytes.NewBufferString(`{}`))
	request.Header.Set("Origin", "https://attacker.example")
	request.Header.Set("X-CSRF-Token", meBody.Data.CSRFToken)
	request.AddCookie(cookies[0])
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden || !strings.Contains(recorder.Body.String(), "csrf_failed") {
		t.Fatalf("cross-site=%d %s", recorder.Code, recorder.Body.String())
	}

	logout := authRequest(handler, http.MethodPost, "/api/v1/auth/logout", `{}`, cookies[0].Value, meBody.Data.CSRFToken)
	if logout.Code != http.StatusOK {
		t.Fatalf("logout=%d %s", logout.Code, logout.Body.String())
	}
	if len(repository.events) == 0 {
		t.Fatal("security event was not recorded")
	}

	disabled, _ := newTestAuth(t, false)
	disabledAPI := NewAPI(NewStore(), nil, nil, nil)
	disabledAPI.EnableAuth(disabled)
	response := authRequest(disabledAPI.Handler(), http.MethodPost, "/api/v1/auth/register", `{"email":"disabled@example.com","password":"Correct-horse-battery-42"}`, "", "")
	if response.Code != http.StatusForbidden || !strings.Contains(response.Body.String(), "registration_disabled") {
		t.Fatalf("disabled=%d %s", response.Code, response.Body.String())
	}
}

func TestAuthAPISecureCookieExpiryAndLoginRotation(t *testing.T) {
	service, _ := newTestAuth(t, true)
	service.Config.CookieSecure = true
	now := time.Date(2026, time.August, 9, 12, 0, 0, 0, time.UTC)
	service.Now = func() time.Time { return now }
	api := NewAPI(NewStore(), nil, nil, nil)
	api.EnableAuth(service)
	handler := api.Handler()

	register := authRequestWithCookie(handler, http.MethodPost, "/api/v1/auth/register", `{"email":"rotation@example.com","password":"Correct-horse-battery-42"}`, nil, "")
	if register.Code != http.StatusCreated {
		t.Fatalf("register=%d %s", register.Code, register.Body.String())
	}
	registeredCookie := register.Result().Cookies()[0]
	if registeredCookie.Name != "__Host-fh_session" || !registeredCookie.Secure || !registeredCookie.HttpOnly || registeredCookie.Path != "/" || registeredCookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("secure cookie=%#v", registeredCookie)
	}

	login := authRequestWithCookie(handler, http.MethodPost, "/api/v1/auth/login", `{"email":"rotation@example.com","password":"Correct-horse-battery-42"}`, registeredCookie, "")
	if login.Code != http.StatusOK {
		t.Fatalf("login=%d %s", login.Code, login.Body.String())
	}
	if stale := authRequestWithCookie(handler, http.MethodGet, "/api/v1/auth/me", "", registeredCookie, ""); stale.Code != http.StatusUnauthorized {
		t.Fatalf("pre-login session remained valid: %d %s", stale.Code, stale.Body.String())
	}

	currentCookie := login.Result().Cookies()[0]
	now = now.Add(25 * time.Hour)
	expired := authRequestWithCookie(handler, http.MethodGet, "/api/v1/auth/me", "", currentCookie, "")
	if expired.Code != http.StatusUnauthorized || !strings.Contains(expired.Body.String(), "authentication_required") {
		t.Fatalf("expired=%d %s", expired.Code, expired.Body.String())
	}
	cleared := expired.Result().Cookies()
	if len(cleared) != 1 || cleared[0].Name != "__Host-fh_session" || cleared[0].MaxAge != -1 {
		t.Fatalf("expired cookie was not cleared: %#v", cleared)
	}
}

func TestAuthAPIRateLimitUsesCommonEnvelope(t *testing.T) {
	service, _ := newTestAuth(t, true)
	api := NewAPI(NewStore(), nil, nil, nil)
	api.EnableAuth(service)
	handler := api.Handler()

	for attempt := 0; attempt < 5; attempt++ {
		response := authRequest(handler, http.MethodPost, "/api/v1/auth/login", `{"email":"missing@example.com","password":"Wrong-password-value-42"}`, "", "")
		if response.Code != http.StatusUnauthorized || !strings.Contains(response.Body.String(), `"code":"invalid_credentials"`) {
			t.Fatalf("attempt %d=%d %s", attempt+1, response.Code, response.Body.String())
		}
	}
	limited := authRequest(handler, http.MethodPost, "/api/v1/auth/login", `{"email":"missing@example.com","password":"Wrong-password-value-42"}`, "", "")
	if limited.Code != http.StatusTooManyRequests || limited.Header().Get("Retry-After") == "" || !strings.Contains(limited.Body.String(), `"code":"auth_rate_limited"`) {
		t.Fatalf("limited=%d retry=%q %s", limited.Code, limited.Header().Get("Retry-After"), limited.Body.String())
	}
}

func authRequest(handler http.Handler, method, path, body, token, csrf string) *httptest.ResponseRecorder {
	var cookie *http.Cookie
	if token != "" {
		cookie = &http.Cookie{Name: "fh_session", Value: token}
	}
	return authRequestWithCookie(handler, method, path, body, cookie, csrf)
}

func authRequestWithCookie(handler http.Handler, method, path, body string, cookie *http.Cookie, csrf string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "http://localhost:5173")
	if cookie != nil {
		request.AddCookie(cookie)
	}
	if csrf != "" {
		request.Header.Set("X-CSRF-Token", csrf)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}
