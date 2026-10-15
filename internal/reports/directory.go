package reports

import (
	"context"
	"fmt"
	"strings"
	"time"

	"directory-management/internal/audit"
	"directory-management/internal/directory"
)

type DirectoryService struct {
	directory directory.Service
	audit     audit.Store
	domain    string
}

func NewDirectoryService(dir directory.Service, auditStore audit.Store, domain string) *DirectoryService {
	if domain == "" {
		domain = "directory"
	}
	return &DirectoryService{directory: dir, audit: auditStore, domain: domain}
}

func (s *DirectoryService) Catalog(context.Context) ([]Definition, error) {
	return append([]Definition(nil), definitions...), nil
}

func (s *DirectoryService) Run(ctx context.Context, id, query string) (Result, error) {
	var definition *Definition
	for i := range definitions {
		if definitions[i].ID == id {
			definition = &definitions[i]
			break
		}
	}
	if definition == nil {
		return Result{}, fmt.Errorf("unknown report %q", id)
	}

	switch id {
	case "locked-users":
		return s.userReport(ctx, *definition, query, func(user directory.User) bool { return user.Locked })
	case "expired-passwords":
		return s.userReport(ctx, *definition, query, func(user directory.User) bool { return user.PasswordExpired })
	case "expiring-passwords":
		// AD does not expose exact expiry without domain policy; surface password-expired and note limited data.
		return s.userReport(ctx, *definition, query, func(user directory.User) bool { return user.PasswordExpired })
	case "password-resets":
		return s.auditReport(*definition, query, "directory.password-reset")
	case "account-unlocks":
		return s.auditReport(*definition, query, "directory.account-unlock")
	case "mfa-enrollment", "mfa-non-enrollment", "mfa-enrollment-audit", "mfa-usage", "mfa-endpoints", "mfa-mobile", "mfa-failures", "trusted-browsers", "trusted-machines", "security-questions", "backup-code-usage", "offline-mfa-machines", "passkeys":
		return Result{
			Report:  *definition,
			Source:  "unavailable",
			Columns: []string{"Status", "Detail"},
			Rows:    [][]string{{"Unavailable", "MFA reports require the MFA enrollment runtime (later wave)."}},
		}, nil
	default:
		return Result{
			Report:  *definition,
			Source:  "catalog",
			Columns: []string{"Status", "Detail"},
			Rows:    [][]string{{"Planned", "This report catalogue entry is reserved; Wave 1 delivers locked/expired users and audit-backed reset/unlock activity."}},
		}, nil
	}
}

func (s *DirectoryService) userReport(ctx context.Context, definition Definition, query string, match func(directory.User) bool) (Result, error) {
	users, err := s.directory.SearchUsers(ctx, "", 200)
	if err != nil {
		return Result{}, err
	}
	columns := []string{"User", "Account", "Directory", "Status", "Observed"}
	rows := make([][]string, 0)
	now := time.Now().UTC().Format("2006-01-02 15:04 UTC")
	q := strings.ToLower(strings.TrimSpace(query))
	for _, user := range users {
		if !match(user) {
			continue
		}
		status := "Active"
		if user.Locked {
			status = "Locked"
		} else if user.PasswordExpired {
			status = "Password expired"
		} else if !user.Enabled {
			status = "Disabled"
		}
		row := []string{user.DisplayName, user.ID, s.domain, status, now}
		if q == "" || strings.Contains(strings.ToLower(strings.Join(row, " ")), q) {
			rows = append(rows, row)
		}
	}
	return Result{Report: definition, Source: "active-directory", Columns: columns, Rows: rows}, nil
}

func (s *DirectoryService) auditReport(definition Definition, query, action string) (Result, error) {
	events := s.audit.List(audit.Query{Action: action, Limit: 200})
	columns := []string{"Actor", "Target", "Outcome", "Occurred"}
	rows := make([][]string, 0, len(events))
	q := strings.ToLower(strings.TrimSpace(query))
	for _, event := range events {
		row := []string{event.Actor, event.TargetID, event.Outcome, event.OccurredAt}
		if q == "" || strings.Contains(strings.ToLower(strings.Join(row, " ")), q) {
			rows = append(rows, row)
		}
	}
	return Result{Report: definition, Source: "audit", Columns: columns, Rows: rows}, nil
}
