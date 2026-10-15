package api

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"directory-management/internal/audit"
	"directory-management/internal/config"
	"directory-management/internal/directory"
	"directory-management/internal/policies"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

var directoryUserIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)

type UsersHandler struct {
	cfg      config.ActiveDirectoryConfig
	service  directory.Service
	audit    audit.Store
	policies policies.Store
}

func NewUsersHandler(cfg config.ActiveDirectoryConfig, service directory.Service, auditStore audit.Store, policyStore policies.Store) *UsersHandler {
	return &UsersHandler{cfg: cfg, service: service, audit: auditStore, policies: policyStore}
}

func (h *UsersHandler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Get("/", h.list)
	r.Post("/{userID}/password-reset", h.resetPassword)
	r.Post("/{userID}/unlock", h.unlock)
	return r
}

func (h *UsersHandler) list(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 200 {
		limit = 50
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.ConnectTimeout)
	defer cancel()
	users, err := h.service.SearchUsers(ctx, r.URL.Query().Get("q"), limit)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"code": "directory_search_failed", "message": err.Error()})
		return
	}
	rows := make([]map[string]any, 0, len(users))
	for _, user := range users {
		row := map[string]any{
			"id": user.ID, "displayName": user.DisplayName, "email": user.Email,
			"distinguishedName": user.DistinguishedName, "organizationalUnit": user.OrganizationalUnit,
			"enabled": user.Enabled, "locked": user.Locked, "passwordExpired": user.PasswordExpired,
			"lastLogon": user.LastLogon, "allowedActions": []string{},
		}
		if h.policies != nil {
			matched, evalErr := h.policies.Evaluate(ctx, user.DistinguishedName, nil)
			if evalErr == nil && matched != nil {
				row["matchedPolicy"] = map[string]any{"id": matched.ID, "name": matched.Name, "priority": matched.Priority}
				row["allowedActions"] = matched.Permissions
			}
		}
		rows = append(rows, row)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"users": rows, "count": len(rows),
		"writeEnabled": strings.EqualFold(h.cfg.Mode, "mock") || h.cfg.WriteEnabled,
	})
}

func (h *UsersHandler) resetPassword(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userID")
	if !directoryUserIDPattern.MatchString(userID) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_user", "message": "directory user identifier is invalid"})
		return
	}
	if !strings.EqualFold(h.cfg.Mode, "mock") && !h.cfg.WriteEnabled {
		writeJSON(w, http.StatusForbidden, map[string]string{"code": "directory_read_only", "message": "directory writes are disabled; set DIRECTORY_WRITE_ENABLED=true after granting the service account least-privilege reset rights"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	var request struct {
		Password    string `json:"password"`
		ForceChange bool   `json:"forceChange"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_json", "message": "password reset request is invalid"})
		return
	}
	if utf8.RuneCountInString(request.Password) < 12 || utf8.RuneCountInString(request.Password) > 128 {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"code": "invalid_password", "message": "temporary password must contain 12 to 128 characters"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.ConnectTimeout)
	defer cancel()
	err := h.service.ResetPassword(ctx, userID, request.Password, request.ForceChange)
	h.record(r, "directory.password-reset", userID, err, map[string]any{"forceChange": request.ForceChange})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"code": "password_reset_failed", "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "completed", "userId": userID, "forceChange": request.ForceChange})
}

func (h *UsersHandler) unlock(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userID")
	if !directoryUserIDPattern.MatchString(userID) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_user", "message": "directory user identifier is invalid"})
		return
	}
	if !strings.EqualFold(h.cfg.Mode, "mock") && !h.cfg.WriteEnabled {
		writeJSON(w, http.StatusForbidden, map[string]string{"code": "directory_read_only", "message": "directory writes are disabled; set DIRECTORY_WRITE_ENABLED=true after granting least-privilege unlock rights"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.ConnectTimeout)
	defer cancel()
	err := h.service.UnlockAccount(ctx, userID)
	h.record(r, "directory.account-unlock", userID, err, nil)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"code": "account_unlock_failed", "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "completed", "userId": userID})
}

func (h *UsersHandler) record(r *http.Request, action, userID string, operationError error, metadata map[string]any) {
	if h.audit == nil {
		return
	}
	outcome := "success"
	if operationError != nil {
		outcome = "failure"
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	h.audit.Record(audit.Event{Actor: "admin-api", Action: action, TargetType: "directory-user", TargetID: userID, Outcome: outcome, RequestID: middleware.GetReqID(r.Context()), SourceIP: host, Metadata: metadata, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano)})
}
