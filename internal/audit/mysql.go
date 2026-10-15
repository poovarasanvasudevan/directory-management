package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"net"
	"strings"
	"time"

	"directory-management/internal/database"
)

type MySQLStore struct{ db *sql.DB }

func NewMySQLStore(db *sql.DB) *MySQLStore { return &MySQLStore{db: db} }

func (s *MySQLStore) Record(event Event) Event {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if event.OccurredAt == "" {
		event.OccurredAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	meta := cloneMetadata(event.Metadata)
	if meta == nil {
		meta = map[string]any{}
	}
	raw, _ := json.Marshal(meta)
	var sourceIP []byte
	if event.SourceIP != "" {
		if ip := net.ParseIP(event.SourceIP); ip != nil {
			sourceIP = ip
		}
	}
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO audit_events (tenant_id, actor_user_id, actor, action, target_type, target_id, outcome, request_id, source_ip, metadata, occurred_at)
		VALUES (?, NULL, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		database.DefaultTenantID, nullIfEmpty(event.Actor), event.Action, event.TargetType, nullIfEmpty(event.TargetID),
		event.Outcome, nullIfEmpty(event.RequestID), sourceIP, raw, parseTime(event.OccurredAt))
	if err != nil {
		return event
	}
	if id, err := result.LastInsertId(); err == nil {
		event.ID = id
	}
	event.Metadata = meta
	return event
}

func (s *MySQLStore) List(query Query) []Event {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	limit := query.Limit
	if limit < 1 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, COALESCE(actor, ''), action, target_type, COALESCE(target_id, ''), outcome,
		       COALESCE(request_id, ''), source_ip, metadata, occurred_at
		FROM audit_events
		WHERE tenant_id = ?
		ORDER BY occurred_at DESC, id DESC
		LIMIT ?`, database.DefaultTenantID, limit*3)
	if err != nil {
		return nil
	}
	defer rows.Close()
	search := strings.ToLower(strings.TrimSpace(query.Search))
	result := make([]Event, 0, limit)
	for rows.Next() {
		var event Event
		var sourceIP []byte
		var raw []byte
		var occurred time.Time
		if err := rows.Scan(&event.ID, &event.Actor, &event.Action, &event.TargetType, &event.TargetID, &event.Outcome, &event.RequestID, &sourceIP, &raw, &occurred); err != nil {
			continue
		}
		if len(sourceIP) > 0 {
			if ip := net.IP(sourceIP); ip != nil {
				event.SourceIP = ip.String()
			}
		}
		_ = json.Unmarshal(raw, &event.Metadata)
		event.OccurredAt = occurred.UTC().Format(time.RFC3339)
		if query.Action != "" && event.Action != query.Action {
			continue
		}
		if query.Outcome != "" && event.Outcome != query.Outcome {
			continue
		}
		candidate := strings.ToLower(event.Actor + " " + event.Action + " " + event.TargetType + " " + event.TargetID + " " + event.Outcome)
		if search != "" && !strings.Contains(candidate, search) {
			continue
		}
		result = append(result, event)
		if len(result) >= limit {
			break
		}
	}
	return result
}

func nullIfEmpty(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func parseTime(raw string) time.Time {
	if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return t
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t
	}
	return time.Now().UTC()
}
