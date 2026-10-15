package api

import (
	"net/http"

	"directory-management/internal/configstore"
	"github.com/go-chi/chi/v5"
)

type Capability struct {
	ID          string   `json:"id"`
	Group       string   `json:"group"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Status      string   `json:"status"`
	Features    []string `json:"features"`
}

type CapabilitiesHandler struct {
	store configstore.Store
}

func NewCapabilitiesHandler(store configstore.Store) *CapabilitiesHandler {
	return &CapabilitiesHandler{store: store}
}

func (h *CapabilitiesHandler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Get("/", h.catalog)
	return r
}

func (h *CapabilitiesHandler) catalog(w http.ResponseWriter, r *http.Request) {
	capabilities := make([]Capability, len(capabilityCatalog))
	copy(capabilities, capabilityCatalog)
	definitions := configurationDefinitions()
	for i := range capabilities {
		item := &capabilities[i]
		if item.ID == "directory-connection" {
			continue
		}
		if _, ok := definitions[item.ID]; !ok {
			continue
		}
		item.Status = "available"
		if h.store != nil {
			if _, err := h.store.Get(r.Context(), "config:"+item.ID); err == nil {
				item.Status = "configured"
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"source":       "product-audit",
		"version":      "2026.10",
		"capabilities": capabilities,
	})
}

var capabilityCatalog = []Capability{
	{ID: "policy-configuration", Group: "Self-Service", Name: "Policy Configuration", Description: "Target users and groups with reset, unlock, change-password, and profile-update permissions.", Status: "available", Features: []string{"OU and group targeting", "Permission assignment", "Priority ordering", "Durable MySQL storage"}},
	{ID: "multi-factor-authentication", Group: "Self-Service", Name: "Multi-factor Authentication", Description: "Configure authenticator setup, enrollment, and advanced MFA behavior for each self-service policy.", Status: "available", Features: []string{"Authenticator enablement", "Enrollment policy settings", "Portal and endpoint MFA toggles", "Durable admin configuration"}},
	{ID: "password-expiration", Group: "Self-Service", Name: "Password Expiration Notification", Description: "Schedule localized expiry reminders and delivery preferences.", Status: "available", Features: []string{"Scheduler settings", "Channel and escalation preferences", "Message content placeholders", "Durable admin configuration"}},
	{ID: "password-policy", Group: "Self-Service", Name: "Password Policy Enforcer", Description: "Apply additional complexity, length, pattern, history, and breached-password controls.", Status: "available", Features: []string{"Complexity rules", "Pattern restrictions", "Breached-password checks", "User-facing guidance toggles"}},
	{ID: "applications", Group: "Self-Service", Name: "Password Sync / Single Sign-On", Description: "Configure enterprise application authentication defaults and certificate monitoring.", Status: "available", Features: []string{"SAML and OIDC toggles", "Assignment defaults", "Certificate warning settings", "Durable admin configuration"}},
	{ID: "conditional-access", Group: "Self-Service", Name: "Conditional Access", Description: "Configure identity, network, device, application, and risk conditions before access.", Status: "available", Features: []string{"Rule precedence", "Network and geo conditions", "Simulation mode", "Durable admin configuration"}},
	{ID: "directory-self-service", Group: "Self-Service", Name: "Directory Self Service", Description: "Control profile updates, employee search, organization charts, and mail-group subscriptions.", Status: "available", Features: []string{"Self-update attributes", "Employee search privacy", "Organization chart toggle", "Mail-group request settings"}},
	{ID: "quick-enrollment", Group: "Administrative Tools", Name: "Quick Enrollment", Description: "Configure enrollment notifications, import modes, and targeting for eligible users.", Status: "available", Features: []string{"Notify and force modes", "CSV import preview settings", "Target OU and group scope", "Durable admin configuration"}},
	{ID: "endpoint-agents", Group: "Administrative Tools", Name: "Endpoint Login Agents", Description: "Configure Windows, macOS, and Linux login agent settings and cached credentials.", Status: "available", Features: []string{"Platform enablement", "Cached credentials over VPN", "Heartbeat interval", "Deployment guidance URL"}},
	{ID: "mobile-deployment", Group: "Administrative Tools", Name: "Mobile App Deployment", Description: "Configure mobile enrollment targeting, schedules, and push provider settings.", Status: "available", Features: []string{"OU and group targeting", "Notification schedules", "APNs and FCM settings", "Durable admin configuration"}},
	{ID: "approval-workflows", Group: "Administrative Tools", Name: "Approval Workflows", Description: "Require staged approval for recovery, profile updates, and group membership changes.", Status: "available", Features: []string{"Action selection", "Approver routing", "Escalation and SLA", "Decision history retention"}},
	{ID: "technicians", Group: "Administrative Tools", Name: "Technicians", Description: "Delegate constrained administrative access by role, domain, and organizational unit.", Status: "available", Features: []string{"Built-in roles", "OU and domain scope", "Privileged MFA requirement", "Session timeout"}},
	{ID: "security-hardening", Group: "Security Centre", Name: "Security Hardening", Description: "Configure product, transport, directory, mail, backup, and session hardening controls.", Status: "available", Features: []string{"HTTPS and LDAPS toggles", "Backup and export protection", "Audit retention", "Remediation tracking toggle"}},
	{ID: "password-strengtheners", Group: "Security Centre", Name: "Password Strengtheners", Description: "Configure password strength levels and next-logon password change support.", Status: "available", Features: []string{"Strength scoring", "History and similarity checks", "Next-logon change", "Policy assignment toggle"}},
	{ID: "qa-strengtheners", Group: "Security Centre", Name: "Security Q&A Strengtheners", Description: "Harden answer quality, presentation, and verification behavior.", Status: "available", Features: []string{"Duplicate-answer prevention", "Blocked words", "Random subsets", "Case sensitivity"}},
	{ID: "anti-hacking", Group: "Security Centre", Name: "Anti-Hacking System", Description: "Protect recovery surfaces from automation, enumeration, and inactive-account abuse.", Status: "available", Features: []string{"Durable settings storage", "Attempt limits", "Session timeouts", "Security alert toggles"}},
	{ID: "login-settings", Group: "Administration", Name: "Login Settings", Description: "Control directory selection, administrator exposure, CAPTCHA, and account-discovery defenses.", Status: "available", Features: []string{"Web and mobile directory list", "Durable settings storage", "User disclaimer", "Accessible CAPTCHA"}},
	{ID: "personalization", Group: "Administration", Name: "Personalization", Description: "Manage portal branding, locale, welcome names, date formats, and timezone.", Status: "available", Features: []string{"Portal branding", "Language", "Date and time formats", "Session expiry"}},
	{ID: "resilience", Group: "Administration", Name: "Failover and Reverse Proxy", Description: "Configure load balancing, failover, secure gateway, and reverse-proxy access.", Status: "available", Features: []string{"Primary and secondary URLs", "Public access URL", "Trusted proxies", "Health check interval"}},
	{ID: "mail-sms", Group: "Administration", Name: "Mail / SMS Settings", Description: "Configure secure delivery providers, sender identity, and diagnostics retention.", Status: "available", Features: []string{"SMTP or API settings", "TLS and OAuth toggles", "SMS adapter selection", "Durable admin configuration"}},
	{ID: "schedulers", Group: "Administration", Name: "Schedulers and Backups", Description: "Configure synchronizers, report jobs, notifications, and encrypted backups.", Status: "available", Features: []string{"Job enablement", "Manual run permission", "Backup retention", "Integrity verification"}},
	{ID: "integrations", Group: "Administration", Name: "Integrations", Description: "Configure ITSM, SIEM, breached-password, and identity administration adapters.", Status: "available", Features: []string{"Adapter enablement", "Health checks", "Retry policy", "Durable admin configuration"}},
	{ID: "privacy-licensing", Group: "Administration", Name: "Security, Privacy, and Licensing", Description: "Control data protection, usage consent, entitlements, and restricted users.", Status: "available", Features: []string{"Data retention", "Protected exports", "License seat reporting", "Inactive-user restriction"}},
	{ID: "directory-connection", Group: "Active Directory", Name: "Directory Connection", Description: "Bind to Active Directory using config.yaml and discover OUs, groups, and users.", Status: "available", Features: []string{"LDAP/LDAPS/StartTLS", "Connection test", "OU discovery", "Group and user search"}},
}
