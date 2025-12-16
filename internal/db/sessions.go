package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Session represents an authentication session
type Session struct {
	ID           int64
	Token        string
	UserID       int64
	CreatedAt    time.Time
	ExpiresAt    time.Time
	LastActivity time.Time
	UserAgent    string
	IPAddress    string
}

// SessionRepository provides database operations for sessions
type SessionRepository struct {
	conn *sql.DB
}

// NewSessionRepository creates a new session repository
func NewSessionRepository(conn *sql.DB) *SessionRepository {
	return &SessionRepository{conn: conn}
}

// Create creates a new session
func (r *SessionRepository) Create(ctx context.Context, token string, userID int64, expiresAt time.Time, userAgent, ipAddress string) (*Session, error) {
	now := time.Now()

	result, err := r.conn.ExecContext(ctx, `
		INSERT INTO sessions (token, user_id, created_at, expires_at, last_activity, user_agent, ip_address)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, token, userID, now, expiresAt, now, userAgent, ipAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get session ID: %w", err)
	}

	return &Session{
		ID:           id,
		Token:        token,
		UserID:       userID,
		CreatedAt:    now,
		ExpiresAt:    expiresAt,
		LastActivity: now,
		UserAgent:    userAgent,
		IPAddress:    ipAddress,
	}, nil
}

// GetByToken retrieves a session by its token
func (r *SessionRepository) GetByToken(ctx context.Context, token string) (*Session, error) {
	var session Session
	var userAgent, ipAddress sql.NullString

	err := r.conn.QueryRowContext(ctx, `
		SELECT id, token, user_id, created_at, expires_at, last_activity, user_agent, ip_address
		FROM sessions
		WHERE token = ?
	`, token).Scan(
		&session.ID,
		&session.Token,
		&session.UserID,
		&session.CreatedAt,
		&session.ExpiresAt,
		&session.LastActivity,
		&userAgent,
		&ipAddress,
	)
	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound // Reuse existing error
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	if userAgent.Valid {
		session.UserAgent = userAgent.String
	}
	if ipAddress.Valid {
		session.IPAddress = ipAddress.String
	}

	return &session, nil
}

// UpdateActivity updates the last activity time and optionally extends the session
func (r *SessionRepository) UpdateActivity(ctx context.Context, token string, newExpiry time.Time) error {
	_, err := r.conn.ExecContext(ctx, `
		UPDATE sessions
		SET last_activity = ?, expires_at = ?
		WHERE token = ?
	`, time.Now(), newExpiry, token)
	if err != nil {
		return fmt.Errorf("failed to update session activity: %w", err)
	}
	return nil
}

// Delete removes a session by token
func (r *SessionRepository) Delete(ctx context.Context, token string) error {
	_, err := r.conn.ExecContext(ctx, `
		DELETE FROM sessions WHERE token = ?
	`, token)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}
	return nil
}

// DeleteByUserID removes all sessions for a user
func (r *SessionRepository) DeleteByUserID(ctx context.Context, userID int64) error {
	_, err := r.conn.ExecContext(ctx, `
		DELETE FROM sessions WHERE user_id = ?
	`, userID)
	if err != nil {
		return fmt.Errorf("failed to delete user sessions: %w", err)
	}
	return nil
}

// DeleteExpired removes all expired sessions
func (r *SessionRepository) DeleteExpired(ctx context.Context) (int64, error) {
	result, err := r.conn.ExecContext(ctx, `
		DELETE FROM sessions WHERE expires_at < ?
	`, time.Now())
	if err != nil {
		return 0, fmt.Errorf("failed to delete expired sessions: %w", err)
	}

	count, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get affected rows: %w", err)
	}

	return count, nil
}

// ListByUserID returns all sessions for a user
func (r *SessionRepository) ListByUserID(ctx context.Context, userID int64) ([]*Session, error) {
	rows, err := r.conn.QueryContext(ctx, `
		SELECT id, token, user_id, created_at, expires_at, last_activity, user_agent, ip_address
		FROM sessions
		WHERE user_id = ?
		ORDER BY last_activity DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*Session
	for rows.Next() {
		var session Session
		var userAgent, ipAddress sql.NullString

		err := rows.Scan(
			&session.ID,
			&session.Token,
			&session.UserID,
			&session.CreatedAt,
			&session.ExpiresAt,
			&session.LastActivity,
			&userAgent,
			&ipAddress,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}

		if userAgent.Valid {
			session.UserAgent = userAgent.String
		}
		if ipAddress.Valid {
			session.IPAddress = ipAddress.String
		}

		sessions = append(sessions, &session)
	}

	return sessions, nil
}

// CountByUserID returns the number of active sessions for a user
func (r *SessionRepository) CountByUserID(ctx context.Context, userID int64) (int, error) {
	var count int
	err := r.conn.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM sessions WHERE user_id = ? AND expires_at > ?
	`, userID, time.Now()).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count sessions: %w", err)
	}
	return count, nil
}

// IsValid checks if a session token is valid (exists and not expired)
func (r *SessionRepository) IsValid(ctx context.Context, token string) (bool, error) {
	var count int
	err := r.conn.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM sessions WHERE token = ? AND expires_at > ?
	`, token, time.Now()).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check session validity: %w", err)
	}
	return count > 0, nil
}
