package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"directory-management/internal/audit"
	"directory-management/internal/configstore"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

var configurationIDPattern = regexp.MustCompile(`^[a-z0-9-]{1,64}$`)

type ConfigurationOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type ConfigurationField struct {
	Key         string                `json:"key"`
	Label       string                `json:"label"`
	Description string                `json:"description,omitempty"`
	Type        string                `json:"type"`
	Required    bool                  `json:"required,omitempty"`
	Min         *float64              `json:"min,omitempty"`
	Max         *float64              `json:"max,omitempty"`
	Options     []ConfigurationOption `json:"options,omitempty"`
	Default     any                   `json:"default"`
}

type ConfigurationSection struct {
	ID          string               `json:"id"`
	Title       string               `json:"title"`
	Description string               `json:"description,omitempty"`
	Fields      []ConfigurationField `json:"fields"`
}

type ConfigurationDefinition struct {
	ID          string                 `json:"id"`
	Group       string                 `json:"group"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Sections    []ConfigurationSection `json:"sections"`
}

type ConfigurationHandler struct {
	definitions map[string]ConfigurationDefinition
	defaults    map[string]map[string]any
	store       configstore.Store
	audit       audit.Store
}

func NewConfigurationHandler(store configstore.Store, auditStores ...audit.Store) *ConfigurationHandler {
	definitions := configurationDefinitions()
	defaults := make(map[string]map[string]any, len(definitions))
	for id, definition := range definitions {
		values := make(map[string]any)
		for _, section := range definition.Sections {
			for _, field := range section.Fields {
				values[field.Key] = field.Default
			}
		}
		defaults[id] = values
	}
	var auditStore audit.Store
	if len(auditStores) > 0 {
		auditStore = auditStores[0]
	}
	return &ConfigurationHandler{definitions: definitions, defaults: defaults, store: store, audit: auditStore}
}

func (h *ConfigurationHandler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Get("/", h.list)
	r.Get("/{moduleID}/history", h.history)
	r.Post("/{moduleID}/restore", h.restore)
	r.Get("/{moduleID}", h.get)
	r.Put("/{moduleID}", h.put)
	return r
}

func (h *ConfigurationHandler) list(w http.ResponseWriter, _ *http.Request) {
	items := make([]ConfigurationDefinition, 0, len(h.definitions))
	for _, definition := range h.definitions {
		items = append(items, definition)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Group == items[j].Group {
			return items[i].Title < items[j].Title
		}
		return items[i].Group < items[j].Group
	})
	writeJSON(w, http.StatusOK, map[string]any{"source": "authenticated-demo-audit", "modules": items})
}

func (h *ConfigurationHandler) moduleKey(id string) string { return "config:" + id }

func (h *ConfigurationHandler) get(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "moduleID"))
	definition, ok := h.definitions[id]
	if !ok || !configurationIDPattern.MatchString(id) {
		writeJSON(w, http.StatusNotFound, map[string]string{"code": "configuration_not_found", "message": "configuration module was not found"})
		return
	}
	values := cloneSettings(h.defaults[id])
	updatedAt := ""
	if profile, err := h.store.Get(r.Context(), h.moduleKey(id)); err == nil {
		for key, value := range profile.Values {
			values[key] = value
		}
		updatedAt = profile.UpdatedAt
	}
	version := int64(0)
	if profile, err := h.store.Get(r.Context(), h.moduleKey(id)); err == nil {
		version = profile.Version
	}
	writeJSON(w, http.StatusOK, map[string]any{"definition": definition, "values": values, "version": version, "updatedAt": updatedAt})
}

func (h *ConfigurationHandler) history(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "moduleID"))
	if _, ok := h.definitions[id]; !ok || !configurationIDPattern.MatchString(id) {
		writeJSON(w, http.StatusNotFound, map[string]string{"code": "configuration_not_found", "message": "configuration module was not found"})
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	revisions, err := h.store.History(r.Context(), h.moduleKey(id), limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"code": "configuration_history_failed", "message": "configuration history could not be loaded"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"moduleId": id, "revisions": revisions, "count": len(revisions)})
}

func (h *ConfigurationHandler) put(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "moduleID"))
	definition, ok := h.definitions[id]
	if !ok || !configurationIDPattern.MatchString(id) {
		writeJSON(w, http.StatusNotFound, map[string]string{"code": "configuration_not_found", "message": "configuration module was not found"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 128<<10)
	var request struct {
		Values  map[string]any `json:"values"`
		Version int64          `json:"version"`
	}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&request); err != nil || request.Values == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_json", "message": "values object is required"})
		return
	}
	if err := validateConfiguration(definition, request.Values); err != nil {
		h.record(r, id, "failure", map[string]any{"reason": err.Error()})
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"code": "invalid_configuration", "message": err.Error()})
		return
	}
	profile, err := h.store.PutVersion(r.Context(), h.moduleKey(id), cloneSettings(request.Values), request.Version)
	if errors.Is(err, configstore.ErrVersionConflict) {
		h.record(r, id, "failure", map[string]any{"reason": "version conflict", "expectedVersion": request.Version})
		current, _ := h.store.Get(r.Context(), h.moduleKey(id))
		writeJSON(w, http.StatusConflict, map[string]any{"code": "configuration_conflict", "message": "This configuration was changed by another administrator. Reload it before saving.", "currentVersion": current.Version})
		return
	}
	if err != nil {
		h.record(r, id, "failure", map[string]any{"reason": "storage failure"})
		writeJSON(w, http.StatusInternalServerError, map[string]string{"code": "configuration_save_failed", "message": err.Error()})
		return
	}
	h.record(r, id, "success", map[string]any{"version": profile.Version, "fieldCount": len(profile.Values)})
	writeJSON(w, http.StatusOK, map[string]any{"moduleId": id, "values": profile.Values, "version": profile.Version, "updatedAt": profile.UpdatedAt})
}

func (h *ConfigurationHandler) restore(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "moduleID"))
	definition, ok := h.definitions[id]
	if !ok || !configurationIDPattern.MatchString(id) {
		writeJSON(w, http.StatusNotFound, map[string]string{"code": "configuration_not_found", "message": "configuration module was not found"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	var request struct {
		TargetVersion  int64 `json:"targetVersion"`
		CurrentVersion int64 `json:"currentVersion"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request.TargetVersion < 1 || request.CurrentVersion < 1 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_restore", "message": "targetVersion and currentVersion are required"})
		return
	}
	revision, err := h.store.Revision(r.Context(), h.moduleKey(id), request.TargetVersion)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"code": "revision_not_found", "message": "configuration revision was not found"})
		return
	}
	if err := validateConfiguration(definition, revision.Values); err != nil {
		h.record(r, id, "failure", map[string]any{"reason": "stored revision is incompatible", "targetVersion": request.TargetVersion})
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"code": "revision_incompatible", "message": "the stored revision is no longer compatible with this module"})
		return
	}
	profile, err := h.store.PutVersion(r.Context(), h.moduleKey(id), cloneSettings(revision.Values), request.CurrentVersion)
	if errors.Is(err, configstore.ErrVersionConflict) {
		h.record(r, id, "failure", map[string]any{"reason": "version conflict", "targetVersion": request.TargetVersion})
		current, _ := h.store.Get(r.Context(), h.moduleKey(id))
		writeJSON(w, http.StatusConflict, map[string]any{"code": "configuration_conflict", "message": "This configuration changed before it could be restored. Reload it and try again.", "currentVersion": current.Version})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"code": "configuration_restore_failed", "message": "configuration could not be restored"})
		return
	}
	h.record(r, id, "success", map[string]any{"operation": "restore", "targetVersion": request.TargetVersion, "version": profile.Version})
	writeJSON(w, http.StatusOK, map[string]any{"moduleId": id, "values": profile.Values, "version": profile.Version, "updatedAt": profile.UpdatedAt, "restoredFrom": request.TargetVersion})
}

func (h *ConfigurationHandler) record(r *http.Request, id, outcome string, metadata map[string]any) {
	if h.audit == nil {
		return
	}
	h.audit.Record(audit.Event{Actor: "product-admin", Action: "configuration.update", TargetType: "configuration", TargetID: id, Outcome: outcome, RequestID: middleware.GetReqID(r.Context()), SourceIP: r.RemoteAddr, Metadata: metadata})
}

func validateConfiguration(definition ConfigurationDefinition, values map[string]any) error {
	fields := make(map[string]ConfigurationField)
	for _, section := range definition.Sections {
		for _, field := range section.Fields {
			fields[field.Key] = field
		}
	}
	for key := range values {
		if _, ok := fields[key]; !ok {
			return fmt.Errorf("unknown field %q", key)
		}
	}
	for key, field := range fields {
		value, exists := values[key]
		if !exists {
			if field.Required {
				return fmt.Errorf("%s is required", field.Label)
			}
			continue
		}
		switch field.Type {
		case "boolean":
			if _, ok := value.(bool); !ok {
				return fmt.Errorf("%s must be true or false", field.Label)
			}
		case "number":
			number, ok := value.(float64)
			if !ok || math.IsNaN(number) || math.IsInf(number, 0) {
				return fmt.Errorf("%s must be a number", field.Label)
			}
			if field.Min != nil && number < *field.Min {
				return fmt.Errorf("%s must be at least %v", field.Label, *field.Min)
			}
			if field.Max != nil && number > *field.Max {
				return fmt.Errorf("%s must be at most %v", field.Label, *field.Max)
			}
		case "text", "textarea", "select":
			text, ok := value.(string)
			if !ok {
				return fmt.Errorf("%s must be text", field.Label)
			}
			if field.Required && strings.TrimSpace(text) == "" {
				return fmt.Errorf("%s is required", field.Label)
			}
			if field.Type == "select" && !containsOption(field.Options, text) {
				return fmt.Errorf("%s has an unsupported option", field.Label)
			}
		default:
			return fmt.Errorf("unsupported field type %q", field.Type)
		}
	}
	return nil
}

func containsOption(options []ConfigurationOption, value string) bool {
	for _, option := range options {
		if option.Value == value {
			return true
		}
	}
	return false
}

func boolField(key, label, description string, value bool) ConfigurationField {
	return ConfigurationField{Key: key, Label: label, Description: description, Type: "boolean", Default: value}
}

func textField(key, label, description, value string, required bool) ConfigurationField {
	return ConfigurationField{Key: key, Label: label, Description: description, Type: "text", Default: value, Required: required}
}

func textareaField(key, label, description, value string) ConfigurationField {
	return ConfigurationField{Key: key, Label: label, Description: description, Type: "textarea", Default: value}
}

func numberField(key, label, description string, value, min, max float64) ConfigurationField {
	return ConfigurationField{Key: key, Label: label, Description: description, Type: "number", Default: value, Min: &min, Max: &max}
}

func selectField(key, label, description, value string, options ...string) ConfigurationField {
	items := make([]ConfigurationOption, 0, len(options))
	for _, option := range options {
		parts := strings.SplitN(option, ":", 2)
		item := ConfigurationOption{Value: parts[0], Label: parts[0]}
		if len(parts) == 2 {
			item.Label = parts[1]
		}
		items = append(items, item)
	}
	return ConfigurationField{Key: key, Label: label, Description: description, Type: "select", Default: value, Options: items}
}

func section(id, title, description string, fields ...ConfigurationField) ConfigurationSection {
	return ConfigurationSection{ID: id, Title: title, Description: description, Fields: fields}
}

func configurationDefinitions() map[string]ConfigurationDefinition {
	definitions := []ConfigurationDefinition{
		{ID: "policy-configuration", Group: "Self-Service", Title: "Policy Configuration", Description: "Set the default self-service permissions and deterministic policy behavior.", Sections: []ConfigurationSection{
			section("permissions", "Self-service features", "Applied when a more specific user, group, or OU policy does not override them.",
				boolField("resetPassword", "Reset password", "Allow eligible users to reset an expired or forgotten password.", true),
				boolField("unlockAccount", "Unlock account", "Allow eligible users to unlock a directory account.", true),
				boolField("changePassword", "Change password", "Allow authenticated password changes.", true),
				boolField("selfUpdate", "Self update", "Allow approved directory attribute updates.", true)),
			section("evaluation", "Policy evaluation", "Control automatic actions and precedence.",
				boolField("automaticReset", "Automatic reset", "Complete approved reset workflows without technician intervention.", false),
				boolField("automaticUnlock", "Automatic unlock", "Complete approved unlock workflows automatically.", false),
				selectField("conflictStrategy", "Conflict strategy", "How overlapping policy assignments are resolved.", "priority", "priority:Highest priority wins", "restrictive:Most restrictive wins")),
		}},
		{ID: "multi-factor-authentication", Group: "Self-Service", Title: "Multi-factor Authentication", Description: "Configure authenticator setup, enrollment, and advanced MFA behavior for each self-service policy. Settings are stored; verification runtimes are not enabled by this form.", Sections: []ConfigurationSection{
			section("policy", "Policy selection", "MFA settings apply to the selected self-service policy.",
				selectField("policy", "Choose the policy", "Policy receiving this authenticator configuration.", "selfservice", "selfservice:Self-service users", "endpoint:Endpoint MFA users", "remote:Remote users", "local:Local users")),
			section("authenticator-setup", "Authenticator setup", "Enable authenticators available during enrollment and verification (admin configuration only).",
				boolField("securityQuestions", "Security Question & Answer", "Administrator- and user-defined security questions.", true),
				boolField("emailVerification", "Email Verification", "One-time codes to approved email addresses.", true),
				boolField("smsVerification", "SMS Verification", "One-time codes through the configured SMS adapter.", false),
				boolField("googleAuthenticator", "Google Authenticator", "TOTP via Google Authenticator.", true),
				boolField("microsoftAuthenticator", "Microsoft Authenticator", "TOTP via Microsoft Authenticator.", true),
				boolField("zohoOneAuth", "Zoho OneAuth TOTP", "TOTP via Zoho OneAuth.", false),
				boolField("customTotp", "Custom TOTP authenticator", "Hardware or software TOTP tokens.", false),
				boolField("push", "Push Notification Authentication", "Push via the product mobile application.", true),
				boolField("qrCode", "QR Code Based Authentication", "QR challenge via the mobile application.", false),
				boolField("fingerprint", "Fingerprint / biometric authentication", "Biometrics via the mobile application.", false),
				boolField("passkeys", "FIDO2 Passkeys", "Phishing-resistant WebAuthn credentials.", true),
				boolField("duo", "Duo Security", "Configured Duo adapter.", false),
				boolField("radius", "RADIUS Authentication", "Configured RADIUS adapter.", false),
				boolField("rsaSecurID", "RSA SecurID", "Configured RSA SecurID adapter.", false),
				boolField("entraMFA", "Microsoft Entra ID MFA", "Configured Entra ID MFA adapter.", false),
				boolField("yubiKey", "YubiKey Authenticator", "Yubico OTP / hardware key authentication.", false),
				boolField("samlAuth", "SAML Authentication", "External SAML identity provider as a factor.", false),
				boolField("allowRecoveryCodes", "Backup recovery codes", "Issue one-time recovery codes after enrollment.", true)),
			section("mfa-enrollment", "MFA enrollment", "Control how users enroll for enabled authenticators.",
				selectField("enrollmentMode", "Enrollment mode", "How users are prompted to enroll.", "notify", "notify:Notify users", "force:Force enrollment when they log in", "admin:Administrator enrollment"),
				boolField("enforceSelectedAuthenticators", "Enforce selected authenticators during enrollment", "Require the authenticators marked mandatory below.", true),
				numberField("minimumFactors", "Required factors", "Minimum configured authenticators per user.", 2, 1, 5),
				boolField("hideEnrollmentWhenComplete", "Hide Enrollment tab for enrolled users", "Remove enrollment UI after requirements are met.", true),
				boolField("forceEnrollmentAfterPassword", "Force enrollment after password verification", "Prompt enrollment after successful password check.", false)),
			section("mfa-advanced", "Advanced", "Portal, endpoint, and trust behavior for MFA challenges.",
				boolField("mfaSelfService", "MFA for self-service actions", "Require MFA for reset, unlock, and related portal actions.", true),
				boolField("mfaPortalLogin", "MFA for product portal logins", "Require MFA when signing in to the end-user portal.", true),
				boolField("mfaEndpointLogins", "MFA for machine / endpoint logins", "Apply MFA to Windows, macOS, and Linux login agents.", false),
				boolField("mfaOWA", "MFA for Outlook on the web / EAC", "Protect OWA and Exchange admin center access.", false),
				boolField("mfaVPN", "MFA for RADIUS / VPN endpoints", "Protect VPN and RADIUS-integrated endpoints.", false),
				boolField("passwordlessPortal", "Enable passwordless portal logins", "Allow passwordless access where authenticators support it.", false),
				numberField("idleMinutes", "MFA idle time limit (minutes)", "Minutes allowed to complete an MFA challenge.", 5, 1, 60),
				boolField("trustBrowserDefault", "Keep Trust this browser selected by default", "Pre-select browser trust on MFA screens.", false),
				numberField("trustBrowserDays", "Expire browser trust after (days)", "Days before a trusted browser must re-verify.", 30, 1, 365),
				selectField("unenrolledAction", "Unenrolled user action", "Behavior when a user has not completed enrollment.", "deny", "deny:Deny access", "allow:Allow with warning", "force:Force enrollment")),
		}},
		{ID: "password-expiration", Group: "Self-Service", Title: "Password Expiration Notification", Description: "Schedule password and account-expiration notifications. Delivery providers are configured separately; this module stores notification policy only.", Sections: []ConfigurationSection{
			section("scheduler", "Notification scheduler", "Controls the recurring expiration scan.",
				textField("schedulerName", "Scheduler name", "A recognizable name for operations and reports.", "Daily password expiry reminder", true),
				boolField("enabled", "Enable scheduler", "Run this notification schedule.", true),
				selectField("frequency", "Frequency", "How often eligible directory accounts are evaluated.", "daily", "daily:Daily", "weekdays:Weekdays", "weekly:Weekly"),
				textField("runAt", "Preferred run time", "Local time of day for the daily evaluation (HH:MM).", "09:00", false),
				numberField("notifyBeforeDays", "Notify before expiry", "Days before password expiration to begin notifications.", 14, 1, 90),
				numberField("repeatEveryDays", "Repeat interval", "Days between repeat notifications.", 3, 1, 30),
				boolField("accountExpiry", "Also notify account expiration", "Include account-expiration dates in the same schedule.", true)),
			section("delivery", "Delivery and escalation", "Choose recipients and escalation behavior.",
				selectField("channel", "Primary channel", "Default notification delivery channel.", "email", "email:Email", "sms:SMS", "both:Email and SMS", "push:Push"),
				textField("adminAddress", "Administrator address", "Receives delivery and scheduler failures.", "identity-ops@example.com", true),
				boolField("notifyUser", "Notify the user", "Send reminders directly to the account holder.", true),
				boolField("managerEscalation", "Manager escalation", "Notify the manager when expiration becomes urgent.", true),
				numberField("escalateBeforeDays", "Escalate before expiry", "Days before expiry to escalate to managers.", 3, 1, 30),
				boolField("includeLockedAccounts", "Include locked accounts", "Notify users whose passwords expire while locked.", false),
				boolField("includeDisabledAccounts", "Include disabled accounts", "Evaluate disabled accounts in the schedule.", false),
				boolField("deliveryReport", "Retain delivery report summary", "Keep aggregate delivery outcomes for administrators.", true)),
			section("content", "Message content", "Template and localization defaults (content only; no outbound send from this form).",
				textField("emailSubject", "Email subject", "Subject line for expiry reminder messages.", "Your password expires soon", false),
				textareaField("emailBody", "Email body", "Plain-text reminder body. Tokens such as {user} and {days} are placeholders.", "Hello {user},\nYour password expires in {days} day(s)."),
				selectField("locale", "Default locale", "Locale used when the user has no preferred language.", "en-US", "en-US:English (US)", "en-GB:English (UK)", "de-DE:German", "fr-FR:French", "ja-JP:Japanese")),
		}},
		{ID: "password-policy", Group: "Self-Service", Title: "Password Policy Enforcer", Description: "Apply additional length, character, pattern, history, and breach controls before directory submission.", Sections: []ConfigurationSection{
			section("complexity", "Custom password policy", "Enforced before a password is submitted to the directory.",
				boolField("enabled", "Enforce custom password policy", "Apply these controls to reset and change-password workflows.", true),
				numberField("minimumLength", "Minimum length", "Minimum accepted password length.", 14, 8, 128),
				numberField("maximumLength", "Maximum length", "Maximum accepted password length.", 64, 8, 256),
				numberField("uppercaseCount", "Uppercase characters", "Minimum uppercase characters.", 1, 0, 16),
				numberField("lowercaseCount", "Lowercase characters", "Minimum lowercase characters.", 1, 0, 16),
				numberField("numberCount", "Numeric characters", "Minimum numeric characters.", 1, 0, 16),
				numberField("specialCount", "Special characters", "Minimum special characters.", 1, 0, 16),
				numberField("historyCount", "Password history", "Previous passwords that cannot be reused.", 12, 0, 64)),
			section("restrictions", "Pattern restrictions", "Block predictable and compromised passwords.",
				boolField("disallowUsername", "Disallow username", "Reject passwords containing the account name.", true),
				boolField("disallowDisplayName", "Disallow display name parts", "Reject passwords containing display-name tokens.", true),
				boolField("disallowNumericSuffix", "Disallow numeric last character", "Prevent a number-only suffix from satisfying complexity.", true),
				boolField("disallowSequential", "Disallow sequential characters", "Reject obvious sequences such as 1234 or abcd.", true),
				boolField("disallowRepeating", "Disallow repeating characters", "Reject long runs of the same character.", true),
				textareaField("dictionaryWords", "Custom dictionary", "Blocked words, one per line.", "password\nwelcome\ncompany"),
				boolField("breachedPasswordCheck", "Block breached passwords", "Check the configured breached-password provider.", true),
				boolField("showRequirements", "Show requirements to users", "Display the effective policy during reset and change.", true),
				boolField("enforceOnAgents", "Enforce on endpoint agents", "Apply the same policy on supported endpoint logon agents.", true)),
		}},
		{ID: "applications", Group: "Self-Service", Title: "Password Sync / Single Sign-On", Description: "Configure application authentication defaults, assignment, and certificate monitoring. Federation runtimes are not started from this form.", Sections: []ConfigurationSection{
			section("protocols", "Application protocols", "Protocols available for enterprise application integrations.",
				boolField("saml", "SAML 2.0 applications", "Allow signed SAML identity-provider flows.", true),
				boolField("oidc", "OpenID Connect applications", "Allow standards-compliant OIDC clients.", true),
				boolField("passwordSync", "Password synchronization", "Synchronize supported downstream application passwords.", false),
				boolField("jit", "Just-in-time provisioning", "Create mapped application accounts after approved sign-in.", false),
				boolField("scim", "SCIM provisioning hooks", "Expose SCIM mapping settings for supported apps.", false)),
			section("assignment", "Assignment and certificates", "Defaults for new application connections.",
				selectField("assignment", "Default assignment", "Who receives a newly configured application.", "none", "none:No users", "policy:Selected policies", "all:All eligible users"),
				boolField("requireMFA", "Require MFA for application access", "Step up with MFA before application launch.", true),
				numberField("certificateWarningDays", "Certificate warning", "Days before certificate expiration to alert administrators.", 30, 1, 180),
				boolField("requireSignedRequests", "Require signed requests", "Reject unsigned requests where the protocol supports signatures.", true),
				boolField("encryptAssertions", "Encrypt SAML assertions", "Encrypt assertions when the SP supports encryption.", false),
				textField("entityID", "Default IdP entity ID", "Placeholder entity ID used for new SAML apps.", "https://identity.example.com/idp", false)),
		}},
		{ID: "conditional-access", Group: "Self-Service", Title: "Conditional Access", Description: "Evaluate network, device, identity, application, and risk conditions before protected actions.", Sections: []ConfigurationSection{
			section("decision", "Rule configuration", "Default behavior and step-up action for matched rules.",
				boolField("enabled", "Enable conditional access", "Evaluate rules before protected self-service actions.", true),
				selectField("defaultAction", "Default action", "Applied when no explicit rule matches.", "allow", "allow:Allow", "mfa:Require MFA", "deny:Deny"),
				selectField("riskAction", "Elevated-risk action", "Applied when the risk engine reports elevated risk.", "mfa", "mfa:Require MFA", "deny:Deny", "review:Require approval"),
				selectField("precedence", "Rule precedence", "How overlapping rules are resolved.", "first-match", "first-match:First matching rule", "most-restrictive:Most restrictive wins")),
			section("conditions", "Conditions", "Signals available to the rule evaluator.",
				textareaField("trustedNetworks", "Trusted networks", "CIDR ranges, one per line.", "10.0.0.0/8\n192.168.0.0/16"),
				textareaField("blockedNetworks", "Blocked networks", "CIDR ranges denied access, one per line.", ""),
				boolField("deviceTrust", "Evaluate device trust", "Require an enrolled or managed device where configured.", true),
				boolField("geoVelocity", "Evaluate impossible travel", "Detect implausible location changes.", true),
				boolField("blockedCountries", "Apply country restrictions", "Evaluate the tenant blocked-country list.", false),
				textareaField("blockedCountryCodes", "Blocked country codes", "ISO country codes, one per line.", ""),
				boolField("applicationAware", "Evaluate application context", "Apply rules based on the target application.", true),
				boolField("simulationMode", "Simulation mode", "Log rule outcomes without enforcing deny actions.", false)),
		}},
		{ID: "directory-self-service", Group: "Self-Service", Title: "Directory Self Service", Description: "Control profile updates, employee search, organization charts, and mail groups.", Sections: []ConfigurationSection{
			section("profile", "Directory profile / self update", "Attributes users may view or request to update.",
				boolField("profileUpdates", "Allow profile updates", "Expose approved attributes in the employee portal.", true),
				textareaField("editableAttributes", "Editable attributes", "Directory attribute names, one per line.", "telephoneNumber\nmobile\nmanager\ntitle"),
				textareaField("readOnlyAttributes", "Read-only attributes", "Attributes shown but not editable, one per line.", "mail\nsAMAccountName\ndepartment"),
				boolField("approvalRequired", "Require update approval", "Queue writes for technician approval.", true),
				boolField("notifyOnUpdate", "Notify user after update", "Send a confirmation when a profile change completes.", true)),
			section("discovery", "Directory discovery", "Employee and group discovery features.",
				boolField("employeeSearch", "Employee search", "Enable permission-filtered directory search.", true),
				boolField("organizationChart", "Organization chart", "Build reporting relationships from the manager attribute.", true),
				boolField("mailGroups", "Mail-group subscription", "Allow eligible join and leave requests.", true),
				boolField("hideRestrictedUsers", "Hide restricted users from search", "Exclude license-restricted accounts from directory search.", true),
				selectField("privacyMode", "Search privacy", "Amount of profile data returned in search.", "limited", "limited:Limited profile", "standard:Standard profile", "custom:Custom attributes")),
		}},
		{ID: "quick-enrollment", Group: "Administrative Tools", Title: "Quick Enrollment", Description: "Enroll eligible users through controlled bulk and notification workflows. Imports validate and store intent only.", Sections: []ConfigurationSection{
			section("method", "Force or import enrollment", "Select the administrative enrollment workflow.",
				selectField("mode", "Enrollment method", "How eligible users enter the enrollment workflow.", "notify", "notify:Notify users", "force:Force enrollment", "csv:CSV import", "database:In-house database import", "script:Enrollment script"),
				textareaField("targetScope", "Target users, groups, or OUs", "One directory target per line.", "OU=Employees,DC=example,DC=com"),
				boolField("dontInheritChildOUs", "Don't inherit child OUs", "Limit targeting to the selected OUs only.", false),
				boolField("sendNotification", "Send enrollment notification", "Queue notification for each targeted user.", true),
				selectField("notificationChannel", "Notification channel", "Channel used for enrollment notices.", "email", "email:Email", "sms:SMS", "both:Email and SMS"),
				numberField("deadlineDays", "Enrollment deadline", "Days before force-enrollment enforcement.", 7, 1, 90),
				boolField("previewRequired", "Require preview", "Require validation and preview before a bulk import runs.", true),
				boolField("exportFailures", "Allow failure export", "Export failed import rows for remediation.", true)),
		}},
		{ID: "endpoint-agents", Group: "Administrative Tools", Title: "GINA / Mac / Linux", Description: "Manage endpoint logon agent settings, deployment guidance, and cached credentials. Agent inventory is not collected by this form.", Sections: []ConfigurationSection{
			section("platforms", "Endpoint agents", "Platforms eligible for deployment and monitoring.",
				boolField("windows", "Windows Credential Provider / GINA", "Manage supported Windows endpoint agents.", true),
				boolField("macos", "macOS login agent", "Manage supported macOS login agents.", true),
				boolField("linux", "Linux login agent", "Manage supported Linux login agents.", false),
				boolField("offlineMFA", "Offline MFA for Windows and macOS", "Allow configured offline authenticators on agents.", false),
				selectField("updateChannel", "Update channel", "Agent release channel used for managed updates.", "stable", "stable:Stable", "preview:Preview", "manual:Manual"),
				textField("installationPackageURL", "Installation package URL", "Optional internal URL shown in deployment guidance.", "", false)),
			section("cached-credentials", "Password sync / cached credentials", "Controls remote cached-credential updates.",
				boolField("vpnCache", "Update cached credentials over VPN", "Allow approved cached-credential jobs.", true),
				boolField("requireTLS", "Require TLS", "Reject unencrypted agent communication.", true),
				boolField("uacMFA", "Protect UAC prompts with MFA", "Apply MFA settings to Windows UAC elevations.", false),
				boolField("rdpMFA", "Protect RDP logins with MFA", "Apply MFA settings to remote desktop logins.", false),
				numberField("heartbeatMinutes", "Heartbeat interval", "Minutes between expected agent heartbeats.", 15, 5, 1440)),
		}},
		{ID: "mobile-deployment", Group: "Administrative Tools", Title: "Mobile App Deployment", Description: "Schedule and track mobile enrollment delivery settings. Push providers are configured here; messages are not sent from this form.", Sections: []ConfigurationSection{
			section("delivery", "Deployment notification", "Targets and delivery settings for mobile enrollment.",
				selectField("platform", "Platform", "Mobile platforms included in the deployment.", "all", "all:iOS and Android", "ios:iOS", "android:Android"),
				textareaField("targetScope", "Target users, groups, or OUs", "One directory target per line.", "OU=Employees,DC=example,DC=com"),
				boolField("sendNotifications", "Send notification", "Queue installation and enrollment instructions.", true),
				selectField("schedule", "Notification schedule", "When queued notifications may be delivered.", "immediate", "immediate:Immediate", "business-hours:Business hours", "weekly:Weekly digest"),
				numberField("batchSize", "Batch size", "Maximum users processed in one delivery batch.", 50, 1, 1000),
				numberField("retryCount", "Retry attempts", "Retries after a transient delivery failure.", 3, 0, 10)),
			section("push", "Push services", "Provider-specific push configuration state.",
				boolField("apns", "Apple Push Notification service", "Use the configured APNs certificate or token.", false),
				textField("apnsTeamID", "APNs team ID", "Apple Developer team identifier when using token auth.", "", false),
				boolField("fcm", "Firebase Cloud Messaging", "Use the configured Android push project.", false),
				textField("fcmProjectID", "FCM project ID", "Firebase project used for Android push.", "", false),
				boolField("requireManagedDevice", "Require managed device", "Limit deployment to managed devices.", false)),
		}},
		{ID: "approval-workflows", Group: "Administrative Tools", Title: "Approval Workflow", Description: "Require technician approval for selected self-service actions.", Sections: []ConfigurationSection{
			section("actions", "Self-service approval workflow", "Actions placed into an approval queue.",
				boolField("enabled", "Enable approval workflow", "Evaluate approval rules for configured policies.", true),
				boolField("recovery", "Reset password and unlock account", "Approve sensitive recovery actions.", true),
				boolField("changePassword", "Change password", "Approve authenticated password changes when policy requires it.", false),
				boolField("profile", "Directory self-update", "Approve directory attribute writes.", true),
				boolField("groups", "Mail-group subscription", "Approve group membership changes.", false)),
			section("routing", "Approval routing", "Approver source, deadline, and escalation.",
				selectField("approver", "Approver source", "First approver for a new workflow instance.", "manager", "manager:User manager", "technician:Assigned technician", "group:Approver group"),
				textField("approverGroup", "Approver group DN", "Used when approver source is Approver group.", "", false),
				numberField("slaHours", "Decision deadline", "Hours before an approval is overdue.", 24, 1, 720),
				boolField("multiStage", "Multi-stage approval", "Require a second approver after the first decision.", false),
				boolField("escalate", "Escalate overdue requests", "Route overdue requests to the fallback approver.", true),
				textField("fallbackApprover", "Fallback approver", "Technician or group used for escalation.", "security-admins@example.com", false),
				boolField("notifyRequester", "Notify requester", "Send status notifications after each decision.", true),
				boolField("retainHistory", "Retain decision history", "Keep approval decisions for audit reporting.", true)),
		}},
		{ID: "technicians", Group: "Administrative Tools", Title: "Technician", Description: "Delegate administration with least-privilege roles and directory scope.", Sections: []ConfigurationSection{
			section("access", "Delegated administration", "Defaults for technician accounts and custom roles.",
				boolField("requireMFA", "Require technician MFA", "Require MFA for every privileged session.", true),
				selectField("defaultRole", "Default role", "Role assigned to newly delegated technicians.", "readonly", "readonly:Auditor", "helpdesk:Help desk", "directory:Directory administrator", "security:Security administrator", "super:Super administrator"),
				textareaField("allowedOUs", "Default OU scope", "Distinguished names, one per line.", "OU=Employees,DC=example,DC=com"),
				textareaField("allowedDomains", "Allowed domains", "Directory domains technicians may administer, one per line.", "example.com"),
				boolField("customRoles", "Allow custom roles", "Permit administrators to define custom permission sets.", true),
				boolField("auditDelegation", "Audit delegated actions", "Record role, scope, target, and result.", true),
				numberField("sessionMinutes", "Privileged session timeout", "Idle minutes before reauthentication.", 30, 5, 480)),
		}},
		{ID: "security-hardening", Group: "Security Centre", Title: "Security Hardening", Description: "Track and enforce secure product, directory, session, mail, and backup controls contributing to the security score.", Sections: []ConfigurationSection{
			section("transport", "Transport and privileged access", "Controls contributing to the product security score.",
				boolField("https", "Require HTTPS", "Redirect application traffic to HTTPS.", true),
				boolField("hsts", "Enable HSTS", "Send Strict-Transport-Security on HTTPS responses.", true),
				boolField("ldaps", "Require LDAPS", "Validate TLS for directory connections.", true),
				boolField("privilegedMFA", "Require privileged MFA", "Require MFA for administrative roles.", true),
				boolField("secureCookies", "Secure session cookies", "Require Secure, HttpOnly, and SameSite attributes.", true),
				boolField("secureAgentComms", "Require secure agent communication", "Reject unencrypted endpoint-agent channels.", true)),
			section("data", "Data protection", "Retention and protected operational artifacts.",
				boolField("encryptedBackups", "Encrypt backups", "Protect database backup artifacts.", true),
				boolField("protectedExports", "Protect report exports", "Require protected scheduled and manual exports.", true),
				boolField("mailTLS", "Require mail TLS", "Reject insecure notification transport.", true),
				numberField("auditRetentionDays", "Audit retention", "Days of append-only audit history retained.", 365, 30, 3650),
				numberField("sessionIdleMinutes", "Session idle timeout", "Idle minutes before an administrator is signed out.", 20, 5, 240),
				boolField("scoreRemediationTracking", "Track remediation items", "Keep open hardening findings until addressed.", true)),
		}},
		{ID: "password-strengtheners", Group: "Security Centre", Title: "Password Strengtheners", Description: "Raise effective password strength and rotation requirements alongside directory policy.", Sections: []ConfigurationSection{
			section("strength", "Password strength", "Controls evaluated alongside the directory password policy.",
				boolField("enabled", "Enable password strengtheners", "Apply strength controls during reset and change.", true),
				selectField("score", "Minimum strength", "Minimum accepted estimator score.", "strong", "medium:Medium", "strong:Strong", "very-strong:Very strong"),
				numberField("minimumLength", "Minimum length", "Minimum password length.", 14, 8, 128),
				numberField("historyCount", "Password history", "Number of previous passwords that cannot be reused.", 12, 0, 64),
				boolField("breachedCheck", "Breached-password check", "Reject known compromised passwords.", true),
				boolField("similarityCheck", "Block similar passwords", "Reject passwords too similar to the previous password.", true),
				boolField("changeAtNextLogon", "Support change at next logon", "Allow approved accounts to be flagged for change.", true),
				boolField("assignByPolicy", "Assign by self-service policy", "Apply strengtheners per selected policies.", true)),
		}},
		{ID: "qa-strengtheners", Group: "Security Centre", Title: "Security Q&A Strengtheners", Description: "Improve security-answer quality, presentation, and authentication behavior.", Sections: []ConfigurationSection{
			section("answers", "Security answer strengtheners", "Quality controls applied during enrollment.",
				numberField("minimumLength", "Minimum answer length", "Minimum characters in each answer.", 6, 3, 128),
				boolField("uniqueAnswers", "Require unique answers", "Prevent answer reuse across questions.", true),
				boolField("hideAnswers", "Hide answers", "Mask answers during entry and review.", true),
				boolField("blockQuestionWords", "Block question words in answers", "Reject answers that repeat words from the question.", true),
				textareaField("blockedWords", "Blocked words", "Disallowed answers, one per line.", "password\nanswer\ncompany")),
			section("authentication", "Question display and authentication", "Controls used when questions verify a user.",
				numberField("randomSubset", "Questions per challenge", "Random questions selected for each challenge.", 3, 1, 10),
				boolField("randomOrder", "Randomize question order", "Change display order for each challenge.", true),
				boolField("caseSensitive", "Case-sensitive answers", "Require answer case to match enrollment.", false),
				numberField("allowedFailures", "Allowed failures", "Failed challenges before a temporary block.", 5, 1, 20)),
		}},
		{ID: "anti-hacking", Group: "Security Centre", Title: "Anti-Hacking System", Description: "Limit automated attacks, alert users, and close inactive-account loopholes.", Sections: []ConfigurationSection{
			section("prevention", "Brute-force and dictionary attack prevention", "Rate limits for authentication and recovery.",
				numberField("attemptLimit", "Attempt limit", "Failures permitted inside the evaluation window.", 5, 1, 50),
				numberField("windowMinutes", "Evaluation window", "Minutes used to count failed attempts.", 10, 1, 1440),
				numberField("blockMinutes", "Temporary block", "Minutes a blocked subject must wait.", 15, 1, 10080),
				boolField("ipThrottling", "IP throttling", "Apply an additional source-address limit.", true),
				boolField("genericResponses", "Generic recovery responses", "Prevent valid-account discovery.", true)),
			section("alerts", "Self-service action alerts", "Notifications generated after sensitive actions.",
				boolField("notifyUser", "Notify users", "Send an alert after reset, unlock, enrollment, or profile change.", true),
				boolField("notifyAdmin", "Notify administrators of blocks", "Alert security administrators after a temporary block.", true),
				selectField("alertChannel", "Alert channel", "Primary channel for security alerts.", "email", "email:Email", "sms:SMS", "both:Email and SMS")),
			section("inactive", "Inactive-account safeguards", "Prevent recovery from bypassing directory lifecycle controls.",
				boolField("restrictInactiveUsers", "Block inactive users", "Deny self-service for inactive directory accounts.", true),
				numberField("inactiveDays", "Inactive threshold", "Days without activity before safeguards apply.", 90, 1, 3650),
				boolField("requireApproval", "Require approval for exceptions", "Route inactive-account exceptions to a technician.", true)),
		}},
		{ID: "login-settings", Group: "Administration", Title: "Login Settings", Description: "Control directory selection, CAPTCHA, account-discovery defenses, and help visibility on end-user pages.", Sections: []ConfigurationSection{
			section("directory-list", "Directory selection", "How users choose a directory on web and mobile.",
				boolField("showWebDirectory", "Show directory selection on web", "Allow users to choose a configured directory.", true),
				boolField("showMobileDirectory", "Show directory selection on mobile", "Apply the directory list to responsive and mobile clients.", true),
				boolField("rememberLastDirectory", "Remember last directory", "Pre-select the user's previous directory choice.", true)),
			section("defenses", "Access defenses", "CAPTCHA and account-discovery protections.",
				boolField("preventAccountDiscovery", "Prevent account discovery", "Use indistinguishable responses for known and unknown users.", true),
				boolField("requireCaptcha", "Require CAPTCHA after repeated attempts", "Challenge automated recovery attempts.", false),
				boolField("audioCaptcha", "Offer accessible audio CAPTCHA", "Provide an audio alternative.", false),
				boolField("hideHelp", "Hide help on end-user pages", "Remove the help action from public recovery pages.", false),
				textareaField("userDisclaimer", "User disclaimer", "Optional disclaimer shown before self-service actions.", "")),
		}},
		{ID: "personalization", Group: "Administration", Title: "Personalization", Description: "Manage portal branding, locale, welcome names, date formats, and session expiry.", Sections: []ConfigurationSection{
			section("branding", "Portal branding", "Organization identity applied to web and mobile portals.",
				boolField("portalBranding", "Use organization branding", "Apply configured name, mark, and theme.", true),
				boolField("mobileBranding", "Use mobile branding", "Apply approved branding to mobile clients.", true),
				textField("portalName", "Portal display name", "Name shown in the end-user portal header.", "Directory Desk", false),
				textField("welcomeName", "Welcome name format", "How users are greeted after sign-in.", "displayName", false),
				textField("primaryColor", "Primary brand color", "Hex color used for primary actions.", "#0B5FFF", false)),
			section("locale", "Language and formats", "Localization and session presentation.",
				boolField("localizedPortal", "Enable localized portal content", "Select language from the user or browser locale.", true),
				selectField("defaultLanguage", "Default language", "Fallback language when none is preferred.", "en", "en:English", "de:German", "fr:French", "ja:Japanese", "es:Spanish"),
				selectField("dateFormat", "Date format", "Date presentation in the portal.", "ymd", "ymd:YYYY-MM-DD", "mdy:MM/DD/YYYY", "dmy:DD/MM/YYYY"),
				selectField("timeZone", "Default timezone", "Timezone used for schedules and timestamps.", "UTC", "UTC:UTC", "America/New_York:America/New_York", "Europe/London:Europe/London", "Asia/Kolkata:Asia/Kolkata"),
				numberField("sessionExpiryMinutes", "Session expiry", "Minutes before an idle end-user session ends.", 30, 5, 480)),
		}},
		{ID: "resilience", Group: "Administration", Title: "Failover and Reverse Proxy", Description: "Model load balancing, failover, secure gateway, and reverse-proxy access. Health probes store configuration only.", Sections: []ConfigurationSection{
			section("failover", "Failover and secure gateway", "Primary/secondary application node settings.",
				boolField("loadBalancing", "Enable load balancing", "Distribute traffic across healthy nodes.", false),
				boolField("failover", "Enable automatic failover", "Promote the secondary node after health failure.", false),
				boolField("secureGateway", "Enable secure gateway", "Publish recovery services through the gateway.", false),
				textField("primaryURL", "Primary node URL", "Base URL of the primary application node.", "https://identity-primary.example.com", false),
				textField("secondaryURL", "Secondary node URL", "Base URL of the secondary application node.", "https://identity-secondary.example.com", false),
				numberField("healthIntervalSeconds", "Health check interval", "Seconds between configured health probes.", 30, 5, 600)),
			section("reverse-proxy", "Reverse proxy", "Publish the application through a controlled external route.",
				boolField("reverseProxy", "Enable reverse-proxy mode", "Trust configured proxy headers for public access.", false),
				textField("publicAccessURL", "Public access URL", "Externally reachable product URL.", "https://password.example.com", false),
				boolField("https", "Require HTTPS", "Only accept secure proxy traffic.", true),
				boolField("forwardedHeaders", "Trust validated forwarded headers", "Use headers only from configured proxies.", true),
				textareaField("trustedProxies", "Trusted proxy CIDRs", "Proxy source ranges, one per line.", "10.0.0.0/8")),
		}},
		{ID: "mail-sms", Group: "Administration", Title: "Mail / SMS Settings", Description: "Configure secure notification delivery providers and sender identity. Test delivery is not performed from this form.", Sections: []ConfigurationSection{
			section("mail", "Mail settings", "SMTP or provider API configuration for notifications.",
				boolField("mail", "Enable mail delivery", "Use the configured SMTP or provider API.", true),
				selectField("mailTransport", "Mail transport", "How outbound mail is sent.", "smtp", "smtp:SMTP", "api:Provider API"),
				textField("smtpHost", "SMTP host", "Mail server hostname.", "smtp.example.com", false),
				numberField("smtpPort", "SMTP port", "Mail server port.", 587, 1, 65535),
				boolField("mailTLS", "Require mail TLS", "Reject insecure mail-server connections.", true),
				boolField("mailOAuth", "Use OAuth for mail", "Authenticate to the mail provider with OAuth where supported.", false),
				textField("mailFrom", "Sender address", "From address used for product mail.", "noreply@example.com", false),
				textField("mailFromName", "Sender display name", "From display name.", "Directory Desk", false)),
			section("sms", "SMS settings", "SMS adapter configuration for verification and alerts.",
				boolField("sms", "Enable SMS delivery", "Use the configured SMS provider.", false),
				selectField("smsProvider", "SMS provider", "Configured SMS adapter.", "none", "none:None", "twilio:Twilio", "custom:Custom HTTP"),
				textField("smsFrom", "SMS sender ID", "Sender ID or long code used for SMS.", "", false),
				boolField("smsDiagnostics", "Retain SMS diagnostics", "Keep non-content delivery diagnostics for administrators.", true)),
		}},
		{ID: "schedulers", Group: "Administration", Title: "Schedulers and Backups", Description: "Operate synchronizers, report jobs, notifications, and encrypted backups. Jobs are configured here; execution status is reported when schedulers run.", Sections: []ConfigurationSection{
			section("jobs", "Product schedulers", "Directory and dashboard aggregation jobs.",
				boolField("directorySync", "Directory synchronizer", "Refresh users, groups, and organizational units.", true),
				boolField("lockedUsers", "Locked-out users refresh", "Update dashboard and locked-user reports.", true),
				boolField("passwordExpiry", "Password-expiry data refresh", "Update expiry reports and notification candidates.", true),
				selectField("syncFrequency", "Synchronizer frequency", "How often directory sync is scheduled.", "hourly", "hourly:Hourly", "daily:Daily", "manual:Manual only"),
				boolField("allowManualRun", "Allow manual runs", "Permit administrators to trigger configured jobs.", true)),
			section("backups", "Backup settings", "Protected backup schedule and retention.",
				boolField("scheduledBackups", "Enable scheduled backups", "Run backups on the configured cadence.", true),
				boolField("encryptedBackups", "Encrypt backup artifacts", "Require password-protected backup files.", true),
				boolField("verifyIntegrity", "Verify backup integrity", "Check every completed artifact.", true),
				numberField("retentionDays", "Backup retention", "Days to retain backup artifacts.", 30, 1, 3650),
				boolField("siteBasedDC", "Prefer site-based domain controllers", "Use site affinity before policy fallback during sync.", false)),
		}},
		{ID: "integrations", Group: "Administration", Title: "Integrations", Description: "Connect ITSM, SIEM, breached-password, and identity administration systems. Credentials are stored encrypted by the config store; adapters are not invoked from this form.", Sections: []ConfigurationSection{
			section("adapters", "Integration adapters", "Enable configured external systems.",
				boolField("itsm", "ITSM integration", "Forward tickets or incidents to the ITSM adapter.", false),
				boolField("siem", "SIEM integration", "Forward security events to the SIEM adapter.", false),
				boolField("breachedPassword", "Breached-password service", "Use an external breach corpus for password checks.", true),
				boolField("iam", "Identity administration integration", "Connect an external IAM / IGA system.", false)),
			section("operations", "Health and retries", "Operational defaults for configured adapters.",
				boolField("healthChecks", "Run integration health checks", "Continuously validate configured adapters.", true),
				boolField("retry", "Retry transient delivery failures", "Use bounded backoff for temporary errors.", true),
				numberField("retryAttempts", "Retry attempts", "Maximum retries for a transient failure.", 3, 0, 20),
				numberField("healthIntervalMinutes", "Health check interval", "Minutes between adapter health checks.", 15, 1, 1440)),
		}},
		{ID: "privacy-licensing", Group: "Administration", Title: "Security, Privacy, and Licensing", Description: "Control data protection, usage consent, entitlements, and restricted users.", Sections: []ConfigurationSection{
			section("privacy", "Security and privacy", "Protected exports, backups, telemetry, and retention.",
				boolField("protectedReports", "Protect report exports", "Require passwords for scheduled and exported reports.", true),
				boolField("protectedBackups", "Protect database backups", "Encrypt backup artifacts.", true),
				boolField("usageStatistics", "Send anonymous usage statistics", "Share non-identifying product telemetry.", false),
				numberField("dataRetentionDays", "Data retention", "Days to retain operational personal data subject to policy.", 365, 30, 3650)),
			section("licensing", "License management", "Entitlement consumption and inactive-user restriction.",
				boolField("inactiveRestriction", "Restrict inactive users", "Schedule restriction after the configured inactivity threshold.", false),
				numberField("inactiveRestrictionDays", "Inactivity threshold", "Days of inactivity before restriction.", 90, 1, 3650),
				boolField("freeEndpointLicense", "Release Endpoint MFA entitlement", "Free endpoint licenses when a user is restricted.", true),
				boolField("deleteEnrollment", "Remove enrollment on restriction", "Delete enrollment material as part of the restriction transaction.", false),
				numberField("licensedSeats", "Licensed seats", "Configured product seat count for reporting.", 5000, 1, 1000000)),
		}},
	}
	result := make(map[string]ConfigurationDefinition, len(definitions))
	for _, definition := range definitions {
		result[definition.ID] = definition
	}
	return result
}
