package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"sync"
	"time"
	"unicode"

	"golang.org/x/crypto/argon2"
)

const (
	defaultArgonMemory      = 64 * 1024
	defaultArgonIterations  = 3
	defaultArgonParallelism = 1
	defaultArgonSaltLength  = 16
	defaultArgonKeyLength   = 32
)

var (
	ErrInvalidCredentials   = errors.New("invalid credentials")
	ErrDuplicateEmail       = errors.New("email is unavailable")
	ErrInvalidEmail         = errors.New("email address is invalid")
	ErrRegistrationDisabled = errors.New("registration is disabled")
	ErrSessionInvalid       = errors.New("session is invalid")
	ErrCSRFInvalid          = errors.New("csrf token is invalid")
)

type User struct {
	ID           int64      `json:"id"`
	Email        string     `json:"email"`
	DisplayName  string     `json:"displayName"`
	PasswordHash string     `json:"-"`
	Status       string     `json:"status"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
	LastLoginAt  *time.Time `json:"lastLoginAt,omitempty"`
}

type Session struct {
	ID                int64
	UserID            int64
	TokenHash         string
	CSRFHash          string
	CreatedAt         time.Time
	LastSeenAt        time.Time
	IdleExpiresAt     time.Time
	AbsoluteExpiresAt time.Time
	RevokedAt         *time.Time
	DeviceMetadata    map[string]string
}

type SecurityEvent struct {
	RequestID     string
	UserID        *int64
	EventType     string
	Outcome       string
	SourceAddress string
	OccurredAt    time.Time
}

type AuthRepository interface {
	CreateUser(context.Context, User) (User, error)
	FindUserByEmail(context.Context, string) (User, bool, error)
	FindUserByID(context.Context, int64) (User, bool, error)
	UpdateUserPassword(context.Context, int64, string) error
	MarkUserLogin(context.Context, int64, time.Time) error
	CreateSession(context.Context, Session) (Session, error)
	FindSessionByTokenHash(context.Context, string) (Session, User, bool, error)
	RefreshSession(context.Context, int64, string, time.Time, time.Time) error
	RevokeSession(context.Context, int64, time.Time) error
	RevokeOtherSessions(context.Context, int64, int64, time.Time) error
	RecordSecurityEvent(context.Context, SecurityEvent) error
	AssignLegacySquads(context.Context, int64) error
}

type PasswordPolicyError struct {
	Message string
}

func (e PasswordPolicyError) Error() string { return e.Message }

type PasswordHasher struct {
	Memory      uint32
	Iterations  uint32
	Parallelism uint8
	SaltLength  uint32
	KeyLength   uint32
}

func DefaultPasswordHasher() PasswordHasher {
	return PasswordHasher{Memory: defaultArgonMemory, Iterations: defaultArgonIterations, Parallelism: defaultArgonParallelism, SaltLength: defaultArgonSaltLength, KeyLength: defaultArgonKeyLength}
}

func ValidatePassword(password string) error {
	if len([]rune(password)) < 12 {
		return PasswordPolicyError{Message: "Password must contain at least 12 characters."}
	}
	lower := strings.ToLower(strings.TrimSpace(password))
	common := map[string]struct{}{"passwordpassword": {}, "password1234": {}, "123456789012": {}, "qwertyqwerty": {}, "letmeinletmein": {}}
	if _, found := common[lower]; found {
		return PasswordPolicyError{Message: "Choose a less common password."}
	}
	runes := []rune(password)
	allSame := true
	classes := map[string]bool{}
	for index, value := range runes {
		if index > 0 && value != runes[0] {
			allSame = false
		}
		switch {
		case unicode.IsLower(value):
			classes["lower"] = true
		case unicode.IsUpper(value):
			classes["upper"] = true
		case unicode.IsDigit(value):
			classes["digit"] = true
		default:
			classes["symbol"] = true
		}
	}
	if allSame || len(classes) < 2 {
		return PasswordPolicyError{Message: "Password must not be an obvious repeated value."}
	}
	return nil
}

func (h PasswordHasher) Hash(password string) (string, error) {
	if err := ValidatePassword(password); err != nil {
		return "", err
	}
	salt := make([]byte, h.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, h.Iterations, h.Memory, h.Parallelism, h.KeyLength)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, h.Memory, h.Iterations, h.Parallelism, base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

func (h PasswordHasher) Verify(encoded, password string) (bool, bool) {
	var version int
	var memory, iterations uint32
	var parallelism uint8
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, false
	}
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false, false
	}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil {
		return false, false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, false
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(expected) == 0 {
		return false, false
	}
	actual := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, uint32(len(expected)))
	valid := subtle.ConstantTimeCompare(actual, expected) == 1
	upgrade := valid && (memory != h.Memory || iterations != h.Iterations || parallelism != h.Parallelism || uint32(len(expected)) != h.KeyLength)
	return valid, upgrade
}

type AuthRuntimeConfig struct {
	RegistrationEnabled bool
	CookieSecure        bool
	AllowedOrigin       string
	IdleTimeout         time.Duration
	AbsoluteTimeout     time.Duration
}

type AuthSessionResult struct {
	User      User      `json:"user"`
	CSRFToken string    `json:"csrfToken"`
	ExpiresAt time.Time `json:"expiresAt"`
	Token     string    `json:"-"`
	SessionID int64     `json:"-"`
	CSRFHash  string    `json:"-"`
}

type AuthService struct {
	Repository AuthRepository
	Config     AuthRuntimeConfig
	Hasher     PasswordHasher
	Limiter    *CredentialLimiter
	Now        func() time.Time
	dummyHash  string
}

func NewAuthService(repository AuthRepository, config AuthRuntimeConfig) (*AuthService, error) {
	hasher := DefaultPasswordHasher()
	dummy, err := hasher.Hash("Dummy-password-credential-42")
	if err != nil {
		return nil, err
	}
	return &AuthService{Repository: repository, Config: config, Hasher: hasher, Limiter: NewCredentialLimiter(), Now: func() time.Time { return time.Now().UTC() }, dummyHash: dummy}, nil
}

func (a *AuthService) Bootstrap(ctx context.Context, email, password string) (User, bool, error) {
	if strings.TrimSpace(email) == "" && password == "" {
		return User{}, false, nil
	}
	normalized, err := NormalizeEmail(email)
	if err != nil {
		return User{}, false, fmt.Errorf("bootstrap email: %w", err)
	}
	if existing, found, findErr := a.Repository.FindUserByEmail(ctx, normalized); findErr != nil {
		return User{}, false, findErr
	} else if found {
		return existing, false, nil
	}
	hash, err := a.Hasher.Hash(password)
	if err != nil {
		return User{}, false, fmt.Errorf("bootstrap password: %w", err)
	}
	user, err := a.Repository.CreateUser(ctx, User{Email: normalized, DisplayName: "Local owner", PasswordHash: hash, Status: "active"})
	if err != nil {
		return User{}, false, err
	}
	if err := a.Repository.AssignLegacySquads(ctx, user.ID); err != nil {
		return User{}, false, fmt.Errorf("assign legacy workspace: %w", err)
	}
	a.record(ctx, &user.ID, "bootstrap", "success", "local", "bootstrap")
	return user, true, nil
}

func NormalizeEmail(value string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	address, err := mail.ParseAddress(normalized)
	if err != nil || address.Address != normalized || !strings.Contains(normalized, "@") || len(normalized) > 254 {
		return "", ErrInvalidEmail
	}
	return normalized, nil
}

func (a *AuthService) Register(ctx context.Context, email, password, displayName, sourceAddress, requestID string) (AuthSessionResult, error) {
	if !a.Config.RegistrationEnabled {
		a.record(ctx, nil, "registration", "disabled", sourceAddress, requestID)
		return AuthSessionResult{}, ErrRegistrationDisabled
	}
	normalized, err := NormalizeEmail(email)
	if err != nil {
		return AuthSessionResult{}, err
	}
	if retry := a.Limiter.Check(normalized, sourceAddress, a.Now()); retry > 0 {
		return AuthSessionResult{}, RateLimitError{RetryAfter: retry}
	}
	hash, err := a.Hasher.Hash(password)
	if err != nil {
		a.Limiter.Failure(normalized, sourceAddress, a.Now())
		return AuthSessionResult{}, err
	}
	user, err := a.Repository.CreateUser(ctx, User{Email: normalized, DisplayName: strings.TrimSpace(displayName), PasswordHash: hash, Status: "active"})
	if err != nil {
		a.Limiter.Failure(normalized, sourceAddress, a.Now())
		a.record(ctx, nil, "registration", "failed", sourceAddress, requestID)
		return AuthSessionResult{}, err
	}
	a.Limiter.Success(normalized, sourceAddress)
	result, err := a.newSession(ctx, user, map[string]string{"sourceAddress": sourceAddress})
	if err == nil {
		a.record(ctx, &user.ID, "registration", "success", sourceAddress, requestID)
	}
	return result, err
}

func (a *AuthService) Login(ctx context.Context, email, password, sourceAddress, requestID string) (AuthSessionResult, error) {
	normalized, normalizeErr := NormalizeEmail(email)
	if normalizeErr != nil {
		normalized = "invalid"
	}
	if retry := a.Limiter.Check(normalized, sourceAddress, a.Now()); retry > 0 {
		return AuthSessionResult{}, RateLimitError{RetryAfter: retry}
	}
	user, found, err := a.Repository.FindUserByEmail(ctx, normalized)
	if err != nil {
		return AuthSessionResult{}, err
	}
	hash := a.dummyHash
	if found {
		hash = user.PasswordHash
	}
	valid, upgrade := a.Hasher.Verify(hash, password)
	if !found || !valid || user.Status != "active" || normalizeErr != nil {
		a.Limiter.Failure(normalized, sourceAddress, a.Now())
		a.record(ctx, userIDPointer(user, found), "login", "failed", sourceAddress, requestID)
		return AuthSessionResult{}, ErrInvalidCredentials
	}
	if upgrade {
		if next, hashErr := a.Hasher.Hash(password); hashErr == nil {
			_ = a.Repository.UpdateUserPassword(ctx, user.ID, next)
		}
	}
	a.Limiter.Success(normalized, sourceAddress)
	result, err := a.newSession(ctx, user, map[string]string{"sourceAddress": sourceAddress})
	if err == nil {
		_ = a.Repository.MarkUserLogin(ctx, user.ID, a.Now())
		a.record(ctx, &user.ID, "login", "success", sourceAddress, requestID)
	}
	return result, err
}

func (a *AuthService) Authenticate(ctx context.Context, token string) (AuthSessionResult, error) {
	if token == "" {
		return AuthSessionResult{}, ErrSessionInvalid
	}
	now := a.Now()
	session, user, found, err := a.Repository.FindSessionByTokenHash(ctx, hashToken(token))
	if err != nil {
		return AuthSessionResult{}, err
	}
	if !found || session.RevokedAt != nil || !now.Before(session.IdleExpiresAt) || !now.Before(session.AbsoluteExpiresAt) || user.Status != "active" {
		if found && session.RevokedAt == nil {
			_ = a.Repository.RevokeSession(ctx, session.ID, now)
		}
		return AuthSessionResult{}, ErrSessionInvalid
	}
	idleExpiry := now.Add(a.Config.IdleTimeout)
	if idleExpiry.After(session.AbsoluteExpiresAt) {
		idleExpiry = session.AbsoluteExpiresAt
	}
	if err := a.Repository.RefreshSession(ctx, session.ID, session.CSRFHash, now, idleExpiry); err != nil {
		return AuthSessionResult{}, err
	}
	return AuthSessionResult{User: safeUser(user), ExpiresAt: session.AbsoluteExpiresAt, Token: token, SessionID: session.ID, CSRFHash: session.CSRFHash}, nil
}

func (a *AuthService) RefreshCSRF(ctx context.Context, session AuthSessionResult) (AuthSessionResult, error) {
	if session.Token == "" {
		return AuthSessionResult{}, ErrSessionInvalid
	}
	csrf := deriveCSRF(session.Token)
	csrfHash := hashToken(csrf)
	now := a.Now()
	idleExpiry := now.Add(a.Config.IdleTimeout)
	if idleExpiry.After(session.ExpiresAt) {
		idleExpiry = session.ExpiresAt
	}
	if err := a.Repository.RefreshSession(ctx, session.SessionID, csrfHash, now, idleExpiry); err != nil {
		return AuthSessionResult{}, err
	}
	session.CSRFToken = csrf
	session.CSRFHash = csrfHash
	return session, nil
}

func (a *AuthService) Logout(ctx context.Context, sessionID int64, now time.Time) error {
	if sessionID == 0 {
		return nil
	}
	return a.Repository.RevokeSession(ctx, sessionID, now)
}

func (a *AuthService) ChangePassword(ctx context.Context, session AuthSessionResult, currentPassword, newPassword, sourceAddress, requestID string) (AuthSessionResult, error) {
	user, found, err := a.Repository.FindUserByID(ctx, session.User.ID)
	if err != nil || !found {
		return AuthSessionResult{}, ErrInvalidCredentials
	}
	if retry := a.Limiter.Check(user.Email, sourceAddress, a.Now()); retry > 0 {
		return AuthSessionResult{}, RateLimitError{RetryAfter: retry}
	}
	valid, _ := a.Hasher.Verify(user.PasswordHash, currentPassword)
	if !valid {
		a.Limiter.Failure(user.Email, sourceAddress, a.Now())
		a.record(ctx, &user.ID, "password_change", "failed", sourceAddress, requestID)
		return AuthSessionResult{}, ErrInvalidCredentials
	}
	hash, err := a.Hasher.Hash(newPassword)
	if err != nil {
		a.Limiter.Failure(user.Email, sourceAddress, a.Now())
		return AuthSessionResult{}, err
	}
	if err := a.Repository.UpdateUserPassword(ctx, user.ID, hash); err != nil {
		return AuthSessionResult{}, err
	}
	now := a.Now()
	if err := a.Repository.RevokeOtherSessions(ctx, user.ID, session.SessionID, now); err != nil {
		return AuthSessionResult{}, err
	}
	if err := a.Repository.RevokeSession(ctx, session.SessionID, now); err != nil {
		return AuthSessionResult{}, err
	}
	result, err := a.newSession(ctx, user, map[string]string{"sourceAddress": sourceAddress})
	if err == nil {
		a.Limiter.Success(user.Email, sourceAddress)
		a.record(ctx, &user.ID, "password_change", "success", sourceAddress, requestID)
	}
	return result, err
}

func (a *AuthService) VerifyCSRF(sessionID int64, supplied string, storedHash string) bool {
	return sessionID > 0 && supplied != "" && subtle.ConstantTimeCompare([]byte(hashToken(supplied)), []byte(storedHash)) == 1
}

func (a *AuthService) newSession(ctx context.Context, user User, metadata map[string]string) (AuthSessionResult, error) {
	now := a.Now()
	token, tokenHash, err := newToken()
	if err != nil {
		return AuthSessionResult{}, err
	}
	csrf := deriveCSRF(token)
	csrfHash := hashToken(csrf)
	session, err := a.Repository.CreateSession(ctx, Session{UserID: user.ID, TokenHash: tokenHash, CSRFHash: csrfHash, CreatedAt: now, LastSeenAt: now, IdleExpiresAt: now.Add(a.Config.IdleTimeout), AbsoluteExpiresAt: now.Add(a.Config.AbsoluteTimeout), DeviceMetadata: metadata})
	if err != nil {
		return AuthSessionResult{}, err
	}
	return AuthSessionResult{User: safeUser(user), CSRFToken: csrf, ExpiresAt: session.AbsoluteExpiresAt, Token: token, SessionID: session.ID, CSRFHash: csrfHash}, nil
}

func safeUser(user User) User {
	user.PasswordHash = ""
	return user
}

func (a *AuthService) record(ctx context.Context, userID *int64, eventType, outcome, sourceAddress, requestID string) {
	_ = a.Repository.RecordSecurityEvent(ctx, SecurityEvent{RequestID: requestID, UserID: userID, EventType: eventType, Outcome: outcome, SourceAddress: sourceAddress, OccurredAt: a.Now()})
}

func newToken() (string, string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", "", err
	}
	raw := base64.RawURLEncoding.EncodeToString(value)
	return raw, hashToken(raw), nil
}

func hashToken(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func deriveCSRF(sessionToken string) string {
	sum := sha256.Sum256([]byte("fantasy-helper:csrf:" + sessionToken))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func userIDPointer(user User, found bool) *int64 {
	if !found {
		return nil
	}
	return &user.ID
}

type RateLimitError struct {
	RetryAfter time.Duration
}

func (e RateLimitError) Error() string { return "credential attempts are rate limited" }

type credentialAttempts struct {
	Failures    []time.Time
	LockedUntil time.Time
	Windows     int
}

type CredentialLimiter struct {
	mu       sync.Mutex
	attempts map[string]credentialAttempts
}

func NewCredentialLimiter() *CredentialLimiter {
	return &CredentialLimiter{attempts: map[string]credentialAttempts{}}
}

func credentialKey(email, source string) string { return email + "\x00" + source }

func (l *CredentialLimiter) Check(email, source string, now time.Time) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	key := credentialKey(email, source)
	entry := l.attempts[key]
	if now.Before(entry.LockedUntil) {
		return entry.LockedUntil.Sub(now)
	}
	cutoff := now.Add(-15 * time.Minute)
	kept := entry.Failures[:0]
	for _, failure := range entry.Failures {
		if failure.After(cutoff) {
			kept = append(kept, failure)
		}
	}
	entry.Failures = kept
	if len(entry.Failures) >= 5 {
		entry.Windows++
		cooldown := time.Duration(entry.Windows) * time.Minute
		if cooldown > 15*time.Minute {
			cooldown = 15 * time.Minute
		}
		entry.LockedUntil = now.Add(cooldown)
		l.attempts[key] = entry
		return cooldown
	}
	l.attempts[key] = entry
	return 0
}

func (l *CredentialLimiter) Failure(email, source string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	key := credentialKey(email, source)
	entry := l.attempts[key]
	entry.Failures = append(entry.Failures, now)
	l.attempts[key] = entry
}

func (l *CredentialLimiter) Success(email, source string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, credentialKey(email, source))
}
