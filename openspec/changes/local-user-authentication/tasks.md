## 1. Account and session persistence

- [x] 1.1 Add users, sessions, and redacted security-event migrations with case-insensitive email uniqueness.
- [x] 1.2 Implement Argon2id hashing, parameter/version configuration, password policy, and hash-upgrade behavior.
- [x] 1.3 Implement opaque token hashing, secure cookie settings, expiry, rotation, revocation, and logout.
- [x] 1.4 Add registration/bootstrap configuration and document the no-email-provider boundary.

## 2. API and security boundary

- [x] 2.1 Add register, login, logout, me, and password-change routes using the common response/error envelope.
- [x] 2.2 Add auth middleware, same-origin/CSRF protections, generic credential errors, and rate limiting.
- [x] 2.3 Add request correlation and redacted security events; prove no password/token appears in logs or responses.
- [x] 2.4 Add ownership to the currently persisted private squad records, repository scope helpers, a staged migration, and authorization tests; require dependent manager/analysis changes to add the same boundary when their private tables are introduced.

## 3. Frontend and Playwright

- [x] 3.1 Add login/register/logout/session-expired UI and protected-route loading/error states.
- [x] 3.2 Add Playwright tests for valid registration, duplicate email, invalid login, reload persistence, logout, expiry, password change, and protected-route denial.
- [x] 3.3 Add Playwright isolation tests with two users proving currently persisted private squads cannot cross accounts while public data remains readable; require equivalent active-team, alert, and saved-analysis cases in the dependent changes that introduce those records.
- [x] 3.4 Add API/integration tests for cookie attributes, login rotation, password-change rotation, revocation, rate limits, CSRF/origin checks, disabled registration, and common error envelopes.

## 4. Documentation and rollout

- [x] 4.1 Document local bootstrap, registration configuration, password policy, session lifetime, and data ownership.
- [x] 4.2 Run migrations, unit/integration tests, build checks, and headed Playwright tests against a disposable database.
