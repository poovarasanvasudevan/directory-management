package policies

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type MemoryStore struct {
	mu       sync.RWMutex
	nextID   int64
	policies map[int64]Policy
}

func NewMemoryStore() *MemoryStore {
	store := &MemoryStore{nextID: 1, policies: map[int64]Policy{}}
	seeds := []Policy{
		{Name: "Self-service users", Scope: "All employees", Domain: "ADSELFSERVICE", Permissions: []string{"Reset Password", "Unlock Account", "Self Update", "Change Password"}, Status: "Active", Priority: 10},
		{Name: "Endpoint MFA users", Scope: "Endpoint MFA group", Domain: "ADSELFSERVICE", Permissions: []string{"Reset Password", "Unlock Account", "Change Password"}, Status: "Active", Priority: 20, GroupDNs: []string{"CN=Endpoint MFA,OU=Groups,DC=example,DC=com"}},
		{Name: "Remote users", Scope: "Remote workforce", Domain: "ADSELFSERVICE", Permissions: []string{"Reset Password", "Self Update", "Change Password"}, Status: "Review", Priority: 30},
		{Name: "Technical support", Scope: "Support technicians", Domain: "ADSELFSERVICE", Permissions: []string{"Self Update", "Change Password"}, Status: "Active", Priority: 40},
	}
	for _, policy := range seeds {
		_, _ = store.Create(context.Background(), policy)
	}
	return store
}

func (s *MemoryStore) List(context.Context) ([]Policy, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]Policy, 0, len(s.policies))
	for _, item := range s.policies {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Priority == items[j].Priority {
			return items[i].ID < items[j].ID
		}
		return items[i].Priority < items[j].Priority
	})
	return items, nil
}

func (s *MemoryStore) Create(_ context.Context, policy Policy) (Policy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if policy.Priority == 0 {
		policy.Priority = int(s.nextID) * 10
	}
	policy.ID = s.nextID
	s.nextID++
	policy.ModifiedBy = "admin"
	policy.ModifiedAt = time.Now().UTC().Format(time.RFC3339)
	if policy.OUDNs == nil {
		policy.OUDNs = []string{}
	}
	if policy.GroupDNs == nil {
		policy.GroupDNs = []string{}
	}
	s.policies[policy.ID] = policy
	return policy, nil
}

func (s *MemoryStore) Update(_ context.Context, policy Policy) (Policy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.policies[policy.ID]; !ok {
		return Policy{}, fmt.Errorf("policy not found")
	}
	policy.ModifiedBy = "admin"
	policy.ModifiedAt = time.Now().UTC().Format(time.RFC3339)
	s.policies[policy.ID] = policy
	return policy, nil
}

func (s *MemoryStore) Delete(_ context.Context, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.policies[id]; !ok {
		return fmt.Errorf("policy not found")
	}
	delete(s.policies, id)
	return nil
}

func (s *MemoryStore) Clone(ctx context.Context, id int64) (Policy, error) {
	s.mu.RLock()
	source, ok := s.policies[id]
	s.mu.RUnlock()
	if !ok {
		return Policy{}, fmt.Errorf("policy not found")
	}
	source.ID = 0
	source.Name += " copy"
	return s.Create(ctx, source)
}

func (s *MemoryStore) Reorder(_ context.Context, orderedIDs []int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for index, id := range orderedIDs {
		policy, ok := s.policies[id]
		if !ok {
			continue
		}
		policy.Priority = (index + 1) * 10
		s.policies[id] = policy
	}
	return nil
}

func (s *MemoryStore) Evaluate(ctx context.Context, userOUDN string, userGroupDNs []string) (*Policy, error) {
	items, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	groupSet := map[string]struct{}{}
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
