## Context

This is a local application, but its database and browser may contain private manager IDs, team selections, transfer experiments, and alerts. The design uses a conventional password/session boundary that is secure by default without adding external identity infrastructure.

## Decisions

### 1. Account and password model

Create `users` with immutable ID, normalized lowercase email, display name, password hash, status, created/updated timestamps, and last-login timestamp. Email uniqueness is enforced case-insensitively after trimming. Passwords are hashed with Argon2id using centrally configured parameters and a version marker so hashes can be upgraded after successful login. Password policy requires at least 12 characters and rejects common/obviously repeated values; the server never logs, returns, or stores the password.

Registration is enabled by configuration for local development. A first-user bootstrap may be supplied through a one-time environment secret or an interactive local command; bootstrap secrets are cleared after use. Email verification and password-reset email are explicitly out of scope until an email provider exists, and the UI says so rather than presenting a non-functional flow.

In non-development environments registration defaults to disabled and must be explicitly enabled. When disabled, the UI hides registration and the API returns the stable `registration_disabled` error without revealing whether an email exists. Bootstrap is the only account-creation path until an operator enables registration.

### 2. Session model

Create `sessions` with session ID, a hash of the opaque token, user ID, created/last-seen/idle-expiry/absolute-expiry timestamps, revoked timestamp, and redacted device metadata. The raw token exists only in the browser cookie. Login rotates any pre-auth session, creates a new token, and sets a `__Host-` cookie where deployment permits, with `HttpOnly`, `SameSite=Lax`, `Secure` in HTTPS, path `/`, idle expiry, and an absolute maximum lifetime. Logout revokes the server-side session. Password change revokes all other sessions.

State-changing requests require same-origin `Origin`/`Referer` validation and a CSRF token when the deployment mode requires cross-site protection. JSON APIs return 401/403 using the common error envelope; they do not redirect to HTML login pages.

### 3. Credential and abuse handling

Login, registration, and password-change attempts are rate-limited by normalized email and source address with bounded exponential lockout. Invalid email/password combinations return one generic message and do not reveal whether an account exists. Timing-sensitive verification is constant-work within normal implementation limits. Security events record request ID, user ID when known, event type, outcome, and timestamp, never passwords or session tokens.

Default abuse controls are five failed credential attempts per normalized email and source address in 15 minutes, followed by a one-minute cooldown; repeated windows increase cooldown up to 15 minutes. A successful login resets the failure counter for that email/source pair. Responses use 429 with `Retry-After` and the common `auth_rate_limited` code while preserving the generic credential message.

### 4. API contract

- `POST /api/v1/auth/register` accepts email, password, and optional display name; returns the authenticated user and session metadata.
- `POST /api/v1/auth/login` accepts email/password; returns the authenticated user and sets the session cookie.
- `POST /api/v1/auth/logout` revokes the current session and is idempotent.
- `GET /api/v1/auth/me` returns the current user or 401.
- `POST /api/v1/auth/password` accepts current and new password; rotates the current session and revokes other sessions.

All responses use `common-response-contract`. The user object never includes password hashes, credential policy internals, or raw session tokens.

Password change requires the current password and a valid new password. On success the current session is rotated, all other sessions are revoked, and the old session token is unusable immediately. A failed current-password check changes neither the hash nor session state.

For cookie-authenticated JSON mutations, same-origin `Origin`/`Referer` validation is mandatory. If the deployment uses a CSRF token, the token is bound to the session and required on every state-changing request; cross-site or missing-token requests fail before repository mutation.

### 5. Ownership boundary

Public warehouse facts, source payloads, and season catalog are shared application data. Manager connections, active-team snapshots, planning squads, transfer scenarios saved by a user, alerts, recommendation preferences, and private analysis runs require `userId` and authorization checks in repository methods and handlers. Every query is scoped by authenticated user before applying user-supplied IDs; an inaccessible ID returns the same not-found/forbidden policy chosen by the common contract without leaking ownership.

### 6. Browser and operational behavior

The app has an unauthenticated entry page, a registration form when enabled, a login form, a logout action, and a session-expired state that preserves no private response data in client storage. Protected pages load only after `/auth/me` resolves. Password forms disable duplicate submits, show safe validation, and clear credential fields after failure. Playwright runs cover registration, login, reload persistence, logout, wrong credentials, duplicate email, expiry/revocation, protected-route denial, and ownership isolation with a disposable database.

## Rollout and migration

1. Add user/session/security-event tables without changing public warehouse tables.
2. Add middleware and auth routes behind local registration configuration.
3. Backfill the existing single local workspace into the bootstrap user only when an explicit bootstrap identity is provided; otherwise require a new local user before exposing private data.
4. Add non-null ownership to private tables through a staged migration and reject unscoped repository access.
5. Enable protected manager/analysis routes after API and Playwright tests pass.

Rollback disables registration and protected writes but preserves revocable sessions and data. No rollback deletes user data.
