package audit

import (
	"sort"
	"strings"
	"sync"
	"time"
)

type Event struct {
	ID         int64          `json:"id"`
	Actor      string         `json:"actor"`
	Action     string         `json:"action"`
	TargetType string         `json:"targetType"`
	TargetID   string         `json:"targetId"`
	Outcome    string         `json:"outcome"`
	RequestID  string         `json:"requestId,omitempty"`
	SourceIP   string         `json:"sourceIp,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	OccurredAt string         `json:"occurredAt"`
}

type Query struct {
	Search  string
	Action  string
	Outcome string
	Limit   int
}

type Store interface {
	Record(Event) Event
	List(Query) []Event
}

type MemoryStore struct {
	mu     sync.RWMutex
	nextID int64
	events []Event
}

func NewMemoryStore() *MemoryStore {
	store := &MemoryStore{nextID: 4}
	store.events = []Event{
		{ID: 1, Actor: "system", Action: "directory.sync", TargetType: "directory", TargetID: "Northstar Directory", Outcome: "success", OccurredAt: "2026-10-14T06:30:00Z", Metadata: map[string]any{"users": 62, "groups": 14}},
		{ID: 2, Actor: "admin", Action: "policy.update", TargetType: "policy", TargetID: "Selfservice Users", Outcome: "success", OccurredAt: "2026-10-14T07:10:00Z"},
		{ID: 3, Actor: "scheduler", Action: "notification.expiry", TargetType: "notification", TargetID: "daily-expiry", Outcome: "success", OccurredAt: "2026-10-14T07:30:00Z", Metadata: map[string]any{"delivered": 12}},
	}
	return store
}

func (s *MemoryStore) Record(event Event) Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	event.ID = s.nextID
	s.nextID++
	if event.OccurredAt == "" {
		event.OccurredAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	event.Metadata = cloneMetadata(event.Metadata)
	s.events = append(s.events, event)
	return event
}

func (s *MemoryStore) List(query Query) []Event {
	s.mu.RLock()
	defer s.mu.RUnlock()
	limit := query.Limit
	if limit < 1 || limit > 500 {
		limit = 100
	}
	search := strings.ToLower(strings.TrimSpace(query.Search))
	result := make([]Event, 0, limit)
	for index := len(s.events) - 1; index >= 0 && len(result) < limit; index-- {
		event := s.events[index]
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
		event.Metadata = cloneMetadata(event.Metadata)
		result = append(result, event)
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].OccurredAt > result[j].OccurredAt })
	return result
}

func cloneMetadata(source map[string]any) map[string]any {
	if source == nil {
		return nil
	}
	copy := make(map[string]any, len(source))
	for key, value := range source {
		copy[key] = value
	}
	return copy
}
