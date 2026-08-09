## ADDED Requirements

### Requirement: Register local users safely

The system SHALL create a local user from a normalized unique email and password hash using Argon2id, subject to the configured password policy, and SHALL never store or expose the plaintext password.

#### Scenario: Valid registration
- **WHEN** registration is enabled and a valid email/password is submitted
- **THEN** the system creates one user, creates an authenticated session, and returns a safe user object in the common response envelope

#### Scenario: Duplicate email
- **WHEN** an email matching an existing account after normalization is submitted
- **THEN** registration fails with a stable generic conflict message and does not create a second user

### Requirement: Authenticate with generic errors and abuse controls

The system SHALL authenticate valid credentials, return a generic failure for invalid credentials, and rate-limit repeated registration/login/password attempts.

#### Scenario: Invalid login
- **WHEN** an unknown email or incorrect password is submitted
- **THEN** the response does not disclose which credential failed, does not create a session, and records a redacted security event

### Requirement: Protect browser sessions

The system SHALL use opaque server-revocable sessions with token hashes in the database, rotation on login/password change, idle and absolute expiry, and secure cookie attributes.

#### Scenario: Session expires
- **WHEN** a request uses an expired or revoked session cookie
- **THEN** the protected request returns the common 401 error and the client clears private in-memory state

#### Scenario: Logout
- **WHEN** the current user logs out
- **THEN** the session is revoked, the cookie is cleared, and repeating logout is harmless

### Requirement: Enforce protected ownership

The system SHALL require authentication and `userId` ownership for private manager, squad, alert, and saved-analysis data while allowing shared public warehouse reads.

#### Scenario: User accesses another user's private object
- **WHEN** an authenticated user requests a private object owned by another user
- **THEN** the API does not return the object or reveal its private fields

#### Scenario: Public warehouse read
- **WHEN** an authenticated or unauthenticated client requests public season/player data
- **THEN** the request may read shared warehouse data subject to freshness and scope contracts

### Requirement: Change passwords with session rotation

The system SHALL require the current password for password changes, hash the new password with the configured Argon2id policy, rotate the current session, and revoke all other sessions on success.

#### Scenario: Password change succeeds
- **WHEN** an authenticated user submits the correct current password and a valid new password
- **THEN** the password hash changes, the current session receives a new token, all other sessions are revoked, and the old token immediately returns 401

#### Scenario: Current password is wrong
- **WHEN** a password change uses an incorrect current password
- **THEN** neither the password hash nor any session state changes and the response is a generic credential error

### Requirement: Enforce CSRF and origin checks

The system SHALL reject cookie-authenticated state-changing requests with a cross-site origin, missing required same-origin headers, or an invalid/missing session-bound CSRF token before repository mutation.

#### Scenario: Cross-site mutation
- **WHEN** a cross-site POST, PUT, PATCH, or DELETE is submitted without a valid session-bound CSRF token
- **THEN** the API returns `csrf_failed` using the common error envelope and makes no state change

#### Scenario: Valid same-origin mutation
- **WHEN** a same-origin mutation supplies the valid session-bound CSRF token when required
- **THEN** the request proceeds through authentication and ownership checks

### Requirement: Rate-limit credential abuse

The system SHALL enforce five failed credential attempts per normalized email and source address within 15 minutes, apply bounded cooldowns, and return `Retry-After` with a stable rate-limit error.

#### Scenario: Credential threshold is exceeded
- **WHEN** the sixth failed credential attempt occurs within the configured window
- **THEN** the API returns 429 with `auth_rate_limited`, a `Retry-After` value, no session, and no account-existence disclosure

### Requirement: Disable public registration safely

The system SHALL allow registration to be disabled outside development and SHALL prevent account creation while preserving bootstrap and login behavior.

#### Scenario: Registration is disabled
- **WHEN** a client submits registration while the registration policy is disabled
- **THEN** the API returns `registration_disabled`, creates no account/session, and the UI does not show an enabled registration action
