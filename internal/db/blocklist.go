package db

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/btafoya/gosip/internal/models"
)

var ErrBlocklistEntryNotFound = errors.New("blocklist entry not found")

// BlocklistRepository handles database operations for blocked numbers
type BlocklistRepository struct {
	db *sql.DB
}

// NewBlocklistRepository creates a new BlocklistRepository
func NewBlocklistRepository(db *sql.DB) *BlocklistRepository {
	return &BlocklistRepository{db: db}
}

// Create inserts a new blocklist entry
func (r *BlocklistRepository) Create(ctx context.Context, entry *models.BlocklistEntry) error {
	result, err := r.db.ExecContext(ctx, `
		INSERT INTO blocklist (pattern, pattern_type, reason, created_at)
		VALUES (?, ?, ?, ?)
	`, entry.Pattern, entry.PatternType, entry.Reason, time.Now())
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	entry.ID = id
	return nil
}

// GetByID retrieves a blocklist entry by ID
func (r *BlocklistRepository) GetByID(ctx context.Context, id int64) (*models.BlocklistEntry, error) {
	entry := &models.BlocklistEntry{}
	err := r.db.QueryRowContext(ctx, `
		SELECT id, pattern, pattern_type, reason, created_at
		FROM blocklist WHERE id = ?
	`, id).Scan(&entry.ID, &entry.Pattern, &entry.PatternType, &entry.Reason, &entry.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrBlocklistEntryNotFound
	}
	if err != nil {
		return nil, err
	}
	return entry, nil
}

// Update updates an existing blocklist entry
func (r *BlocklistRepository) Update(ctx context.Context, entry *models.BlocklistEntry) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE blocklist SET pattern = ?, pattern_type = ?, reason = ?
		WHERE id = ?
	`, entry.Pattern, entry.PatternType, entry.Reason, entry.ID)
	return err
}

// Delete removes a blocklist entry
func (r *BlocklistRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM blocklist WHERE id = ?`, id)
	return err
}

// List returns all blocklist entries
func (r *BlocklistRepository) List(ctx context.Context) ([]*models.BlocklistEntry, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, pattern, pattern_type, reason, created_at
		FROM blocklist ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*models.BlocklistEntry
	for rows.Next() {
		entry := &models.BlocklistEntry{}
		if err := rows.Scan(&entry.ID, &entry.Pattern, &entry.PatternType, &entry.Reason, &entry.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

// IsBlocked checks if a phone number matches any blocklist entry
func (r *BlocklistRepository) IsBlocked(ctx context.Context, number string) (bool, *models.BlocklistEntry, error) {
	entries, err := r.List(ctx)
	if err != nil {
		return false, nil, err
	}

	// Normalize the number (remove spaces, dashes)
	normalizedNumber := normalizeNumber(number)

	for _, entry := range entries {
		normalizedPattern := normalizeNumber(entry.Pattern)

		switch entry.PatternType {
		case "exact":
			if normalizedNumber == normalizedPattern {
				return true, entry, nil
			}
		case "prefix":
			if strings.HasPrefix(normalizedNumber, normalizedPattern) {
				return true, entry, nil
			}
		case "regex":
			matched, err := regexp.MatchString(entry.Pattern, normalizedNumber)
			if err != nil {
				continue // Skip invalid regex patterns
			}
			if matched {
				return true, entry, nil
			}
		}
	}

	return false, nil, nil
}

// Count returns the total number of blocklist entries
func (r *BlocklistRepository) Count(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM blocklist`).Scan(&count)
	return count, err
}

// normalizeNumber removes non-digit characters except leading +
func normalizeNumber(number string) string {
	var result strings.Builder
	for i, ch := range number {
		if ch == '+' && i == 0 {
			result.WriteRune(ch)
		} else if ch >= '0' && ch <= '9' {
			result.WriteRune(ch)
		}
	}
	return result.String()
}
