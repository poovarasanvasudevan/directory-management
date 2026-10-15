package configstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"directory-management/internal/database"
)

type Profile struct {
	ModuleKey string
	Values    map[string]any
	Version   int64
	UpdatedAt string
}

type Revision struct {
	Version    int64          `json:"version"`
	Values     map[string]any `json:"values"`
	ModifiedBy string         `json:"modifiedBy"`
	CreatedAt  string         `json:"createdAt"`
}

var ErrVersionConflict = errors.New("configuration version conflict")

type Store interface {
	Get(ctx context.Context, moduleKey string) (Profile, error)
	Put(ctx context.Context, moduleKey string, values map[string]any) (Profile, error)
	PutVersion(ctx context.Context, moduleKey string, values map[string]any, expectedVersion int64) (Profile, error)
	History(ctx context.Context, moduleKey string, limit int) ([]Revision, error)
	Revision(ctx context.Context, moduleKey string, version int64) (Revision, error)
}

type MySQLStore struct{ db *sql.DB }

func NewMySQLStore(db *sql.DB) *MySQLStore { return &MySQLStore{db: db} }

func (s *MySQLStore) Get(ctx context.Context, moduleKey string) (Profile, error) {
	var raw []byte
	var profile Profile
	var updated time.Time
	err := s.db.QueryRowContext(ctx, `
		SELECT values_json, version, updated_at FROM configuration_profiles
		WHERE tenant_id = ? AND module_key = ?`, database.DefaultTenantID, moduleKey).Scan(&raw, &profile.Version, &updated)
	if err == sql.ErrNoRows {
		return Profile{}, fmt.Errorf("not found")
	}
	if err != nil {
		return Profile{}, err
	}
	values := map[string]any{}
	_ = json.Unmarshal(raw, &values)
	profile.ModuleKey = moduleKey
	profile.Values = values
	profile.UpdatedAt = updated.UTC().Format(time.RFC3339)
	return profile, nil
}

func (s *MySQLStore) Put(ctx context.Context, moduleKey string, values map[string]any) (Profile, error) {
	return s.PutVersion(ctx, moduleKey, values, -1)
}

func (s *MySQLStore) PutVersion(ctx context.Context, moduleKey string, values map[string]any, expectedVersion int64) (Profile, error) {
	raw, err := json.Marshal(values)
	if err != nil {
		return Profile{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Profile{}, err
	}
	defer tx.Rollback()
	var currentVersion int64
	err = tx.QueryRowContext(ctx, `SELECT version FROM configuration_profiles WHERE tenant_id = ? AND module_key = ? FOR UPDATE`, database.DefaultTenantID, moduleKey).Scan(&currentVersion)
	if err != nil && err != sql.ErrNoRows {
		return Profile{}, err
	}
	if err == sql.ErrNoRows {
		currentVersion = 0
	}
	if expectedVersion >= 0 && expectedVersion != currentVersion {
		return Profile{}, ErrVersionConflict
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO configuration_profiles (tenant_id, module_key, values_json, version, modified_by)
		VALUES (?, ?, ?, 1, ?)
		ON DUPLICATE KEY UPDATE values_json = VALUES(values_json), version = version + 1, modified_by = VALUES(modified_by)`,
		database.DefaultTenantID, moduleKey, raw, database.DefaultAdminUserID)
	if err != nil {
		return Profile{}, err
	}
	var profile Profile
	var updated time.Time
	err = tx.QueryRowContext(ctx, `SELECT values_json, version, updated_at FROM configuration_profiles WHERE tenant_id = ? AND module_key = ?`, database.DefaultTenantID, moduleKey).Scan(&raw, &profile.Version, &updated)
	if err != nil {
		return Profile{}, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO configuration_profile_revisions (tenant_id, module_key, version, values_json, modified_by) VALUES (?, ?, ?, ?, ?)`, database.DefaultTenantID, moduleKey, profile.Version, raw, database.DefaultAdminUserID)
	if err != nil {
		return Profile{}, err
	}
	if err := tx.Commit(); err != nil {
		return Profile{}, err
	}
	profile.ModuleKey = moduleKey
	profile.Values = values
	profile.UpdatedAt = updated.UTC().Format(time.RFC3339)
	return profile, nil
}

func (s *MySQLStore) History(ctx context.Context, moduleKey string, limit int) ([]Revision, error) {
	if limit < 1 || limit > 50 {
		limit = 10
	}
	rows, err := s.db.QueryContext(ctx, `SELECT version, values_json, COALESCE(u.display_name, 'system'), r.created_at FROM configuration_profile_revisions r LEFT JOIN users u ON u.id = r.modified_by WHERE r.tenant_id = ? AND r.module_key = ? ORDER BY r.version DESC LIMIT ?`, database.DefaultTenantID, moduleKey, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]Revision, 0, limit)
	for rows.Next() {
		var revision Revision
		var raw []byte
		var created time.Time
		if err := rows.Scan(&revision.Version, &raw, &revision.ModifiedBy, &created); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, &revision.Values); err != nil {
			return nil, err
		}
		revision.CreatedAt = created.UTC().Format(time.RFC3339)
		result = append(result, revision)
	}
	return result, rows.Err()
}

func (s *MySQLStore) Revision(ctx context.Context, moduleKey string, version int64) (Revision, error) {
	var revision Revision
	var raw []byte
	var created time.Time
	err := s.db.QueryRowContext(ctx, `SELECT r.version, r.values_json, COALESCE(u.display_name, 'system'), r.created_at FROM configuration_profile_revisions r LEFT JOIN users u ON u.id = r.modified_by WHERE r.tenant_id = ? AND r.module_key = ? AND r.version = ?`, database.DefaultTenantID, moduleKey, version).Scan(&revision.Version, &raw, &revision.ModifiedBy, &created)
	if err != nil {
		return Revision{}, err
	}
	if err := json.Unmarshal(raw, &revision.Values); err != nil {
		return Revision{}, err
	}
	revision.CreatedAt = created.UTC().Format(time.RFC3339)
	return revision, nil
}
