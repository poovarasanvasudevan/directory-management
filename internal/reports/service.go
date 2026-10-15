package reports

import (
	"context"
	"fmt"
	"strings"
)

type Definition struct {
	ID          string `json:"id"`
	Category    string `json:"category"`
	Name        string `json:"name"`
	Description string `json:"description"`
}
type Result struct {
	Report  Definition `json:"report"`
	Source  string     `json:"source"`
	Columns []string   `json:"columns"`
	Rows    [][]string `json:"rows"`
}
type Service interface {
	Catalog(context.Context) ([]Definition, error)
	Run(context.Context, string, string) (Result, error)
}

type MockService struct{}

var definitions = []Definition{
	{ID: "locked-users", Category: "User Reports", Name: "Locked out users", Description: "Accounts currently marked as locked."},
	{ID: "expiring-passwords", Category: "User Reports", Name: "Soon-to-expire passwords", Description: "Passwords approaching their configured expiry."},
	{ID: "expired-passwords", Category: "User Reports", Name: "Expired passwords", Description: "Accounts with an expired password."},
	{ID: "password-resets", Category: "Password Self-Service Reports", Name: "Password reset activity", Description: "Completed and failed self-service resets."},
	{ID: "account-unlocks", Category: "Password Self-Service Reports", Name: "Account unlock activity", Description: "Self-service account unlock outcomes."},
	{ID: "agent-password-resets", Category: "Password Self-Service Reports", Name: "Endpoint-agent reset password audit", Description: "Password resets initiated from endpoint login agents."},
	{ID: "agent-account-unlocks", Category: "Password Self-Service Reports", Name: "Endpoint-agent unlock account audit", Description: "Account unlocks initiated from endpoint login agents."},
	{ID: "profile-updates", Category: "Password Self-Service Reports", Name: "Self-update audit", Description: "Directory profile changes requested through self-service."},
	{ID: "password-changes", Category: "Password Self-Service Reports", Name: "Change password audit", Description: "Authenticated password change outcomes."},
	{ID: "expiry-delivery", Category: "Password Self-Service Reports", Name: "Expiry notification delivery", Description: "Password and account expiry notification outcomes."},
	{ID: "mfa-enrollment", Category: "MFA Reports", Name: "MFA enrolled users", Description: "Enrolled and partially enrolled users by verification method."},
	{ID: "mfa-non-enrollment", Category: "MFA Reports", Name: "MFA non-enrolled users", Description: "Eligible users without complete MFA enrollment."},
	{ID: "mfa-enrollment-audit", Category: "MFA Reports", Name: "MFA enrollment audit", Description: "Authenticator enrollment and removal events."},
	{ID: "mfa-usage", Category: "MFA Reports", Name: "MFA usage audit", Description: "Verification activity across self-service actions."},
	{ID: "mfa-endpoints", Category: "MFA Reports", Name: "MFA usage for machines, VPN, and OWA", Description: "Endpoint and remote access verification outcomes."},
	{ID: "mfa-mobile", Category: "MFA Reports", Name: "MFA usage from mobile devices", Description: "Verification activity from mobile clients."},
	{ID: "mfa-failures", Category: "MFA Reports", Name: "MFA failures", Description: "Recent unsuccessful verification attempts."},
	{ID: "trusted-browsers", Category: "MFA Reports", Name: "MFA trusted browsers", Description: "Browsers currently trusted for MFA bypass policy."},
	{ID: "trusted-machines", Category: "MFA Reports", Name: "MFA trusted machines", Description: "Machines currently trusted by endpoint MFA."},
	{ID: "security-questions", Category: "MFA Reports", Name: "Security questions", Description: "Security-question enrollment completeness."},
	{ID: "backup-code-usage", Category: "MFA Reports", Name: "Backup code usage", Description: "One-time backup code consumption events."},
	{ID: "offline-mfa-machines", Category: "MFA Reports", Name: "Offline MFA enrolled machines", Description: "Machines provisioned for offline verification."},
	{ID: "passkeys", Category: "MFA Reports", Name: "FIDO2 passkeys", Description: "Registered passkeys and their last use."},
	{ID: "blocked-users", Category: "MFA Reports", Name: "Blocked users", Description: "Users temporarily blocked by anti-hacking controls."},
	{ID: "agent-status", Category: "GINA/Mac/Linux Agent Reports", Name: "Agent installed machines", Description: "Endpoint agent version and check-in state."},
	{ID: "agent-failures", Category: "GINA/Mac/Linux Agent Reports", Name: "Agent installation failures", Description: "Failed endpoint agent deployments and diagnostics."},
	{ID: "user-attempts", Category: "Other Reports", Name: "User attempts audit", Description: "Successful and failed portal activity."},
	{ID: "licensed-users", Category: "Other Reports", Name: "Licensed users", Description: "Users consuming product and Endpoint MFA entitlements."},
	{ID: "notification-delivery", Category: "Other Reports", Name: "Notification delivery", Description: "Email, SMS, and push delivery outcomes."},
	{ID: "push-devices", Category: "Other Reports", Name: "Push registered devices", Description: "Devices registered for push verification."},
	{ID: "application-access", Category: "Other Reports", Name: "Application access audit", Description: "Federated application access decisions."},
	{ID: "jit-provisioning", Category: "Other Reports", Name: "JIT provisioning audit", Description: "Just-in-time account provisioning outcomes."},
}

func (MockService) Catalog(context.Context) ([]Definition, error) {
	return append([]Definition(nil), definitions...), nil
}
func (MockService) Run(_ context.Context, id, query string) (Result, error) {
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
	columns := []string{"User", "Directory", "Status", "Observed"}
	rows := [][]string{{"Avery Stone", "Northstar", "Locked", "2026-10-14 08:42 UTC"}, {"Mina Patel", "Northstar", "Review", "2026-10-14 07:18 UTC"}, {"Jon Bell", "Northstar", "Resolved", "2026-10-13 16:03 UTC"}}
	if strings.Contains(id, "mfa") {
		columns = []string{"User", "Method", "Status", "Observed"}
		rows = [][]string{{"Avery Stone", "Authenticator", "Enrolled", "2026-10-14 08:42 UTC"}, {"Mina Patel", "Passkey", "Pending", "2026-10-14 07:18 UTC"}, {"Jon Bell", "Email", "Enrolled", "2026-10-13 16:03 UTC"}}
	}
	if id == "agent-status" {
		columns = []string{"Device", "Platform", "Version", "Last check-in"}
		rows = [][]string{{"HR-LT-204", "Windows", "2.4.1", "2026-10-14 08:31 UTC"}, {"DESIGN-MAC-18", "macOS", "2.4.0", "2026-10-14 08:27 UTC"}, {"OPS-LNX-07", "Linux", "2.3.8", "2026-10-13 23:44 UTC"}}
	}
	if query != "" {
		filtered := rows[:0]
		q := strings.ToLower(query)
		for _, row := range rows {
			if strings.Contains(strings.ToLower(strings.Join(row, " ")), q) {
				filtered = append(filtered, row)
			}
		}
		rows = filtered
	}
	return Result{Report: *definition, Source: "mock", Columns: columns, Rows: rows}, nil
}
