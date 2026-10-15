package api

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"directory-management/internal/audit"
	"directory-management/internal/config"
	"directory-management/internal/directory"
	"directory-management/internal/policies"
	"github.com/go-chi/chi/v5"
)

type DashboardHandler struct {
	cfg      config.ActiveDirectoryConfig
	service  directory.Service
	audit    audit.Store
	policies policies.Store
}

func NewDashboardHandler(cfg config.ActiveDirectoryConfig, service directory.Service, auditStore audit.Store, policyStore policies.Store) *DashboardHandler {
	return &DashboardHandler{cfg: cfg, service: service, audit: auditStore, policies: policyStore}
}

func (h *DashboardHandler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Get("/", h.summary)
	return r
}

func (h *DashboardHandler) summary(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.ConnectTimeout)
	defer cancel()

	directoryError := ""
	users, err := h.service.SearchUsers(ctx, "", 200)
	if err != nil {
		directoryError = err.Error()
		users = nil
	}
	locked, expired, disabled := 0, 0, 0
	for _, user := range users {
		if user.Locked {
			locked++
		}
		if user.PasswordExpired {
			expired++
		}
		if !user.Enabled {
			disabled++
		}
	}

	policyItems, _ := h.policies.List(ctx)
	activePolicies := 0
	for _, policy := range policyItems {
		if policy.Status == "Active" {
			activePolicies++
		}
	}

	resets := h.audit.List(audit.Query{Action: "directory.password-reset", Limit: 100})
	unlocks := h.audit.List(audit.Query{Action: "directory.account-unlock", Limit: 100})

	writeJSON(w, http.StatusOK, map[string]any{
		"generatedAt": time.Now().UTC().Format(time.RFC3339),
		"directory": map[string]any{
			"name":         h.cfg.Name,
			"mode":         h.cfg.Mode,
			"host":         h.cfg.Host,
			"writeEnabled": h.cfg.WriteEnabled || h.cfg.Mode == "mock",
			"error":        directoryError,
		},
		"tiles": map[string]any{
			"lockedUsers":          locked,
			"expiredPasswords":     expired,
			"disabledUsers":        disabled,
			"sampledUsers":         len(users),
			"activePolicies":       activePolicies,
			"totalPolicies":        len(policyItems),
			"recentPasswordResets": len(resets),
			"recentUnlocks":        len(unlocks),
		},
		"enrollment": map[string]any{
			"status":  "unavailable",
			"message": "Enrollment coverage requires MFA enrollment (later wave).",
		},
		"attention": attentionItems(directoryError, locked, expired, activePolicies, len(policyItems)),
		"activity": []map[string]any{
			{"label": "Reset password", "value": len(resets)},
			{"label": "Unlock account", "value": len(unlocks)},
		},
	})
}

func attentionItems(directoryError string, locked, expired, activePolicies, totalPolicies int) []map[string]string {
	items := make([]map[string]string, 0, 4)
	if directoryError != "" {
		items = append(items, map[string]string{"title": "Directory bind failed", "detail": directoryError})
	}
	if locked > 0 {
		items = append(items, map[string]string{"title": "Locked accounts", "detail": strconv.Itoa(locked) + " users currently locked in the sampled directory search"})
	}
	if expired > 0 {
		items = append(items, map[string]string{"title": "Expired passwords", "detail": strconv.Itoa(expired) + " accounts report pwdLastSet=0 / expired password"})
	}
	if totalPolicies == 0 {
		items = append(items, map[string]string{"title": "No self-service policies", "detail": "Create an OU/group-scoped policy before enabling recovery writes"})
	} else if activePolicies == 0 {
		items = append(items, map[string]string{"title": "No active policies", "detail": "All policies are paused or under review"})
	}
	if len(items) == 0 {
		items = append(items, map[string]string{"title": "Directory healthy", "detail": "No locked or expired accounts in the current sample"})
	}
	return items
}
