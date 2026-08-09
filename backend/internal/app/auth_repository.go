package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

func (r *PostgresRepository) CreateUser(ctx context.Context, user User) (User, error) {
	var lastLogin sql.NullTime
	err := r.db.QueryRowContext(ctx, `INSERT INTO users (email, display_name, password_hash, status) VALUES ($1,$2,$3,$4) RETURNING id, email, display_name, password_hash, status, created_at, updated_at, last_login_at`, user.Email, user.DisplayName, user.PasswordHash, user.Status).Scan(&user.ID, &user.Email, &user.DisplayName, &user.PasswordHash, &user.Status, &user.CreatedAt, &user.UpdatedAt, &lastLogin)
	if err != nil {
		var postgresError *pgconn.PgError
		if errors.As(err, &postgresError) && postgresError.Code == "23505" {
			return User{}, ErrDuplicateEmail
		}
		return User{}, fmt.Errorf("create user: %w", err)
	}
	if lastLogin.Valid {
		user.LastLoginAt = &lastLogin.Time
	}
	return user, nil
}

func (r *PostgresRepository) FindUserByEmail(ctx context.Context, email string) (User, bool, error) {
	return r.findUser(ctx, `SELECT id, email, display_name, password_hash, status, created_at, updated_at, last_login_at FROM users WHERE LOWER(BTRIM(email))=$1`, email)
}

func (r *PostgresRepository) FindUserByID(ctx context.Context, id int64) (User, bool, error) {
	return r.findUser(ctx, `SELECT id, email, display_name, password_hash, status, created_at, updated_at, last_login_at FROM users WHERE id=$1`, id)
}

func (r *PostgresRepository) findUser(ctx context.Context, query string, value interface{}) (User, bool, error) {
	var user User
	var lastLogin sql.NullTime
	err := r.db.QueryRowContext(ctx, query, value).Scan(&user.ID, &user.Email, &user.DisplayName, &user.PasswordHash, &user.Status, &user.CreatedAt, &user.UpdatedAt, &lastLogin)
	if err == sql.ErrNoRows {
		return User{}, false, nil
	}
	if err != nil {
		return User{}, false, err
	}
	if lastLogin.Valid {
		user.LastLoginAt = &lastLogin.Time
	}
	return user, true, nil
}

func (r *PostgresRepository) UpdateUserPassword(ctx context.Context, userID int64, passwordHash string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE users SET password_hash=$1, updated_at=NOW() WHERE id=$2`, passwordHash, userID)
	return err
}

func (r *PostgresRepository) MarkUserLogin(ctx context.Context, userID int64, at time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE users SET last_login_at=$1, updated_at=NOW() WHERE id=$2`, at, userID)
	return err
}

func (r *PostgresRepository) CreateSession(ctx context.Context, session Session) (Session, error) {
	metadata, err := json.Marshal(session.DeviceMetadata)
	if err != nil {
		return Session{}, err
	}
	err = r.db.QueryRowContext(ctx, `INSERT INTO sessions (token_hash, csrf_hash, user_id, created_at, last_seen_at, idle_expires_at, absolute_expires_at, device_metadata) VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`, session.TokenHash, session.CSRFHash, session.UserID, session.CreatedAt, session.LastSeenAt, session.IdleExpiresAt, session.AbsoluteExpiresAt, metadata).Scan(&session.ID)
	if err != nil {
		return Session{}, fmt.Errorf("create session: %w", err)
	}
	return session, nil
}

func (r *PostgresRepository) FindSessionByTokenHash(ctx context.Context, tokenHash string) (Session, User, bool, error) {
	var session Session
	var user User
	var revoked, lastLogin sql.NullTime
	var metadata []byte
	err := r.db.QueryRowContext(ctx, `SELECT s.id, s.user_id, s.token_hash, s.csrf_hash, s.created_at, s.last_seen_at, s.idle_expires_at, s.absolute_expires_at, s.revoked_at, s.device_metadata, u.id, u.email, u.display_name, u.password_hash, u.status, u.created_at, u.updated_at, u.last_login_at FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.token_hash=$1`, tokenHash).Scan(&session.ID, &session.UserID, &session.TokenHash, &session.CSRFHash, &session.CreatedAt, &session.LastSeenAt, &session.IdleExpiresAt, &session.AbsoluteExpiresAt, &revoked, &metadata, &user.ID, &user.Email, &user.DisplayName, &user.PasswordHash, &user.Status, &user.CreatedAt, &user.UpdatedAt, &lastLogin)
	if err == sql.ErrNoRows {
		return Session{}, User{}, false, nil
	}
	if err != nil {
		return Session{}, User{}, false, err
	}
	if revoked.Valid {
		session.RevokedAt = &revoked.Time
	}
	if lastLogin.Valid {
		user.LastLoginAt = &lastLogin.Time
	}
	_ = json.Unmarshal(metadata, &session.DeviceMetadata)
	return session, user, true, nil
}

func (r *PostgresRepository) RefreshSession(ctx context.Context, sessionID int64, csrfHash string, seenAt, idleExpiresAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE sessions SET csrf_hash=$1, last_seen_at=$2, idle_expires_at=$3 WHERE id=$4 AND revoked_at IS NULL`, csrfHash, seenAt, idleExpiresAt, sessionID)
	return err
}

func (r *PostgresRepository) RevokeSession(ctx context.Context, sessionID int64, at time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE sessions SET revoked_at=COALESCE(revoked_at,$1) WHERE id=$2`, at, sessionID)
	return err
}

func (r *PostgresRepository) RevokeOtherSessions(ctx context.Context, userID, keepSessionID int64, at time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE sessions SET revoked_at=COALESCE(revoked_at,$1) WHERE user_id=$2 AND id<>$3`, at, userID, keepSessionID)
	return err
}

func (r *PostgresRepository) RecordSecurityEvent(ctx context.Context, event SecurityEvent) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO security_events (request_id, user_id, event_type, outcome, source_address, occurred_at) VALUES ($1,$2,$3,$4,$5,$6)`, event.RequestID, event.UserID, event.EventType, event.Outcome, nullableString(event.SourceAddress), event.OccurredAt)
	return err
}

func (r *PostgresRepository) AssignLegacySquads(ctx context.Context, userID int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE squad_plans SET user_id=$1 WHERE user_id IS NULL`, userID)
	return err
}
