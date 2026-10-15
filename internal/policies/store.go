package policies

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"directory-management/internal/database"
)

type Policy struct {
	ID          int64    `json:"id"`
	Name        string   `json:"name"`
	Scope       string   `json:"scope"`
	Domain      string   `json:"domain"`
	Permissions []string `json:"permissions"`
	Status      string   `json:"status"`
	Priority    int      `json:"priority"`
	OUDNs       []string `json:"ouDNs"`
	GroupDNs    []string `json:"groupDNs"`
	ModifiedBy  string   `json:"modifiedBy"`
	ModifiedAt  string   `json:"modifiedAt"`
}

type rulesPayload struct {
	Scope       string   `json:"scope"`
	Domain      string   `json:"domain"`
	Permissions []string `json:"permissions"`
	OUDNs       []string `json:"ouDNs"`
	GroupDNs    []string `json:"groupDNs"`
	UIStatus    string   `json:"uiStatus"`
	ModifiedBy  string   `json:"modifiedBy"`
}

type Store interface {
	List(ctx context.Context) ([]Policy, error)
	Create(ctx context.Context, policy Policy) (Policy, error)
	Update(ctx context.Context, policy Policy) (Policy, error)
	Delete(ctx context.Context, id int64) error
	Clone(ctx context.Context, id int64) (Policy, error)
	Reorder(ctx context.Context, orderedIDs []int64) error
	Evaluate(ctx context.Context, userOUDN string, userGroupDNs []string) (*Policy, error)
}

type MySQLStore struct{ db *sql.DB }

func NewMySQLStore(db *sql.DB) *MySQLStore { return &MySQLStore{db: db} }

func (s *MySQLStore) List(ctx context.Context) ([]Policy, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, priority, status, rules, updated_at
		FROM policies WHERE tenant_id = ? ORDER BY priority ASC, id ASC`, database.DefaultTenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Policy, 0)
	for rows.Next() {
		policy, err := scanPolicy(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, policy)
	}
	return items, rows.Err()
}

func (s *MySQLStore) Create(ctx context.Context, policy Policy) (Policy, error) {
	priority := policy.Priority
	if priority == 0 {
		var max sql.NullInt64
		_ = s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(priority), 0) FROM policies WHERE tenant_id = ?`, database.DefaultTenantID).Scan(&max)
		priority = int(max.Int64) + 10
	}
	policy.Priority = priority
	policy.ModifiedBy = "admin"
	policy.ModifiedAt = time.Now().UTC().Format(time.RFC3339)
	payload, err := json.Marshal(rulesFrom(policy))
	if err != nil {
		return Policy{}, err
	}
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO policies (tenant_id, name, priority, status, version, rules, created_by)
		VALUES (?, ?, ?, ?, 1, ?, ?)`,
		database.DefaultTenantID, policy.Name, priority, dbStatus(policy.Status), payload, database.DefaultAdminUserID)
	if err != nil {
		return Policy{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Policy{}, err
	}
	policy.ID = id
	return policy, nil
}

func (s *MySQLStore) Update(ctx context.Context, policy Policy) (Policy, error) {
	policy.ModifiedBy = "admin"
	policy.ModifiedAt = time.Now().UTC().Format(time.RFC3339)
	payload, err := json.Marshal(rulesFrom(policy))
	if err != nil {
		return Policy{}, err
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE policies SET name = ?, priority = ?, status = ?, version = version + 1, rules = ?
		WHERE id = ? AND tenant_id = ?`,
		policy.Name, policy.Priority, dbStatus(policy.Status), payload, policy.ID, database.DefaultTenantID)
	if err != nil {
		return Policy{}, err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return Policy{}, fmt.Errorf("policy not found")
	}
	return policy, nil
}

func (s *MySQLStore) Delete(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM policies WHERE id = ? AND tenant_id = ?`, id, database.DefaultTenantID)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("policy not found")
	}
	return nil
}

func (s *MySQLStore) Clone(ctx context.Context, id int64) (Policy, error) {
	items, err := s.List(ctx)
	if err != nil {
		return Policy{}, err
	}
	var source *Policy
	for i := range items {
		if items[i].ID == id {
			source = &items[i]
			break
		}
	}
	if source == nil {
		return Policy{}, fmt.Errorf("policy not found")
	}
	copy := *source
	copy.ID = 0
	copy.Name += " copy"
	return s.Create(ctx, copy)
}

func (s *MySQLStore) Reorder(ctx context.Context, orderedIDs []int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for index, id := range orderedIDs {
		if _, err := tx.ExecContext(ctx, `UPDATE policies SET priority = ? WHERE id = ? AND tenant_id = ?`, (index+1)*10, id, database.DefaultTenantID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *MySQLStore) Evaluate(ctx context.Context, userOUDN string, userGroupDNs []string) (*Policy, error) {
	items, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	groupSet := make(map[string]struct{}, len(userGroupDNs))
	for _, dn := range userGroupDNs {
		groupSet[strings.ToLower(dn)] = struct{}{}
	}
	for _, policy := range items {
		if policy.Status != "Active" {
			continue
		}
		if len(policy.OUDNs) == 0 && len(policy.GroupDNs) == 0 {
			return &policy, nil
		}
		for _, ou := range policy.OUDNs {
			if ou != "" && strings.Contains(strings.ToLower(userOUDN), strings.ToLower(ou)) {
				return &policy, nil
			}
		}
		for _, group := range policy.GroupDNs {
			if _, ok := groupSet[strings.ToLower(group)]; ok {
				return &policy, nil
			}
		}
	}
	return nil, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanPolicy(row rowScanner) (Policy, error) {
	var (
		id       int64
		name     string
		priority int
		status   string
		raw      []byte
		updated  time.Time
	)
	if err := row.Scan(&id, &name, &priority, &status, &raw, &updated); err != nil {
		return Policy{}, err
	}
	var rules rulesPayload
	_ = json.Unmarshal(raw, &rules)
	uiStatus := rules.UIStatus
	if uiStatus == "" {
		uiStatus = uiStatusFromDB(status)
	}
	return Policy{
		ID: id, Name: name, Scope: rules.Scope, Domain: rules.Domain, Permissions: rules.Permissions,
		Status: uiStatus, Priority: priority, OUDNs: rules.OUDNs, GroupDNs: rules.GroupDNs,
		ModifiedBy: rules.ModifiedBy, ModifiedAt: updated.UTC().Format(time.RFC3339),
	}, nil
}

func rulesFrom(policy Policy) rulesPayload {
	return rulesPayload{
		Scope: policy.Scope, Domain: policy.Domain, Permissions: policy.Permissions,
		OUDNs: policy.OUDNs, GroupDNs: policy.GroupDNs, UIStatus: policy.Status, ModifiedBy: policy.ModifiedBy,
	}
}

func dbStatus(ui string) string {
	switch ui {
	case "Active":
		return "active"
	case "Paused":
		return "disabled"
	default:
		return "draft"
	}
}

func uiStatusFromDB(db string) string {
	switch db {
	case "active":
		return "Active"
	case "disabled":
		return "Paused"
	default:
		return "Review"
	}
}
