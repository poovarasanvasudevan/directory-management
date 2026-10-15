package directory

import (
	"context"
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf16"

	"github.com/go-ldap/ldap/v3"
)

var DefaultAttributeAllowlist = []string{
	"cn", "displayName", "mail", "givenName", "sn", "telephoneNumber", "mobile",
	"title", "department", "description", "sAMAccountName", "uid",
}

var sensitiveAttributes = map[string]struct{}{
	"userpassword": {}, "unicodepwd": {}, "ntpwdhistory": {}, "lmPwdHistory": {},
	"dscorepropagationdata": {},
}

func NormalizeAllowlist(extra []string) map[string]struct{} {
	allowed := make(map[string]struct{}, len(DefaultAttributeAllowlist)+len(extra))
	for _, key := range DefaultAttributeAllowlist {
		allowed[strings.ToLower(key)] = struct{}{}
	}
	for _, key := range extra {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if _, sensitive := sensitiveAttributes[strings.ToLower(key)]; sensitive {
			continue
		}
		allowed[strings.ToLower(key)] = struct{}{}
	}
	return allowed
}

func FilterAttributes(attrs map[string]string, allowlist map[string]struct{}) (map[string]string, error) {
	if allowlist == nil {
		allowlist = NormalizeAllowlist(nil)
	}
	result := make(map[string]string, len(attrs))
	for key, value := range attrs {
		normalized := strings.ToLower(strings.TrimSpace(key))
		if normalized == "" {
			continue
		}
		if _, sensitive := sensitiveAttributes[normalized]; sensitive {
			return nil, fmt.Errorf("attribute %q cannot be written through the directory browser", key)
		}
		if _, ok := allowlist[normalized]; !ok {
			return nil, fmt.Errorf("attribute %q is not in the editable allowlist", key)
		}
		result[canonicalAttributeName(normalized)] = value
	}
	return result, nil
}

func canonicalAttributeName(lower string) string {
	for _, key := range DefaultAttributeAllowlist {
		if strings.EqualFold(key, lower) {
			return key
		}
	}
	return lower
}

func DNUnderBase(dn, base string) bool {
	dn = strings.TrimSpace(dn)
	base = strings.TrimSpace(base)
	if dn == "" || base == "" {
		return false
	}
	if strings.EqualFold(dn, base) {
		return true
	}
	suffix := "," + base
	return len(dn) > len(suffix) && strings.EqualFold(dn[len(dn)-len(suffix):], suffix)
}

func (s *LDAPService) requireWrite() error {
	if !s.cfg.WriteEnabled {
		return fmt.Errorf("directory is configured in read-only mode")
	}
	return nil
}

func (s *LDAPService) isActiveDirectory(conn *ldap.Conn) bool {
	req := ldap.NewSearchRequest("", ldap.ScopeBaseObject, ldap.NeverDerefAliases, 1, 5, false, "(objectClass=*)",
		[]string{"forestFunctionality", "domainFunctionality", "dsServiceName", "vendorName"}, nil)
	result, err := conn.Search(req)
	if err != nil || len(result.Entries) == 0 {
		return false
	}
	entry := result.Entries[0]
	return entry.GetAttributeValue("forestFunctionality") != "" ||
		entry.GetAttributeValue("domainFunctionality") != "" ||
		entry.GetAttributeValue("dsServiceName") != ""
}

func (s *LDAPService) ListChildren(ctx context.Context, parentDN string, limit int) ([]DirectoryNode, error) {
	conn, err := s.connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	parentDN = strings.TrimSpace(parentDN)
	if parentDN == "" {
		parentDN = s.cfg.BaseDN
	}
	if !DNUnderBase(parentDN, s.cfg.BaseDN) {
		return nil, fmt.Errorf("parent DN is outside the configured base DN")
	}
	if limit < 1 || limit > 500 {
		limit = 200
	}
	req := ldap.NewSearchRequest(parentDN, ldap.ScopeSingleLevel, ldap.NeverDerefAliases, limit, 15, false, "(objectClass=*)",
		[]string{"objectClass", "cn", "ou", "name", "sAMAccountName", "uid", "displayName"}, nil)
	result, err := searchAllowingSizeLimit(conn, req)
	if err != nil {
		return nil, fmt.Errorf("browse directory: %w", err)
	}
	nodes := make([]DirectoryNode, 0, len(result.Entries))
	for _, entry := range result.Entries {
		kind := classifyEntry(entry.GetAttributeValues("objectClass"))
		name := entryDisplayName(entry)
		hasChildren := kind == NodeOU || kind == NodeContainer
		nodes = append(nodes, DirectoryNode{
			Name: name, DistinguishedName: entry.DN, Kind: kind, HasChildren: hasChildren,
		})
	}
	return nodes, nil
}

func (s *LDAPService) GetEntry(ctx context.Context, dn string) (DirectoryEntry, error) {
	conn, err := s.connect(ctx)
	if err != nil {
		return DirectoryEntry{}, err
	}
	defer conn.Close()
	dn = strings.TrimSpace(dn)
	if !DNUnderBase(dn, s.cfg.BaseDN) {
		return DirectoryEntry{}, fmt.Errorf("entry DN is outside the configured base DN")
	}
	attrs := append([]string{"objectClass", "userAccountControl"}, DefaultAttributeAllowlist...)
	req := ldap.NewSearchRequest(dn, ldap.ScopeBaseObject, ldap.NeverDerefAliases, 1, 10, false, "(objectClass=*)", attrs, nil)
	result, err := conn.Search(req)
	if err != nil || len(result.Entries) == 0 {
		return DirectoryEntry{}, fmt.Errorf("directory entry was not found")
	}
	return entryFromLDAP(result.Entries[0]), nil
}

func (s *LDAPService) CreateUser(ctx context.Context, input CreateUserInput) (DirectoryEntry, error) {
	if err := s.requireWrite(); err != nil {
		return DirectoryEntry{}, err
	}
	name := strings.TrimSpace(input.Name)
	parent := strings.TrimSpace(input.ParentDN)
	if name == "" || parent == "" {
		return DirectoryEntry{}, fmt.Errorf("parent DN and name are required")
	}
	if !DNUnderBase(parent, s.cfg.BaseDN) {
		return DirectoryEntry{}, fmt.Errorf("parent DN is outside the configured base DN")
	}
	attrs, err := FilterAttributes(input.Attributes, nil)
	if err != nil {
		return DirectoryEntry{}, err
	}
	conn, err := s.connect(ctx)
	if err != nil {
		return DirectoryEntry{}, err
	}
	defer conn.Close()
	ad := s.isActiveDirectory(conn)
	var dn string
	var add *ldap.AddRequest
	if ad {
		dn = "CN=" + ldap.EscapeDN(name) + "," + parent
		add = ldap.NewAddRequest(dn, nil)
		add.Attribute("objectClass", []string{"top", "person", "organizationalPerson", "user"})
		add.Attribute("cn", []string{name})
		sam := attrs["sAMAccountName"]
		if sam == "" {
			sam = sanitizeAccountName(name)
		}
		add.Attribute("sAMAccountName", []string{sam})
		if display := firstNonEmpty(attrs["displayName"], name); display != "" {
			add.Attribute("displayName", []string{display})
		}
		for _, key := range []string{"mail", "givenName", "sn", "telephoneNumber", "mobile", "title", "department", "description"} {
			if value := attrs[key]; value != "" {
				add.Attribute(key, []string{value})
			}
		}
		if attrs["sn"] == "" {
			add.Attribute("sn", []string{name})
		}
		// 512 = NORMAL_ACCOUNT; leave enabled when password will be set, else disabled until password exists.
		uac := "512"
		if strings.TrimSpace(input.Password) == "" {
			uac = "514" // NORMAL_ACCOUNT | ACCOUNTDISABLE
		}
		add.Attribute("userAccountControl", []string{uac})
	} else {
		uid := attrs["uid"]
		if uid == "" {
			uid = sanitizeAccountName(name)
		}
		dn = "uid=" + ldap.EscapeDN(uid) + "," + parent
		add = ldap.NewAddRequest(dn, nil)
		add.Attribute("objectClass", []string{"top", "person", "organizationalPerson", "inetOrgPerson"})
		add.Attribute("uid", []string{uid})
		add.Attribute("cn", []string{firstNonEmpty(attrs["cn"], name)})
		add.Attribute("sn", []string{firstNonEmpty(attrs["sn"], name)})
		for _, key := range []string{"displayName", "mail", "givenName", "telephoneNumber", "mobile", "title", "department", "description"} {
			if value := attrs[key]; value != "" {
				add.Attribute(key, []string{value})
			}
		}
		if password := strings.TrimSpace(input.Password); password != "" {
			add.Attribute("userPassword", []string{password})
		}
	}
	if err := conn.Add(add); err != nil {
		return DirectoryEntry{}, fmt.Errorf("create directory user: %w", err)
	}
	if ad && strings.TrimSpace(input.Password) != "" {
		if err := s.setADPassword(conn, dn, input.Password); err != nil {
			return DirectoryEntry{}, fmt.Errorf("user created but password could not be set: %w", err)
		}
	}
	return s.GetEntry(ctx, dn)
}

func (s *LDAPService) CreateGroup(ctx context.Context, input CreateGroupInput) (DirectoryEntry, error) {
	if err := s.requireWrite(); err != nil {
		return DirectoryEntry{}, err
	}
	name := strings.TrimSpace(input.Name)
	parent := strings.TrimSpace(input.ParentDN)
	if name == "" || parent == "" {
		return DirectoryEntry{}, fmt.Errorf("parent DN and name are required")
	}
	if !DNUnderBase(parent, s.cfg.BaseDN) {
		return DirectoryEntry{}, fmt.Errorf("parent DN is outside the configured base DN")
	}
	conn, err := s.connect(ctx)
	if err != nil {
		return DirectoryEntry{}, err
	}
	defer conn.Close()
	dn := "CN=" + ldap.EscapeDN(name) + "," + parent
	add := ldap.NewAddRequest(dn, nil)
	if s.isActiveDirectory(conn) {
		add.Attribute("objectClass", []string{"top", "group"})
		add.Attribute("cn", []string{name})
		add.Attribute("sAMAccountName", []string{sanitizeAccountName(name)})
		add.Attribute("groupType", []string{"-2147483646"}) // global security group
	} else {
		add.Attribute("objectClass", []string{"top", "groupOfNames"})
		add.Attribute("cn", []string{name})
		// groupOfNames requires at least one member; use the bind DN as bootstrap member.
		add.Attribute("member", []string{s.cfg.BindDN})
	}
	if desc := strings.TrimSpace(input.Description); desc != "" {
		add.Attribute("description", []string{desc})
	}
	if err := conn.Add(add); err != nil {
		return DirectoryEntry{}, fmt.Errorf("create directory group: %w", err)
	}
	return s.GetEntry(ctx, dn)
}

func (s *LDAPService) UpdateAttributes(ctx context.Context, dn string, attrs map[string]string) (DirectoryEntry, error) {
	if err := s.requireWrite(); err != nil {
		return DirectoryEntry{}, err
	}
	dn = strings.TrimSpace(dn)
	if !DNUnderBase(dn, s.cfg.BaseDN) {
		return DirectoryEntry{}, fmt.Errorf("entry DN is outside the configured base DN")
	}
	filtered, err := FilterAttributes(attrs, nil)
	if err != nil {
		return DirectoryEntry{}, err
	}
	if len(filtered) == 0 {
		return s.GetEntry(ctx, dn)
	}
	conn, err := s.connect(ctx)
	if err != nil {
		return DirectoryEntry{}, err
	}
	defer conn.Close()
	mod := ldap.NewModifyRequest(dn, nil)
	for key, value := range filtered {
		if strings.TrimSpace(value) == "" {
			mod.Delete(key, nil)
			continue
		}
		mod.Replace(key, []string{value})
	}
	if err := conn.Modify(mod); err != nil {
		return DirectoryEntry{}, fmt.Errorf("update directory attributes: %w", err)
	}
	return s.GetEntry(ctx, dn)
}

func (s *LDAPService) SetUserEnabled(ctx context.Context, dn string, enabled bool) error {
	if err := s.requireWrite(); err != nil {
		return err
	}
	dn = strings.TrimSpace(dn)
	if !DNUnderBase(dn, s.cfg.BaseDN) {
		return fmt.Errorf("entry DN is outside the configured base DN")
	}
	conn, err := s.connect(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if !s.isActiveDirectory(conn) {
		// OpenLDAP has no universal enable flag; treat as no-op success for browser UX.
		return nil
	}
	req := ldap.NewSearchRequest(dn, ldap.ScopeBaseObject, ldap.NeverDerefAliases, 1, 10, false, "(objectClass=*)", []string{"userAccountControl"}, nil)
	result, err := conn.Search(req)
	if err != nil || len(result.Entries) == 0 {
		return fmt.Errorf("directory entry was not found")
	}
	uac, _ := strconv.ParseInt(result.Entries[0].GetAttributeValue("userAccountControl"), 10, 64)
	if uac == 0 {
		uac = 512
	}
	const accountDisable = 2
	if enabled {
		uac &^= accountDisable
	} else {
		uac |= accountDisable
	}
	mod := ldap.NewModifyRequest(dn, nil)
	mod.Replace("userAccountControl", []string{strconv.FormatInt(uac, 10)})
	if err := conn.Modify(mod); err != nil {
		return fmt.Errorf("update account enabled state: %w", err)
	}
	return nil
}

func (s *LDAPService) ListMembers(ctx context.Context, groupDN string) ([]MemberRef, error) {
	conn, err := s.connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	groupDN = strings.TrimSpace(groupDN)
	if !DNUnderBase(groupDN, s.cfg.BaseDN) {
		return nil, fmt.Errorf("group DN is outside the configured base DN")
	}
	req := ldap.NewSearchRequest(groupDN, ldap.ScopeBaseObject, ldap.NeverDerefAliases, 1, 10, false, "(objectClass=*)",
		[]string{"member", "uniqueMember", "memberUid"}, nil)
	result, err := conn.Search(req)
	if err != nil || len(result.Entries) == 0 {
		return nil, fmt.Errorf("directory group was not found")
	}
	entry := result.Entries[0]
	dns := append(entry.GetAttributeValues("member"), entry.GetAttributeValues("uniqueMember")...)
	refs := make([]MemberRef, 0, len(dns))
	for _, memberDN := range dns {
		refs = append(refs, MemberRef{DistinguishedName: memberDN, Name: rdnValue(memberDN), Kind: "entry"})
	}
	return refs, nil
}

func (s *LDAPService) ListMemberOf(ctx context.Context, userDN string) ([]MemberRef, error) {
	conn, err := s.connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	userDN = strings.TrimSpace(userDN)
	if !DNUnderBase(userDN, s.cfg.BaseDN) {
		return nil, fmt.Errorf("user DN is outside the configured base DN")
	}
	req := ldap.NewSearchRequest(userDN, ldap.ScopeBaseObject, ldap.NeverDerefAliases, 1, 10, false, "(objectClass=*)", []string{"memberOf"}, nil)
	result, err := conn.Search(req)
	if err != nil || len(result.Entries) == 0 {
		return nil, fmt.Errorf("directory entry was not found")
	}
	memberOf := result.Entries[0].GetAttributeValues("memberOf")
	if len(memberOf) > 0 {
		refs := make([]MemberRef, 0, len(memberOf))
		for _, groupDN := range memberOf {
			refs = append(refs, MemberRef{DistinguishedName: groupDN, Name: rdnValue(groupDN), Kind: "group"})
		}
		return refs, nil
	}
	// OpenLDAP often lacks memberOf; reverse-search groups.
	base := s.cfg.GroupSearchBase
	if base == "" {
		base = s.cfg.BaseDN
	}
	escaped := ldap.EscapeFilter(userDN)
	filter := fmt.Sprintf("(&(|(objectClass=group)(objectClass=groupOfNames)(objectClass=groupOfUniqueNames))(|(member=%s)(uniqueMember=%s)))", escaped, escaped)
	search := ldap.NewSearchRequest(base, ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 200, 15, false, filter, []string{"cn"}, nil)
	groups, err := searchAllowingSizeLimit(conn, search)
	if err != nil {
		return nil, fmt.Errorf("search group membership: %w", err)
	}
	refs := make([]MemberRef, 0, len(groups.Entries))
	for _, entry := range groups.Entries {
		refs = append(refs, MemberRef{DistinguishedName: entry.DN, Name: entryDisplayName(entry), Kind: "group"})
	}
	return refs, nil
}

func (s *LDAPService) AddMember(ctx context.Context, groupDN, memberDN string) error {
	if err := s.requireWrite(); err != nil {
		return err
	}
	return s.modifyMembership(ctx, groupDN, memberDN, true)
}

func (s *LDAPService) RemoveMember(ctx context.Context, groupDN, memberDN string) error {
	if err := s.requireWrite(); err != nil {
		return err
	}
	return s.modifyMembership(ctx, groupDN, memberDN, false)
}

func (s *LDAPService) modifyMembership(ctx context.Context, groupDN, memberDN string, add bool) error {
	groupDN = strings.TrimSpace(groupDN)
	memberDN = strings.TrimSpace(memberDN)
	if !DNUnderBase(groupDN, s.cfg.BaseDN) || !DNUnderBase(memberDN, s.cfg.BaseDN) {
		return fmt.Errorf("group or member DN is outside the configured base DN")
	}
	conn, err := s.connect(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	attr := "member"
	req := ldap.NewSearchRequest(groupDN, ldap.ScopeBaseObject, ldap.NeverDerefAliases, 1, 10, false, "(objectClass=*)",
		[]string{"objectClass", "member", "uniqueMember"}, nil)
	result, err := conn.Search(req)
	if err != nil || len(result.Entries) == 0 {
		return fmt.Errorf("directory group was not found")
	}
	classes := strings.ToLower(strings.Join(result.Entries[0].GetAttributeValues("objectClass"), " "))
	if strings.Contains(classes, "groupofuniquenames") {
		attr = "uniqueMember"
	}
	mod := ldap.NewModifyRequest(groupDN, nil)
	if add {
		mod.Add(attr, []string{memberDN})
	} else {
		mod.Delete(attr, []string{memberDN})
	}
	if err := conn.Modify(mod); err != nil {
		action := "add"
		if !add {
			action = "remove"
		}
		return fmt.Errorf("%s group member: %w", action, err)
	}
	return nil
}

func (s *LDAPService) setADPassword(conn *ldap.Conn, dn, password string) error {
	if strings.EqualFold(s.cfg.Security, "ldap") {
		return fmt.Errorf("password set requires LDAPS or StartTLS")
	}
	encoded := encodeUnicodePwd(password)
	mod := ldap.NewModifyRequest(dn, nil)
	mod.Replace("unicodePwd", []string{encoded})
	mod.Replace("userAccountControl", []string{"512"})
	return conn.Modify(mod)
}

func encodeUnicodePwd(password string) string {
	encoded := utf16.Encode([]rune(`"` + password + `"`))
	buffer := make([]byte, len(encoded)*2)
	for index, value := range encoded {
		binary.LittleEndian.PutUint16(buffer[index*2:], value)
	}
	return string(buffer)
}

func classifyEntry(objectClasses []string) NodeKind {
	classes := make(map[string]struct{}, len(objectClasses))
	for _, class := range objectClasses {
		classes[strings.ToLower(strings.TrimSpace(class))] = struct{}{}
	}
	has := func(name string) bool {
		_, ok := classes[name]
		return ok
	}
	// Users before organization/container: organizationalPerson must not match "organization".
	switch {
	case has("organizationalunit"):
		return NodeOU
	case has("inetorgperson") || has("user") || has("organizationalperson") || has("person") || has("posixaccount"):
		return NodeUser
	case has("groupofnames") || has("groupofuniquenames") || has("posixgroup") || has("group"):
		return NodeGroup
	case has("container") || has("builtindomain") || has("organization") || has("domaindns") || has("domain"):
		return NodeContainer
	default:
		return NodeOther
	}
}

func entryDisplayName(entry *ldap.Entry) string {
	for _, key := range []string{"displayName", "cn", "name", "ou", "sAMAccountName", "uid"} {
		if value := entry.GetAttributeValue(key); value != "" {
			return value
		}
	}
	return rdnValue(entry.DN)
}

func entryFromLDAP(entry *ldap.Entry) DirectoryEntry {
	kind := classifyEntry(entry.GetAttributeValues("objectClass"))
	attrs := map[string]string{}
	allow := NormalizeAllowlist(nil)
	for _, attr := range entry.Attributes {
		lower := strings.ToLower(attr.Name)
		if _, sensitive := sensitiveAttributes[lower]; sensitive {
			continue
		}
		if _, ok := allow[lower]; !ok && lower != "objectclass" {
			continue
		}
		if lower == "objectclass" {
			continue
		}
		if len(attr.Values) > 0 {
			attrs[canonicalAttributeName(lower)] = attr.Values[0]
		}
	}
	result := DirectoryEntry{
		DistinguishedName: entry.DN,
		Kind:              kind,
		Name:              entryDisplayName(entry),
		Attributes:        attrs,
	}
	if kind == NodeUser {
		enabled := true
		if raw := entry.GetAttributeValue("userAccountControl"); raw != "" {
			uac, _ := strconv.ParseInt(raw, 10, 64)
			enabled = uac&2 == 0
		}
		result.Enabled = &enabled
	}
	return result
}

func rdnValue(dn string) string {
	parts := strings.SplitN(dn, ",", 2)
	if len(parts) == 0 {
		return dn
	}
	rdn := strings.TrimSpace(parts[0])
	if idx := strings.Index(rdn, "="); idx >= 0 {
		return rdn[idx+1:]
	}
	return rdn
}

func sanitizeAccountName(name string) string {
	replacer := strings.NewReplacer(" ", ".", ",", "", "/", "", "\\", "", "+", "")
	value := strings.ToLower(replacer.Replace(strings.TrimSpace(name)))
	if len(value) > 64 {
		value = value[:64]
	}
	if value == "" {
		return "user"
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
