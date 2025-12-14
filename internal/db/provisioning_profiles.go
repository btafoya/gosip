package db

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/btafoya/gosip/internal/models"
)

var (
	ErrProfileNotFound = errors.New("provisioning profile not found")
)

// ProvisioningProfileRepository handles database operations for provisioning profiles
type ProvisioningProfileRepository struct {
	db *sql.DB
}

// NewProvisioningProfileRepository creates a new ProvisioningProfileRepository
func NewProvisioningProfileRepository(db *sql.DB) *ProvisioningProfileRepository {
	return &ProvisioningProfileRepository{db: db}
}

// Create inserts a new provisioning profile
func (r *ProvisioningProfileRepository) Create(ctx context.Context, profile *models.ProvisioningProfile) error {
	now := time.Now()
	profile.CreatedAt = now
	profile.UpdatedAt = now

	result, err := r.db.ExecContext(ctx, `
		INSERT INTO provisioning_profiles (name, vendor, model, description, config_template, variables, is_default, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, profile.Name, profile.Vendor, profile.Model, profile.Description, profile.ConfigTemplate, profile.Variables, profile.IsDefault, now, now)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	profile.ID = id
	return nil
}

// GetByID retrieves a profile by ID
func (r *ProvisioningProfileRepository) GetByID(ctx context.Context, id int64) (*models.ProvisioningProfile, error) {
	profile := &models.ProvisioningProfile{}
	var variablesStr sql.NullString
	err := r.db.QueryRowContext(ctx, `
		SELECT id, name, vendor, model, description, config_template, variables, is_default, created_at, updated_at
		FROM provisioning_profiles WHERE id = ?
	`, id).Scan(&profile.ID, &profile.Name, &profile.Vendor, &profile.Model, &profile.Description, &profile.ConfigTemplate, &variablesStr, &profile.IsDefault, &profile.CreatedAt, &profile.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrProfileNotFound
	}
	if err != nil {
		return nil, err
	}
	if variablesStr.Valid && variablesStr.String != "" {
		profile.Variables = []byte(variablesStr.String)
	}
	return profile, nil
}

// GetByVendorModel retrieves a profile by vendor and model
func (r *ProvisioningProfileRepository) GetByVendorModel(ctx context.Context, vendor, model string) (*models.ProvisioningProfile, error) {
	profile := &models.ProvisioningProfile{}
	var variablesStr sql.NullString
	err := r.db.QueryRowContext(ctx, `
		SELECT id, name, vendor, model, description, config_template, variables, is_default, created_at, updated_at
		FROM provisioning_profiles WHERE vendor = ? AND model = ?
	`, vendor, model).Scan(&profile.ID, &profile.Name, &profile.Vendor, &profile.Model, &profile.Description, &profile.ConfigTemplate, &variablesStr, &profile.IsDefault, &profile.CreatedAt, &profile.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrProfileNotFound
	}
	if err != nil {
		return nil, err
	}
	if variablesStr.Valid && variablesStr.String != "" {
		profile.Variables = []byte(variablesStr.String)
	}
	return profile, nil
}

// GetDefaultForVendor retrieves the default profile for a vendor
func (r *ProvisioningProfileRepository) GetDefaultForVendor(ctx context.Context, vendor string) (*models.ProvisioningProfile, error) {
	profile := &models.ProvisioningProfile{}
	var variablesStr sql.NullString
	err := r.db.QueryRowContext(ctx, `
		SELECT id, name, vendor, model, description, config_template, variables, is_default, created_at, updated_at
		FROM provisioning_profiles WHERE vendor = ? AND is_default = TRUE LIMIT 1
	`, vendor).Scan(&profile.ID, &profile.Name, &profile.Vendor, &profile.Model, &profile.Description, &profile.ConfigTemplate, &variablesStr, &profile.IsDefault, &profile.CreatedAt, &profile.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrProfileNotFound
	}
	if err != nil {
		return nil, err
	}
	if variablesStr.Valid && variablesStr.String != "" {
		profile.Variables = []byte(variablesStr.String)
	}
	return profile, nil
}

// Update updates an existing profile
func (r *ProvisioningProfileRepository) Update(ctx context.Context, profile *models.ProvisioningProfile) error {
	profile.UpdatedAt = time.Now()
	_, err := r.db.ExecContext(ctx, `
		UPDATE provisioning_profiles SET name = ?, vendor = ?, model = ?, description = ?, config_template = ?, variables = ?, is_default = ?, updated_at = ?
		WHERE id = ?
	`, profile.Name, profile.Vendor, profile.Model, profile.Description, profile.ConfigTemplate, profile.Variables, profile.IsDefault, profile.UpdatedAt, profile.ID)
	return err
}

// Delete removes a provisioning profile
func (r *ProvisioningProfileRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM provisioning_profiles WHERE id = ?`, id)
	return err
}

// List returns all profiles with optional vendor filter
func (r *ProvisioningProfileRepository) List(ctx context.Context, vendor string) ([]*models.ProvisioningProfile, error) {
	var rows *sql.Rows
	var err error

	if vendor != "" {
		rows, err = r.db.QueryContext(ctx, `
			SELECT id, name, vendor, model, description, config_template, variables, is_default, created_at, updated_at
			FROM provisioning_profiles WHERE vendor = ? ORDER BY name ASC
		`, vendor)
	} else {
		rows, err = r.db.QueryContext(ctx, `
			SELECT id, name, vendor, model, description, config_template, variables, is_default, created_at, updated_at
			FROM provisioning_profiles ORDER BY vendor ASC, name ASC
		`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var profiles []*models.ProvisioningProfile
	for rows.Next() {
		profile := &models.ProvisioningProfile{}
		var variablesStr sql.NullString
		if err := rows.Scan(&profile.ID, &profile.Name, &profile.Vendor, &profile.Model, &profile.Description, &profile.ConfigTemplate, &variablesStr, &profile.IsDefault, &profile.CreatedAt, &profile.UpdatedAt); err != nil {
			return nil, err
		}
		if variablesStr.Valid && variablesStr.String != "" {
			profile.Variables = []byte(variablesStr.String)
		}
		profiles = append(profiles, profile)
	}
	return profiles, rows.Err()
}

// ListVendors returns all distinct vendors
func (r *ProvisioningProfileRepository) ListVendors(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT DISTINCT vendor FROM provisioning_profiles ORDER BY vendor ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var vendors []string
	for rows.Next() {
		var vendor string
		if err := rows.Scan(&vendor); err != nil {
			return nil, err
		}
		vendors = append(vendors, vendor)
	}
	return vendors, rows.Err()
}

// SetDefault sets a profile as the default for its vendor
func (r *ProvisioningProfileRepository) SetDefault(ctx context.Context, id int64) error {
	// Get the profile to find its vendor
	profile, err := r.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// Clear other defaults for this vendor
	_, err = r.db.ExecContext(ctx, `UPDATE provisioning_profiles SET is_default = FALSE WHERE vendor = ?`, profile.Vendor)
	if err != nil {
		return err
	}

	// Set this one as default
	_, err = r.db.ExecContext(ctx, `UPDATE provisioning_profiles SET is_default = TRUE WHERE id = ?`, id)
	return err
}
