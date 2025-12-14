package db

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"

	"github.com/btafoya/gosip/internal/models"
)

var (
	ErrTokenNotFound = errors.New("provisioning token not found")
	ErrTokenExpired  = errors.New("provisioning token expired")
	ErrTokenRevoked  = errors.New("provisioning token revoked")
	ErrTokenMaxUses  = errors.New("provisioning token max uses exceeded")
)

// ProvisioningTokenRepository handles database operations for provisioning tokens
type ProvisioningTokenRepository struct {
	db *sql.DB
}

// NewProvisioningTokenRepository creates a new ProvisioningTokenRepository
func NewProvisioningTokenRepository(db *sql.DB) *ProvisioningTokenRepository {
	return &ProvisioningTokenRepository{db: db}
}

// GenerateToken generates a cryptographically secure random token
func GenerateToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// Create inserts a new provisioning token
func (r *ProvisioningTokenRepository) Create(ctx context.Context, token *models.ProvisioningToken) error {
	if token.Token == "" {
		t, err := GenerateToken()
		if err != nil {
			return err
		}
		token.Token = t
	}

	now := time.Now()
	token.CreatedAt = now

	result, err := r.db.ExecContext(ctx, `
		INSERT INTO provisioning_tokens (token, device_id, created_at, expires_at, revoked, used_count, max_uses, ip_restriction, created_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, token.Token, token.DeviceID, now, token.ExpiresAt, false, 0, token.MaxUses, token.IPRestriction, token.CreatedBy)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	token.ID = id
	return nil
}

// GetByToken retrieves a token by its value
func (r *ProvisioningTokenRepository) GetByToken(ctx context.Context, token string) (*models.ProvisioningToken, error) {
	pt := &models.ProvisioningToken{}
	err := r.db.QueryRowContext(ctx, `
		SELECT id, token, device_id, created_at, expires_at, revoked, revoked_at, used_count, max_uses, ip_restriction, created_by
		FROM provisioning_tokens WHERE token = ?
	`, token).Scan(&pt.ID, &pt.Token, &pt.DeviceID, &pt.CreatedAt, &pt.ExpiresAt, &pt.Revoked, &pt.RevokedAt, &pt.UsedCount, &pt.MaxUses, &pt.IPRestriction, &pt.CreatedBy)
	if err == sql.ErrNoRows {
		return nil, ErrTokenNotFound
	}
	if err != nil {
		return nil, err
	}
	return pt, nil
}

// GetByID retrieves a token by ID
func (r *ProvisioningTokenRepository) GetByID(ctx context.Context, id int64) (*models.ProvisioningToken, error) {
	pt := &models.ProvisioningToken{}
	err := r.db.QueryRowContext(ctx, `
		SELECT id, token, device_id, created_at, expires_at, revoked, revoked_at, used_count, max_uses, ip_restriction, created_by
		FROM provisioning_tokens WHERE id = ?
	`, id).Scan(&pt.ID, &pt.Token, &pt.DeviceID, &pt.CreatedAt, &pt.ExpiresAt, &pt.Revoked, &pt.RevokedAt, &pt.UsedCount, &pt.MaxUses, &pt.IPRestriction, &pt.CreatedBy)
	if err == sql.ErrNoRows {
		return nil, ErrTokenNotFound
	}
	if err != nil {
		return nil, err
	}
	return pt, nil
}

// ValidateAndUse validates a token and increments its use count if valid
func (r *ProvisioningTokenRepository) ValidateAndUse(ctx context.Context, token string, clientIP string) (*models.ProvisioningToken, error) {
	pt, err := r.GetByToken(ctx, token)
	if err != nil {
		return nil, err
	}

	// Check if revoked
	if pt.Revoked {
		return nil, ErrTokenRevoked
	}

	// Check if expired
	if time.Now().After(pt.ExpiresAt) {
		return nil, ErrTokenExpired
	}

	// Check max uses
	if pt.MaxUses > 0 && pt.UsedCount >= pt.MaxUses {
		return nil, ErrTokenMaxUses
	}

	// Check IP restriction if set
	if pt.IPRestriction != nil && *pt.IPRestriction != "" && *pt.IPRestriction != clientIP {
		return nil, errors.New("IP address not allowed for this token")
	}

	// Increment use count
	_, err = r.db.ExecContext(ctx, `UPDATE provisioning_tokens SET used_count = used_count + 1 WHERE id = ?`, pt.ID)
	if err != nil {
		return nil, err
	}
	pt.UsedCount++

	return pt, nil
}

// Revoke revokes a provisioning token
func (r *ProvisioningTokenRepository) Revoke(ctx context.Context, id int64) error {
	now := time.Now()
	_, err := r.db.ExecContext(ctx, `UPDATE provisioning_tokens SET revoked = TRUE, revoked_at = ? WHERE id = ?`, now, id)
	return err
}

// RevokeByToken revokes a provisioning token by its value
func (r *ProvisioningTokenRepository) RevokeByToken(ctx context.Context, token string) error {
	now := time.Now()
	_, err := r.db.ExecContext(ctx, `UPDATE provisioning_tokens SET revoked = TRUE, revoked_at = ? WHERE token = ?`, now, token)
	return err
}

// ListByDevice returns all tokens for a device
func (r *ProvisioningTokenRepository) ListByDevice(ctx context.Context, deviceID int64) ([]*models.ProvisioningToken, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, token, device_id, created_at, expires_at, revoked, revoked_at, used_count, max_uses, ip_restriction, created_by
		FROM provisioning_tokens WHERE device_id = ? ORDER BY created_at DESC
	`, deviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tokens []*models.ProvisioningToken
	for rows.Next() {
		pt := &models.ProvisioningToken{}
		if err := rows.Scan(&pt.ID, &pt.Token, &pt.DeviceID, &pt.CreatedAt, &pt.ExpiresAt, &pt.Revoked, &pt.RevokedAt, &pt.UsedCount, &pt.MaxUses, &pt.IPRestriction, &pt.CreatedBy); err != nil {
			return nil, err
		}
		tokens = append(tokens, pt)
	}
	return tokens, rows.Err()
}

// ListActive returns all active (non-revoked, non-expired) tokens
func (r *ProvisioningTokenRepository) ListActive(ctx context.Context) ([]*models.ProvisioningToken, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, token, device_id, created_at, expires_at, revoked, revoked_at, used_count, max_uses, ip_restriction, created_by
		FROM provisioning_tokens WHERE revoked = FALSE AND expires_at > ? ORDER BY created_at DESC
	`, time.Now())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tokens []*models.ProvisioningToken
	for rows.Next() {
		pt := &models.ProvisioningToken{}
		if err := rows.Scan(&pt.ID, &pt.Token, &pt.DeviceID, &pt.CreatedAt, &pt.ExpiresAt, &pt.Revoked, &pt.RevokedAt, &pt.UsedCount, &pt.MaxUses, &pt.IPRestriction, &pt.CreatedBy); err != nil {
			return nil, err
		}
		tokens = append(tokens, pt)
	}
	return tokens, rows.Err()
}

// Delete removes a provisioning token
func (r *ProvisioningTokenRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM provisioning_tokens WHERE id = ?`, id)
	return err
}

// CleanupExpired removes all expired tokens
func (r *ProvisioningTokenRepository) CleanupExpired(ctx context.Context) (int64, error) {
	result, err := r.db.ExecContext(ctx, `DELETE FROM provisioning_tokens WHERE expires_at < ?`, time.Now())
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
