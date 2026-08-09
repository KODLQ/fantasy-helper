package app

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type authContextKey struct{}

func (a *API) EnableAuth(service *AuthService) {
	a.Auth = service
}

func (a *API) withOptionalAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/v1/auth/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		if a.Auth == nil {
			next.ServeHTTP(w, r)
			return
		}
		cookie, err := r.Cookie(a.authCookieName())
		if err == nil && cookie.Value != "" {
			session, authErr := a.Auth.Authenticate(r.Context(), cookie.Value)
			if authErr == nil {
				r = r.WithContext(context.WithValue(r.Context(), authContextKey{}, session))
			} else if errors.Is(authErr, ErrSessionInvalid) {
				a.clearAuthCookie(w)
			}
		}
		next.ServeHTTP(w, r)
	})
}

func authSessionFromContext(ctx context.Context) (AuthSessionResult, bool) {
	session, found := ctx.Value(authContextKey{}).(AuthSessionResult)
	return session, found
}

func (a *API) authConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeContractError(w, http.StatusMethodNotAllowed, requestIDFrom(w), "method_not_allowed", "Use GET for authentication configuration.", false, nil)
		return
	}
	writeEnvelope(w, http.StatusOK, requestIDFrom(w), Scope{}, Freshness{}, map[string]interface{}{"registrationEnabled": a.Auth != nil && a.Auth.Config.RegistrationEnabled, "emailProviderConfigured": false, "minimumPasswordLength": 12})
}

func (a *API) authRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeContractError(w, http.StatusMethodNotAllowed, requestIDFrom(w), "method_not_allowed", "Use POST to register.", false, nil)
		return
	}
	if a.Auth == nil {
		writeContractError(w, http.StatusServiceUnavailable, requestIDFrom(w), "auth_unavailable", "Authentication is unavailable.", true, nil)
		return
	}
	if !a.validMutationOrigin(r) {
		a.writeCSRFError(w)
		return
	}
	var input struct {
		Email       string `json:"email"`
		Password    string `json:"password"`
		DisplayName string `json:"displayName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeContractError(w, http.StatusBadRequest, requestIDFrom(w), "invalid_json", "Request body is not valid JSON.", false, nil)
		return
	}
	result, err := a.Auth.Register(r.Context(), input.Email, input.Password, input.DisplayName, requestSource(r), requestIDFrom(w))
	if err != nil {
		a.writeAuthError(w, err)
		return
	}
	a.rotatePreAuthSession(r.Context(), r, result.SessionID)
	a.setAuthCookie(w, result.Token, result.ExpiresAt)
	writeEnvelope(w, http.StatusCreated, requestIDFrom(w), Scope{}, Freshness{}, result)
}

func (a *API) authLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeContractError(w, http.StatusMethodNotAllowed, requestIDFrom(w), "method_not_allowed", "Use POST to log in.", false, nil)
		return
	}
	if a.Auth == nil {
		writeContractError(w, http.StatusServiceUnavailable, requestIDFrom(w), "auth_unavailable", "Authentication is unavailable.", true, nil)
		return
	}
	if !a.validMutationOrigin(r) {
		a.writeCSRFError(w)
		return
	}
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeContractError(w, http.StatusBadRequest, requestIDFrom(w), "invalid_json", "Request body is not valid JSON.", false, nil)
		return
	}
	result, err := a.Auth.Login(r.Context(), input.Email, input.Password, requestSource(r), requestIDFrom(w))
	if err != nil {
		a.writeAuthError(w, err)
		return
	}
	a.rotatePreAuthSession(r.Context(), r, result.SessionID)
	a.setAuthCookie(w, result.Token, result.ExpiresAt)
	writeEnvelope(w, http.StatusOK, requestIDFrom(w), Scope{}, Freshness{}, result)
}

func (a *API) authMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeContractError(w, http.StatusMethodNotAllowed, requestIDFrom(w), "method_not_allowed", "Use GET for the current user.", false, nil)
		return
	}
	session, ok := a.requireAuth(w, r)
	if !ok {
		return
	}
	refreshed, err := a.Auth.RefreshCSRF(r.Context(), session)
	if err != nil {
		writeContractError(w, http.StatusServiceUnavailable, requestIDFrom(w), "auth_unavailable", "Authentication is temporarily unavailable.", true, nil)
		return
	}
	writeEnvelope(w, http.StatusOK, requestIDFrom(w), Scope{}, Freshness{}, refreshed)
}

func (a *API) authLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeContractError(w, http.StatusMethodNotAllowed, requestIDFrom(w), "method_not_allowed", "Use POST to log out.", false, nil)
		return
	}
	session, found := authSessionFromContext(r.Context())
	if found && (!a.validMutationOrigin(r) || !a.Auth.VerifyCSRF(session.SessionID, r.Header.Get("X-CSRF-Token"), session.CSRFHash)) {
		a.writeCSRFError(w)
		return
	}
	if found {
		_ = a.Auth.Logout(r.Context(), session.SessionID, a.Auth.Now())
		a.Auth.record(r.Context(), &session.User.ID, "logout", "success", requestSource(r), requestIDFrom(w))
	}
	a.clearAuthCookie(w)
	writeEnvelope(w, http.StatusOK, requestIDFrom(w), Scope{}, Freshness{}, map[string]bool{"loggedOut": true})
}

func (a *API) authPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeContractError(w, http.StatusMethodNotAllowed, requestIDFrom(w), "method_not_allowed", "Use POST to change the password.", false, nil)
		return
	}
	session, ok := a.requireMutationAuth(w, r)
	if !ok {
		return
	}
	var input struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeContractError(w, http.StatusBadRequest, requestIDFrom(w), "invalid_json", "Request body is not valid JSON.", false, nil)
		return
	}
	result, err := a.Auth.ChangePassword(r.Context(), session, input.CurrentPassword, input.NewPassword, requestSource(r), requestIDFrom(w))
	if err != nil {
		a.writeAuthError(w, err)
		return
	}
	a.setAuthCookie(w, result.Token, result.ExpiresAt)
	writeEnvelope(w, http.StatusOK, requestIDFrom(w), Scope{}, Freshness{}, result)
}

func (a *API) requireAuth(w http.ResponseWriter, r *http.Request) (AuthSessionResult, bool) {
	if a.Auth == nil {
		return AuthSessionResult{}, true
	}
	session, found := authSessionFromContext(r.Context())
	if !found {
		writeContractError(w, http.StatusUnauthorized, requestIDFrom(w), "authentication_required", "Sign in to access this workspace.", false, nil)
		return AuthSessionResult{}, false
	}
	return session, true
}

func (a *API) requireMutationAuth(w http.ResponseWriter, r *http.Request) (AuthSessionResult, bool) {
	session, ok := a.requireAuth(w, r)
	if !ok || a.Auth == nil {
		return session, ok
	}
	if !a.validMutationOrigin(r) || !a.Auth.VerifyCSRF(session.SessionID, r.Header.Get("X-CSRF-Token"), session.CSRFHash) {
		a.writeCSRFError(w)
		return AuthSessionResult{}, false
	}
	return session, true
}

func (a *API) validMutationOrigin(r *http.Request) bool {
	if a.Auth == nil || a.Auth.Config.AllowedOrigin == "" {
		return true
	}
	origin := strings.TrimSuffix(r.Header.Get("Origin"), "/")
	if origin == "" {
		if referer := r.Header.Get("Referer"); referer != "" {
			parsed, err := url.Parse(referer)
			if err == nil {
				origin = parsed.Scheme + "://" + parsed.Host
			}
		}
	}
	return origin == strings.TrimSuffix(a.Auth.Config.AllowedOrigin, "/")
}

func (a *API) writeCSRFError(w http.ResponseWriter) {
	writeContractError(w, http.StatusForbidden, requestIDFrom(w), "csrf_failed", "The request origin or CSRF token is invalid.", false, nil)
}

func (a *API) writeAuthError(w http.ResponseWriter, err error) {
	var policy PasswordPolicyError
	var rateLimit RateLimitError
	switch {
	case errors.Is(err, ErrRegistrationDisabled):
		writeContractError(w, http.StatusForbidden, requestIDFrom(w), "registration_disabled", "Registration is disabled.", false, nil)
	case errors.Is(err, ErrDuplicateEmail):
		writeContractError(w, http.StatusConflict, requestIDFrom(w), "registration_conflict", "An account cannot be created with those details.", false, nil)
	case errors.Is(err, ErrInvalidCredentials):
		writeContractError(w, http.StatusUnauthorized, requestIDFrom(w), "invalid_credentials", "Email or password is incorrect.", false, nil)
	case errors.Is(err, ErrInvalidEmail):
		writeContractError(w, http.StatusUnprocessableEntity, requestIDFrom(w), "invalid_email", "Enter a valid email address.", false, nil)
	case errors.As(err, &policy):
		writeContractError(w, http.StatusUnprocessableEntity, requestIDFrom(w), "password_policy_failed", policy.Message, false, nil)
	case errors.As(err, &rateLimit):
		seconds := int(rateLimit.RetryAfter.Round(time.Second).Seconds())
		if seconds < 1 {
			seconds = 1
		}
		w.Header().Set("Retry-After", strconv.Itoa(seconds))
		writeContractError(w, http.StatusTooManyRequests, requestIDFrom(w), "auth_rate_limited", "Email or password is incorrect.", true, nil)
	default:
		writeContractError(w, http.StatusServiceUnavailable, requestIDFrom(w), "auth_unavailable", "Authentication is temporarily unavailable.", true, nil)
	}
}

func (a *API) authCookieName() string {
	if a.Auth != nil && a.Auth.Config.CookieSecure {
		return "__Host-fh_session"
	}
	return "fh_session"
}

func (a *API) setAuthCookie(w http.ResponseWriter, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{Name: a.authCookieName(), Value: token, Path: "/", HttpOnly: true, Secure: a.Auth.Config.CookieSecure, SameSite: http.SameSiteLaxMode, Expires: expires, MaxAge: int(time.Until(expires).Seconds())})
}

func (a *API) clearAuthCookie(w http.ResponseWriter) {
	secure := a.Auth != nil && a.Auth.Config.CookieSecure
	name := "fh_session"
	if secure {
		name = "__Host-fh_session"
	}
	http.SetCookie(w, &http.Cookie{Name: name, Value: "", Path: "/", HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode, MaxAge: -1, Expires: time.Unix(1, 0)})
}

func (a *API) rotatePreAuthSession(ctx context.Context, r *http.Request, nextSessionID int64) {
	if previous, found := authSessionFromContext(r.Context()); found && previous.SessionID != nextSessionID {
		_ = a.Auth.Logout(ctx, previous.SessionID, a.Auth.Now())
	}
}

func requestSource(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func requestIDFrom(w http.ResponseWriter) string { return w.Header().Get("X-Request-ID") }
