package directory

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

type mockEntry struct {
	dn          string
	kind        NodeKind
	name        string
	attrs       map[string]string
	enabled     bool
	members     []string
	objectClass []string
}

type MockService struct {
	mu      sync.RWMutex
	baseDN  string
	entries map[string]*mockEntry
	users   []User // kept for SearchUsers compatibility
}

func NewMockService() *MockService {
	base := "DC=example,DC=com"
	s := &MockService{
		baseDN:  base,
		entries: map[string]*mockEntry{},
	}
	for _, ou := range []struct{ name, dn string }{
		{"Employees", "OU=Employees,DC=example,DC=com"},
		{"Contractors", "OU=Contractors,DC=example,DC=com"},
		{"Service Accounts", "OU=Service Accounts,DC=example,DC=com"},
		{"Groups", "OU=Groups,DC=example,DC=com"},
	} {
		s.entries[strings.ToLower(ou.dn)] = &mockEntry{
			dn: ou.dn, kind: NodeOU, name: ou.name,
			attrs:       map[string]string{"ou": ou.name},
			objectClass: []string{"top", "organizationalUnit"},
		}
	}
	seedUsers := []User{
		{ID: "aisha.khan", DisplayName: "Aisha Khan", Email: "aisha.khan@example.com", DistinguishedName: "CN=Aisha Khan,OU=Employees,DC=example,DC=com", OrganizationalUnit: "Employees", Enabled: true, Locked: true, PasswordExpired: false, LastLogon: "2026-10-14T07:45:00Z"},
		{ID: "marcus.chen", DisplayName: "Marcus Chen", Email: "marcus.chen@example.com", DistinguishedName: "CN=Marcus Chen,OU=Employees,DC=example,DC=com", OrganizationalUnit: "Employees", Enabled: true, Locked: false, PasswordExpired: true, LastLogon: "2026-10-13T16:20:00Z"},
		{ID: "priya.nair", DisplayName: "Priya Nair", Email: "priya.nair@example.com", DistinguishedName: "CN=Priya Nair,OU=Contractors,DC=example,DC=com", OrganizationalUnit: "Contractors", Enabled: false, Locked: false, PasswordExpired: false, LastLogon: "2026-09-28T10:05:00Z"},
	}
	s.users = append([]User{}, seedUsers...)
	for _, user := range seedUsers {
		s.entries[strings.ToLower(user.DistinguishedName)] = &mockEntry{
			dn: user.DistinguishedName, kind: NodeUser, name: user.DisplayName, enabled: user.Enabled,
			attrs: map[string]string{
				"cn": user.DisplayName, "displayName": user.DisplayName, "mail": user.Email,
				"sAMAccountName": user.ID, "uid": user.ID,
			},
			objectClass: []string{"top", "person", "organizationalPerson", "user"},
		}
	}
	for _, group := range []struct {
		name, dn, sam string
		members       []string
	}{
		{"Self-service users", "CN=Self-service users,OU=Groups,DC=example,DC=com", "ss-users", []string{"CN=Aisha Khan,OU=Employees,DC=example,DC=com"}},
		{"Remote workforce", "CN=Remote workforce,OU=Groups,DC=example,DC=com", "remote", nil},
		{"Endpoint MFA", "CN=Endpoint MFA,OU=Groups,DC=example,DC=com", "endpoint-mfa", []string{"CN=Marcus Chen,OU=Employees,DC=example,DC=com"}},
	} {
		s.entries[strings.ToLower(group.dn)] = &mockEntry{
			dn: group.dn, kind: NodeGroup, name: group.name, members: append([]string{}, group.members...),
			attrs:       map[string]string{"cn": group.name, "sAMAccountName": group.sam, "description": group.name},
			objectClass: []string{"top", "group"},
		}
	}
	return s
}

func (*MockService) TestConnection(context.Context) error { return nil }

func (s *MockService) OrganizationalUnits(context.Context) ([]OrganizationalUnit, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	units := make([]OrganizationalUnit, 0)
	for _, entry := range s.entries {
		if entry.kind == NodeOU {
			units = append(units, OrganizationalUnit{Name: entry.name, DistinguishedName: entry.dn})
		}
	}
	return units, nil
}

func (s *MockService) SearchGroups(_ context.Context, query string, limit int) ([]Group, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	query = strings.ToLower(strings.TrimSpace(query))
	result := make([]Group, 0)
	for _, entry := range s.entries {
		if entry.kind != NodeGroup {
			continue
		}
		sam := entry.attrs["sAMAccountName"]
		if query == "" || strings.Contains(strings.ToLower(entry.name+" "+sam), query) {
			result = append(result, Group{Name: entry.name, DistinguishedName: entry.dn, SamAccountName: sam})
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

func (s *MockService) SearchUsers(_ context.Context, query string, limit int) ([]User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	query = strings.ToLower(strings.TrimSpace(query))
	result := make([]User, 0, len(s.users))
	for _, user := range s.users {
		candidate := strings.ToLower(user.ID + " " + user.DisplayName + " " + user.Email + " " + user.OrganizationalUnit)
		if query == "" || strings.Contains(candidate, query) {
			result = append(result, user)
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

func (s *MockService) ResetPassword(_ context.Context, userID, _ string, forceChange bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for index := range s.users {
		if s.users[index].ID == userID {
			s.users[index].PasswordExpired = forceChange
			return nil
		}
	}
	return fmt.Errorf("directory user was not found")
}

func (s *MockService) UnlockAccount(_ context.Context, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for index := range s.users {
		if s.users[index].ID == userID {
			s.users[index].Locked = false
			return nil
		}
	}
	return fmt.Errorf("directory user was not found")
}

func (s *MockService) ListChildren(_ context.Context, parentDN string, limit int) ([]DirectoryNode, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	parentDN = strings.TrimSpace(parentDN)
	if parentDN == "" {
		parentDN = s.baseDN
	}
	if !DNUnderBase(parentDN, s.baseDN) {
		return nil, fmt.Errorf("parent DN is outside the configured base DN")
	}
	if limit < 1 {
		limit = 200
	}
	parentLower := strings.ToLower(parentDN)
	nodes := make([]DirectoryNode, 0)
	for _, entry := range s.entries {
		suffix := "," + parentLower
		dnLower := strings.ToLower(entry.dn)
		if !strings.HasSuffix(dnLower, suffix) {
			continue
		}
		rest := strings.TrimSuffix(dnLower, suffix)
		if strings.Contains(rest, ",") {
			continue // not a direct child
		}
		nodes = append(nodes, DirectoryNode{
			Name: entry.name, DistinguishedName: entry.dn, Kind: entry.kind,
			HasChildren: entry.kind == NodeOU || entry.kind == NodeContainer,
		})
		if len(nodes) >= limit {
			break
		}
	}
	return nodes, nil
}

func (s *MockService) GetEntry(_ context.Context, dn string) (DirectoryEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.entries[strings.ToLower(strings.TrimSpace(dn))]
	if !ok {
		return DirectoryEntry{}, fmt.Errorf("directory entry was not found")
	}
	return s.toEntry(entry), nil
}

func (s *MockService) CreateUser(_ context.Context, input CreateUserInput) (DirectoryEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	name := strings.TrimSpace(input.Name)
	parent := strings.TrimSpace(input.ParentDN)
	if name == "" || parent == "" {
		return DirectoryEntry{}, fmt.Errorf("parent DN and name are required")
	}
	if !DNUnderBase(parent, s.baseDN) {
		return DirectoryEntry{}, fmt.Errorf("parent DN is outside the configured base DN")
	}
	attrs, err := FilterAttributes(input.Attributes, nil)
	if err != nil {
		return DirectoryEntry{}, err
	}
	dn := "CN=" + name + "," + parent
	if _, exists := s.entries[strings.ToLower(dn)]; exists {
		return DirectoryEntry{}, fmt.Errorf("directory user already exists")
	}
	id := attrs["sAMAccountName"]
	if id == "" {
		id = sanitizeAccountName(name)
	}
	if attrs["displayName"] == "" {
		attrs["displayName"] = name
	}
	if attrs["cn"] == "" {
		attrs["cn"] = name
	}
	attrs["sAMAccountName"] = id
	attrs["uid"] = id
	enabled := true
	entry := &mockEntry{
		dn: dn, kind: NodeUser, name: firstNonEmpty(attrs["displayName"], name), enabled: enabled, attrs: attrs,
		objectClass: []string{"top", "person", "organizationalPerson", "user"},
	}
	s.entries[strings.ToLower(dn)] = entry
	s.users = append(s.users, User{
		ID: id, DisplayName: entry.name, Email: attrs["mail"], DistinguishedName: dn,
		OrganizationalUnit: organizationalUnit(dn), Enabled: enabled,
	})
	return s.toEntry(entry), nil
}

func (s *MockService) CreateGroup(_ context.Context, input CreateGroupInput) (DirectoryEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	name := strings.TrimSpace(input.Name)
	parent := strings.TrimSpace(input.ParentDN)
	if name == "" || parent == "" {
		return DirectoryEntry{}, fmt.Errorf("parent DN and name are required")
	}
	if !DNUnderBase(parent, s.baseDN) {
		return DirectoryEntry{}, fmt.Errorf("parent DN is outside the configured base DN")
	}
	dn := "CN=" + name + "," + parent
	if _, exists := s.entries[strings.ToLower(dn)]; exists {
		return DirectoryEntry{}, fmt.Errorf("directory group already exists")
	}
	attrs := map[string]string{"cn": name, "sAMAccountName": sanitizeAccountName(name)}
	if desc := strings.TrimSpace(input.Description); desc != "" {
		attrs["description"] = desc
	}
	entry := &mockEntry{
		dn: dn, kind: NodeGroup, name: name, attrs: attrs, members: []string{},
		objectClass: []string{"top", "group"},
	}
	s.entries[strings.ToLower(dn)] = entry
	return s.toEntry(entry), nil
}

func (s *MockService) UpdateAttributes(_ context.Context, dn string, attrs map[string]string) (DirectoryEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.entries[strings.ToLower(strings.TrimSpace(dn))]
	if !ok {
		return DirectoryEntry{}, fmt.Errorf("directory entry was not found")
	}
	filtered, err := FilterAttributes(attrs, nil)
	if err != nil {
		return DirectoryEntry{}, err
	}
	for key, value := range filtered {
		if strings.TrimSpace(value) == "" {
			delete(entry.attrs, key)
			continue
		}
		entry.attrs[key] = value
	}
	if display := entry.attrs["displayName"]; display != "" {
		entry.name = display
	} else if cn := entry.attrs["cn"]; cn != "" {
		entry.name = cn
	}
	s.syncUserRow(entry)
	return s.toEntry(entry), nil
}

func (s *MockService) SetUserEnabled(_ context.Context, dn string, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.entries[strings.ToLower(strings.TrimSpace(dn))]
	if !ok || entry.kind != NodeUser {
		return fmt.Errorf("directory user was not found")
	}
	entry.enabled = enabled
	s.syncUserRow(entry)
	return nil
}

func (s *MockService) ListMembers(_ context.Context, groupDN string) ([]MemberRef, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.entries[strings.ToLower(strings.TrimSpace(groupDN))]
	if !ok || entry.kind != NodeGroup {
		return nil, fmt.Errorf("directory group was not found")
	}
	refs := make([]MemberRef, 0, len(entry.members))
	for _, memberDN := range entry.members {
		name := rdnValue(memberDN)
		if member, ok := s.entries[strings.ToLower(memberDN)]; ok {
			name = member.name
		}
		refs = append(refs, MemberRef{DistinguishedName: memberDN, Name: name, Kind: "user"})
	}
	return refs, nil
}

func (s *MockService) ListMemberOf(_ context.Context, userDN string) ([]MemberRef, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	userDN = strings.TrimSpace(userDN)
	if _, ok := s.entries[strings.ToLower(userDN)]; !ok {
		return nil, fmt.Errorf("directory entry was not found")
	}
	refs := make([]MemberRef, 0)
	for _, entry := range s.entries {
		if entry.kind != NodeGroup {
			continue
		}
		for _, member := range entry.members {
			if strings.EqualFold(member, userDN) {
				refs = append(refs, MemberRef{DistinguishedName: entry.dn, Name: entry.name, Kind: "group"})
				break
			}
		}
	}
	return refs, nil
}

func (s *MockService) AddMember(_ context.Context, groupDN, memberDN string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	group, ok := s.entries[strings.ToLower(strings.TrimSpace(groupDN))]
	if !ok || group.kind != NodeGroup {
		return fmt.Errorf("directory group was not found")
	}
	memberDN = strings.TrimSpace(memberDN)
	if _, ok := s.entries[strings.ToLower(memberDN)]; !ok {
		return fmt.Errorf("member entry was not found")
	}
	for _, existing := range group.members {
		if strings.EqualFold(existing, memberDN) {
			return nil
		}
	}
	group.members = append(group.members, memberDN)
	return nil
}

func (s *MockService) RemoveMember(_ context.Context, groupDN, memberDN string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	group, ok := s.entries[strings.ToLower(strings.TrimSpace(groupDN))]
	if !ok || group.kind != NodeGroup {
		return fmt.Errorf("directory group was not found")
	}
	memberDN = strings.TrimSpace(memberDN)
	next := group.members[:0]
	for _, existing := range group.members {
		if !strings.EqualFold(existing, memberDN) {
			next = append(next, existing)
		}
	}
	group.members = next
	return nil
}

func (s *MockService) toEntry(entry *mockEntry) DirectoryEntry {
	attrs := make(map[string]string, len(entry.attrs))
	for key, value := range entry.attrs {
		attrs[key] = value
	}
	result := DirectoryEntry{
		DistinguishedName: entry.dn,
		Kind:              entry.kind,
		Name:              entry.name,
		Attributes:        attrs,
	}
	if entry.kind == NodeUser {
		enabled := entry.enabled
		result.Enabled = &enabled
	}
	return result
}

func (s *MockService) syncUserRow(entry *mockEntry) {
	if entry.kind != NodeUser {
		return
	}
	for index := range s.users {
		if strings.EqualFold(s.users[index].DistinguishedName, entry.dn) {
			s.users[index].DisplayName = entry.name
			s.users[index].Email = entry.attrs["mail"]
			s.users[index].Enabled = entry.enabled
			if id := entry.attrs["sAMAccountName"]; id != "" {
				s.users[index].ID = id
			}
			return
		}
	}
}
