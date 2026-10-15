package directory

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"

	"directory-management/internal/config"
	"github.com/go-ldap/ldap/v3"
)

type OrganizationalUnit struct {
	Name              string `json:"name"`
	DistinguishedName string `json:"distinguishedName"`
}

type User struct {
	ID                 string `json:"id"`
	DisplayName        string `json:"displayName"`
	Email              string `json:"email"`
	DistinguishedName  string `json:"distinguishedName"`
	OrganizationalUnit string `json:"organizationalUnit"`
	Enabled            bool   `json:"enabled"`
	Locked             bool   `json:"locked"`
	PasswordExpired    bool   `json:"passwordExpired"`
	LastLogon          string `json:"lastLogon,omitempty"`
}

type Group struct {
	Name              string `json:"name"`
	DistinguishedName string `json:"distinguishedName"`
	SamAccountName    string `json:"samAccountName,omitempty"`
}

type NodeKind string

const (
	NodeContainer NodeKind = "container"
	NodeOU        NodeKind = "ou"
	NodeUser      NodeKind = "user"
	NodeGroup     NodeKind = "group"
	NodeOther     NodeKind = "other"
)

type DirectoryNode struct {
	Name              string   `json:"name"`
	DistinguishedName string   `json:"distinguishedName"`
	Kind              NodeKind `json:"kind"`
	HasChildren       bool     `json:"hasChildren"`
}

type DirectoryEntry struct {
	DistinguishedName string            `json:"distinguishedName"`
	Kind              NodeKind          `json:"kind"`
	Name              string            `json:"name"`
	Attributes        map[string]string `json:"attributes"`
	Enabled           *bool             `json:"enabled,omitempty"`
}

type CreateUserInput struct {
	ParentDN   string
	Name       string
	Attributes map[string]string
	Password   string
}

type CreateGroupInput struct {
	ParentDN    string
	Name        string
	Description string
}

type MemberRef struct {
	DistinguishedName string `json:"distinguishedName"`
	Name              string `json:"name"`
	Kind              string `json:"kind,omitempty"`
}

type Service interface {
	TestConnection(context.Context) error
	OrganizationalUnits(context.Context) ([]OrganizationalUnit, error)
	SearchGroups(context.Context, string, int) ([]Group, error)
	SearchUsers(context.Context, string, int) ([]User, error)
	ResetPassword(context.Context, string, string, bool) error
	UnlockAccount(context.Context, string) error
	ListChildren(context.Context, string, int) ([]DirectoryNode, error)
	GetEntry(context.Context, string) (DirectoryEntry, error)
	CreateUser(context.Context, CreateUserInput) (DirectoryEntry, error)
	CreateGroup(context.Context, CreateGroupInput) (DirectoryEntry, error)
	UpdateAttributes(context.Context, string, map[string]string) (DirectoryEntry, error)
	SetUserEnabled(context.Context, string, bool) error
	ListMembers(context.Context, string) ([]MemberRef, error)
	ListMemberOf(context.Context, string) ([]MemberRef, error)
	AddMember(context.Context, string, string) error
	RemoveMember(context.Context, string, string) error
}
type LDAPService struct{ cfg config.ActiveDirectoryConfig }

func NewLDAPService(cfg config.ActiveDirectoryConfig) *LDAPService { return &LDAPService{cfg: cfg} }

// tlsConfig builds the client TLS settings. When SkipTLSVerify is set (testing
// only), accept self-signed and non-RFC-compliant AD certificates.
func (s *LDAPService) tlsConfig() *tls.Config {
	cfg := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: s.cfg.SkipTLSVerify, // #nosec G402 -- explicit opt-in for lab/self-signed AD.
	}
	if !s.cfg.SkipTLSVerify {
		cfg.ServerName = s.cfg.Host
	} else {
		// Still send SNI for AD, but never fail handshake on cert content.
		cfg.ServerName = s.cfg.Host
		cfg.VerifyPeerCertificate = func([][]byte, [][]*x509.Certificate) error { return nil }
		cfg.VerifyConnection = func(tls.ConnectionState) error { return nil }
	}
	return cfg
}

func (s *LDAPService) connect(ctx context.Context) (*ldap.Conn, error) {
	if s.cfg.BindPassword == "" {
		return nil, fmt.Errorf("AD_BIND_PASSWORD is not set")
	}
	tlsConfig := s.tlsConfig()
	dialer := &net.Dialer{Timeout: s.cfg.ConnectTimeout}
	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)

	var conn *ldap.Conn
	var err error
	switch strings.ToLower(s.cfg.Security) {
	case "ldaps":
		// Dial TCP then wrap TLS ourselves so SkipTLSVerify fully controls verification.
		raw, dialErr := dialer.DialContext(ctx, "tcp", addr)
		if dialErr != nil {
			return nil, fmt.Errorf("connect to directory: %w", dialErr)
		}
		tlsConn := tls.Client(raw, tlsConfig)
		if handshakeErr := tlsConn.HandshakeContext(ctx); handshakeErr != nil {
			_ = raw.Close()
			return nil, fmt.Errorf("connect to directory: %w", handshakeErr)
		}
		conn = ldap.NewConn(tlsConn, true)
		conn.Start()
	default:
		conn, err = ldap.DialURL("ldap://"+addr, ldap.DialWithDialer(dialer), ldap.DialWithTLSConfig(tlsConfig))
		if err != nil {
			return nil, fmt.Errorf("connect to directory: %w", err)
		}
		if strings.EqualFold(s.cfg.Security, "starttls") {
			if err := conn.StartTLS(tlsConfig); err != nil {
				conn.Close()
				return nil, fmt.Errorf("start TLS: %w", err)
			}
		}
	}
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining > 0 {
			conn.SetTimeout(remaining)
		}
	}
	if err := conn.Bind(s.cfg.BindDN, s.cfg.BindPassword); err != nil {
		conn.Close()
		return nil, fmt.Errorf("bind to directory: %w", err)
	}
	return conn, nil
}
func (s *LDAPService) TestConnection(ctx context.Context) error {
	conn, err := s.connect(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	req := ldap.NewSearchRequest(s.cfg.BaseDN, ldap.ScopeBaseObject, ldap.NeverDerefAliases, 1, 5, false, "(objectClass=*)", []string{"defaultNamingContext"}, nil)
	if _, err := conn.Search(req); err != nil {
		return fmt.Errorf("read directory root: %w", err)
	}
	return nil
}
func (s *LDAPService) OrganizationalUnits(ctx context.Context) ([]OrganizationalUnit, error) {
	conn, err := s.connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	req := ldap.NewSearchRequest(s.cfg.BaseDN, ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 500, 15, false, "(objectClass=organizationalUnit)", []string{"name", "ou"}, nil)
	result, err := searchAllowingSizeLimit(conn, req)
	if err != nil {
		return nil, fmt.Errorf("search organizational units: %w", err)
	}
	units := make([]OrganizationalUnit, 0, len(result.Entries))
	for _, entry := range result.Entries {
		name := entry.GetAttributeValue("name")
		if name == "" {
			name = entry.GetAttributeValue("ou")
		}
		units = append(units, OrganizationalUnit{Name: name, DistinguishedName: entry.DN})
	}
	return units, nil
}

func (s *LDAPService) SearchGroups(ctx context.Context, query string, limit int) ([]Group, error) {
	conn, err := s.connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	if limit < 1 || limit > 200 {
		limit = 50
	}
	escaped := ldap.EscapeFilter(strings.TrimSpace(query))
	// Support AD group and OpenLDAP groupOfNames / groupOfUniqueNames / posixGroup.
	filter := "(|(objectClass=group)(objectClass=groupOfNames)(objectClass=groupOfUniqueNames)(objectClass=posixGroup))"
	if escaped != "" {
		filter = fmt.Sprintf("(&(|(objectClass=group)(objectClass=groupOfNames)(objectClass=groupOfUniqueNames)(objectClass=posixGroup))(|(cn=*%s*)(sAMAccountName=*%s*)))", escaped, escaped)
	}
	base := s.cfg.GroupSearchBase
	if base == "" {
		base = s.cfg.BaseDN
	}
	req := ldap.NewSearchRequest(base, ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, limit, 15, false, filter, []string{"cn", "sAMAccountName", "distinguishedName"}, nil)
	result, err := searchAllowingSizeLimit(conn, req)
	if err != nil {
		return nil, fmt.Errorf("search directory groups: %w", err)
	}
	groups := make([]Group, 0, len(result.Entries))
	for _, entry := range result.Entries {
		name := entry.GetAttributeValue("cn")
		if name == "" {
			name = entry.GetAttributeValue("sAMAccountName")
		}
		groups = append(groups, Group{Name: name, DistinguishedName: entry.DN, SamAccountName: entry.GetAttributeValue("sAMAccountName")})
	}
	return groups, nil
}

func (s *LDAPService) SearchUsers(ctx context.Context, query string, limit int) ([]User, error) {
	conn, err := s.connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	if limit < 1 || limit > 200 {
		limit = 50
	}
	escaped := ldap.EscapeFilter(strings.TrimSpace(query))
	// AD users use objectCategory/sAMAccountName; OpenLDAP test data uses inetOrgPerson/uid.
	filter := "(|(&(objectCategory=person)(objectClass=user))(objectClass=inetOrgPerson))"
	if escaped != "" {
		filter = fmt.Sprintf("(&(|(&(objectCategory=person)(objectClass=user))(objectClass=inetOrgPerson))(|(sAMAccountName=*%s*)(uid=*%s*)(cn=*%s*)(displayName=*%s*)(mail=*%s*)))",
			escaped, escaped, escaped, escaped, escaped)
	}
	base := s.cfg.UserSearchBase
	if base == "" {
		base = s.cfg.BaseDN
	}
	req := ldap.NewSearchRequest(base, ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, limit, 15, false, filter,
		[]string{"sAMAccountName", "uid", "cn", "displayName", "mail", "distinguishedName", "userAccountControl", "lockoutTime", "pwdLastSet", "lastLogonTimestamp"}, nil)
	result, err := searchAllowingSizeLimit(conn, req)
	if err != nil {
		return nil, fmt.Errorf("search directory users: %w", err)
	}
	users := make([]User, 0, len(result.Entries))
	for _, entry := range result.Entries {
		id := entry.GetAttributeValue("sAMAccountName")
		if id == "" {
			id = entry.GetAttributeValue("uid")
		}
		display := entry.GetAttributeValue("displayName")
		if display == "" {
			display = entry.GetAttributeValue("cn")
		}
		accountControl, _ := strconv.ParseInt(entry.GetAttributeValue("userAccountControl"), 10, 64)
		lockout, _ := strconv.ParseInt(entry.GetAttributeValue("lockoutTime"), 10, 64)
		enabled := true
		if entry.GetAttributeValue("userAccountControl") != "" {
			enabled = accountControl&2 == 0
		}
		users = append(users, User{
			ID: id, DisplayName: display, Email: entry.GetAttributeValue("mail"),
			DistinguishedName: entry.DN, OrganizationalUnit: organizationalUnit(entry.DN), Enabled: enabled,
			Locked: lockout > 0, PasswordExpired: entry.GetAttributeValue("pwdLastSet") == "0",
			LastLogon: adTimestamp(entry.GetAttributeValue("lastLogonTimestamp")),
		})
	}
	return users, nil
}

func searchAllowingSizeLimit(conn *ldap.Conn, req *ldap.SearchRequest) (*ldap.SearchResult, error) {
	result, err := conn.Search(req)
	if err == nil {
		return result, nil
	}
	// Servers often return partial results with Size Limit Exceeded; keep those rows.
	if ldap.IsErrorWithCode(err, ldap.LDAPResultSizeLimitExceeded) && result != nil && len(result.Entries) > 0 {
		return result, nil
	}
	return nil, err
}

func (s *LDAPService) ResetPassword(ctx context.Context, userID, password string, forceChange bool) error {
	if !s.cfg.WriteEnabled {
		return fmt.Errorf("directory is configured in read-only mode")
	}
	if strings.EqualFold(s.cfg.Security, "ldap") {
		return fmt.Errorf("password reset requires LDAPS or StartTLS")
	}
	conn, entry, err := s.userEntry(ctx, userID)
	if err != nil {
		return err
	}
	defer conn.Close()
	encoded := utf16.Encode([]rune(`"` + password + `"`))
	buffer := make([]byte, len(encoded)*2)
	for index, value := range encoded {
		binary.LittleEndian.PutUint16(buffer[index*2:], value)
	}
	request := ldap.NewModifyRequest(entry.DN, nil)
	request.Replace("unicodePwd", []string{string(buffer)})
	if forceChange {
		request.Replace("pwdLastSet", []string{"0"})
	}
	if err := conn.Modify(request); err != nil {
		return fmt.Errorf("reset directory password: %w", err)
	}
	return nil
}

func (s *LDAPService) UnlockAccount(ctx context.Context, userID string) error {
	if !s.cfg.WriteEnabled {
		return fmt.Errorf("directory is configured in read-only mode")
	}
	conn, entry, err := s.userEntry(ctx, userID)
	if err != nil {
		return err
	}
	defer conn.Close()
	request := ldap.NewModifyRequest(entry.DN, nil)
	request.Replace("lockoutTime", []string{"0"})
	if err := conn.Modify(request); err != nil {
		return fmt.Errorf("unlock directory account: %w", err)
	}
	return nil
}

func (s *LDAPService) userEntry(ctx context.Context, userID string) (*ldap.Conn, *ldap.Entry, error) {
	conn, err := s.connect(ctx)
	if err != nil {
		return nil, nil, err
	}
	base := s.cfg.UserSearchBase
	if base == "" {
		base = s.cfg.BaseDN
	}
	escaped := ldap.EscapeFilter(userID)
	filter := fmt.Sprintf("(&(|(&(objectCategory=person)(objectClass=user))(objectClass=inetOrgPerson))(|(sAMAccountName=%s)(uid=%s)))", escaped, escaped)
	req := ldap.NewSearchRequest(base, ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 2, 10, false, filter, []string{"distinguishedName"}, nil)
	result, err := conn.Search(req)
	if err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("find directory user: %w", err)
	}
	if len(result.Entries) != 1 {
		conn.Close()
		return nil, nil, fmt.Errorf("directory user was not found")
	}
	return conn, result.Entries[0], nil
}

func organizationalUnit(dn string) string {
	for _, part := range strings.Split(dn, ",") {
		if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(part)), "OU=") {
			return strings.TrimSpace(part[3:])
		}
	}
	return ""
}

func adTimestamp(raw string) string {
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return ""
	}
	const windowsToUnix = 116444736000000000
	return time.Unix((value-windowsToUnix)/10000000, 0).UTC().Format(time.RFC3339)
}
