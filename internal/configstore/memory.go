package configstore

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type MemoryStore struct {
	mu       sync.RWMutex
	profiles map[string]Profile
	history  map[string][]Revision
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{profiles: map[string]Profile{}, history: map[string][]Revision{}}
}

func (s *MemoryStore) Get(_ context.Context, moduleKey string) (Profile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	profile, ok := s.profiles[moduleKey]
	if !ok {
		return Profile{}, fmt.Errorf("not found")
	}
	return cloneProfile(profile), nil
}

func (s *MemoryStore) Put(ctx context.Context, moduleKey string, values map[string]any) (Profile, error) {
	return s.PutVersion(ctx, moduleKey, values, -1)
}

func (s *MemoryStore) PutVersion(_ context.Context, moduleKey string, values map[string]any, expectedVersion int64) (Profile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	version := s.profiles[moduleKey].Version + 1
	if expectedVersion >= 0 && expectedVersion != version-1 {
		return Profile{}, ErrVersionConflict
	}
	now := time.Now().UTC().Format(time.RFC3339)
	profile := Profile{ModuleKey: moduleKey, Values: cloneMap(values), Version: version, UpdatedAt: now}
	s.profiles[moduleKey] = profile
	s.history[moduleKey] = append(s.history[moduleKey], Revision{Version: version, Values: cloneMap(values), ModifiedBy: "admin", CreatedAt: now})
	return cloneProfile(profile), nil
}

func (s *MemoryStore) History(_ context.Context, moduleKey string, limit int) ([]Revision, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := s.history[moduleKey]
	if limit < 1 || limit > 50 {
		limit = 10
	}
	result := make([]Revision, 0, min(limit, len(items)))
	for index := len(items) - 1; index >= 0 && len(result) < limit; index-- {
		item := items[index]
		item.Values = cloneMap(item.Values)
		result = append(result, item)
	}
	return result, nil
}

func (s *MemoryStore) Revision(_ context.Context, moduleKey string, version int64) (Revision, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, revision := range s.history[moduleKey] {
		if revision.Version == version {
			revision.Values = cloneMap(revision.Values)
			return revision, nil
		}
	}
	return Revision{}, fmt.Errorf("not found")
}

func cloneProfile(profile Profile) Profile {
	return Profile{ModuleKey: profile.ModuleKey, Values: cloneMap(profile.Values), Version: profile.Version, UpdatedAt: profile.UpdatedAt}
}

func cloneMap(source map[string]any) map[string]any {
	result := make(map[string]any, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}
