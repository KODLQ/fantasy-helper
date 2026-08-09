## Why

The local application currently assumes a single unauthenticated user, which prevents safe persistence of manager connections, planning squads, alerts, and personal analysis history. A small, browser-first account boundary is needed before those features can be shared by multiple local users.

## What Changes

- Add local accounts identified by normalized email address and password credentials.
- Store password hashes with Argon2id and never persist or log plaintext passwords.
- Add secure, opaque, revocable browser sessions with idle and absolute expiry, rotation, logout, and safe cookie settings.
- Add registration, login, logout, current-user, and password-change APIs with generic credential errors and rate limiting.
- Add user ownership boundaries for manager settings, imported active teams, planning squads, alerts, and private analysis artifacts; public warehouse data remains shared and read-only.
- Add a configurable local registration/bootstrap policy without pretending to provide email verification when no mail provider exists.
- Add Playwright coverage for the complete auth lifecycle and protected routes.

## Capabilities

### New Capabilities

- `local-user-authentication`: Local account, password, session, and protected-route behavior.
- `user-workspace-ownership`: Ownership and isolation of private manager/analysis data on top of shared public data.

### Modified Capabilities

- None; existing manager and analysis changes must depend on this change before persisting user-owned data.

## Impact

- Adds user, session, password-policy, and security-event migrations plus authentication middleware.
- Adds `/api/v1/auth/*` routes and a typed client auth state.
- Requires manager sync, active-team import, squad planning, alerts, and private analysis records to include `userId` ownership.
- Does not add outbound email, social login, administrator roles, or FPL password storage.
