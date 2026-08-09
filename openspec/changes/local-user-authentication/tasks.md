## 1. Account and session persistence

- [ ] 1.1 Add users, sessions, and redacted security-event migrations with case-insensitive email uniqueness.
- [ ] 1.2 Implement Argon2id hashing, parameter/version configuration, password policy, and hash-upgrade behavior.
- [ ] 1.3 Implement opaque token hashing, secure cookie settings, expiry, rotation, revocation, and logout.
- [ ] 1.4 Add registration/bootstrap configuration and document the no-email-provider boundary.

## 2. API and security boundary

- [ ] 2.1 Add register, login, logout, me, and password-change routes using the common response/error envelope.
- [ ] 2.2 Add auth middleware, same-origin/CSRF protections, generic credential errors, and rate limiting.
- [ ] 2.3 Add request correlation and redacted security events; prove no password/token appears in logs or responses.
- [ ] 2.4 Add ownership columns, repository scope helpers, staged migration, and authorization tests for private manager/analysis records.

## 3. Frontend and Playwright

- [ ] 3.1 Add login/register/logout/session-expired UI and protected-route loading/error states.
- [ ] 3.2 Add Playwright tests for valid registration, duplicate email, invalid login, reload persistence, logout, expiry, password change, and protected-route denial.
- [ ] 3.3 Add Playwright isolation tests with two users proving private squads, active teams, alerts, and saved analyses cannot cross accounts while public data remains readable.
- [ ] 3.4 Add API/integration tests for cookie attributes, login rotation, password-change rotation, revocation, rate limits, CSRF/origin checks, disabled registration, and common error envelopes.

## 4. Documentation and rollout

- [ ] 4.1 Document local bootstrap, registration configuration, password policy, session lifetime, and data ownership.
- [ ] 4.2 Run migrations, unit/integration tests, build checks, and headed Playwright tests against a disposable database.
