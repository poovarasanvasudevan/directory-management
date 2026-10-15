package api

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"directory-management/internal/audit"
	"directory-management/internal/config"
	"directory-management/internal/directory"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type DirectoryHandler struct {
	cfg     config.ActiveDirectoryConfig
	service directory.Service
	audit   audit.Store
}

func NewDirectoryHandler(cfg config.ActiveDirectoryConfig, service directory.Service, auditStore audit.Store) *DirectoryHandler {
	return &DirectoryHandler{cfg: cfg, service: service, audit: auditStore}
}

func (h *DirectoryHandler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Get("/config", h.getConfig)
	r.Post("/test", h.testConnection)
	r.Get("/organizational-units", h.organizationalUnits)
	r.Get("/groups", h.groups)
	r.Get("/browse", h.browse)
	r.Post("/users", h.createUser)
	r.Post("/groups", h.createGroup)
	r.Get("/entry", h.getEntry)
	r.Put("/entry/attributes", h.updateAttributes)
	r.Post("/entry/enabled", h.setEnabled)
	r.Get("/entry/member-of", h.listMemberOf)
	r.Get("/group/members", h.listMembers)
	r.Post("/group/members", h.addMember)
	r.Delete("/group/members", h.removeMember)
	return r
}

func (h *DirectoryHandler) getConfig(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"directory":             h.cfg,
		"credentialsConfigured": h.cfg.BindPassword != "",
		"tlsVerification":       !h.cfg.SkipTLSVerify,
		"writeEnabled":          h.writesEnabled(),
		"attributeAllowlist":    directory.DefaultAttributeAllowlist,
	})
}

func (h *DirectoryHandler) testConnection(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.ConnectTimeout)
	defer cancel()
	started := time.Now()
	if err := h.service.TestConnection(ctx); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"status": "unavailable", "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "connected", "message": "Bind and directory root lookup succeeded", "latencyMs": time.Since(started).Milliseconds()})
}

func (h *DirectoryHandler) organizationalUnits(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.ConnectTimeout)
	defer cancel()
	units, err := h.service.OrganizationalUnits(ctx)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"organizationalUnits": units})
}

func (h *DirectoryHandler) groups(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.ConnectTimeout)
	defer cancel()
	groups, err := h.service.SearchGroups(ctx, r.URL.Query().Get("q"), 100)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"groups": groups})
}

func (h *DirectoryHandler) browse(w http.ResponseWriter, r *http.Request) {
	parent := strings.TrimSpace(r.URL.Query().Get("parent"))
	if parent == "" {
		parent = h.cfg.BaseDN
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.ConnectTimeout)
	defer cancel()
	nodes, err := h.service.ListChildren(ctx, parent, limit)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"code": "directory_browse_failed", "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"parentDN": parent, "nodes": nodes, "writeEnabled": h.writesEnabled(),
		"attributeAllowlist": directory.DefaultAttributeAllowlist, "baseDN": h.cfg.BaseDN,
	})
}

func (h *DirectoryHandler) getEntry(w http.ResponseWriter, r *http.Request) {
	dn := strings.TrimSpace(r.URL.Query().Get("dn"))
	if dn == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_dn", "message": "dn query parameter is required"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.ConnectTimeout)
	defer cancel()
	entry, err := h.service.GetEntry(ctx, dn)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"code": "entry_not_found", "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"entry": entry, "writeEnabled": h.writesEnabled()})
}

func (h *DirectoryHandler) createUser(w http.ResponseWriter, r *http.Request) {
	if !h.writesEnabled() {
		writeJSON(w, http.StatusForbidden, map[string]string{"code": "directory_read_only", "message": "directory writes are disabled"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
	var request struct {
		ParentDN   string            `json:"parentDN"`
		Name       string            `json:"name"`
		Attributes map[string]string `json:"attributes"`
		Password   string            `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_json", "message": "create user request is invalid"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.ConnectTimeout)
	defer cancel()
	entry, err := h.service.CreateUser(ctx, directory.CreateUserInput{
		ParentDN: request.ParentDN, Name: request.Name, Attributes: request.Attributes, Password: request.Password,
	})
	h.record(r, "directory.user-create", request.Name, err, map[string]any{"parentDN": request.ParentDN})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"code": "user_create_failed", "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"entry": entry})
}

func (h *DirectoryHandler) createGroup(w http.ResponseWriter, r *http.Request) {
	if !h.writesEnabled() {
		writeJSON(w, http.StatusForbidden, map[string]string{"code": "directory_read_only", "message": "directory writes are disabled"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	var request struct {
		ParentDN    string `json:"parentDN"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_json", "message": "create group request is invalid"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.ConnectTimeout)
	defer cancel()
	entry, err := h.service.CreateGroup(ctx, directory.CreateGroupInput{
		ParentDN: request.ParentDN, Name: request.Name, Description: request.Description,
	})
	h.record(r, "directory.group-create", request.Name, err, map[string]any{"parentDN": request.ParentDN})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"code": "group_create_failed", "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"entry": entry})
}

func (h *DirectoryHandler) updateAttributes(w http.ResponseWriter, r *http.Request) {
	if !h.writesEnabled() {
		writeJSON(w, http.StatusForbidden, map[string]string{"code": "directory_read_only", "message": "directory writes are disabled"})
		return
	}
	dn := strings.TrimSpace(r.URL.Query().Get("dn"))
	if dn == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_dn", "message": "dn query parameter is required"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
	var request struct {
		Attributes map[string]string `json:"attributes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request.Attributes == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_json", "message": "attributes object is required"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.ConnectTimeout)
	defer cancel()
	entry, err := h.service.UpdateAttributes(ctx, dn, request.Attributes)
	h.record(r, "directory.attributes-update", dn, err, map[string]any{"keys": mapKeys(request.Attributes)})
	if err != nil {
		status := http.StatusBadGateway
		if strings.Contains(err.Error(), "allowlist") || strings.Contains(err.Error(), "cannot be written") {
			status = http.StatusUnprocessableEntity
		}
		writeJSON(w, status, map[string]string{"code": "attributes_update_failed", "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"entry": entry})
}

func (h *DirectoryHandler) setEnabled(w http.ResponseWriter, r *http.Request) {
	if !h.writesEnabled() {
		writeJSON(w, http.StatusForbidden, map[string]string{"code": "directory_read_only", "message": "directory writes are disabled"})
		return
	}
	dn := strings.TrimSpace(r.URL.Query().Get("dn"))
	if dn == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_dn", "message": "dn query parameter is required"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	var request struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_json", "message": "enabled flag is required"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.ConnectTimeout)
	defer cancel()
	err := h.service.SetUserEnabled(ctx, dn, request.Enabled)
	h.record(r, "directory.user-enabled", dn, err, map[string]any{"enabled": request.Enabled})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"code": "enabled_update_failed", "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "completed", "distinguishedName": dn, "enabled": request.Enabled})
}

func (h *DirectoryHandler) listMembers(w http.ResponseWriter, r *http.Request) {
	dn := strings.TrimSpace(r.URL.Query().Get("dn"))
	if dn == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_dn", "message": "dn query parameter is required"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.ConnectTimeout)
	defer cancel()
	members, err := h.service.ListMembers(ctx, dn)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"code": "members_list_failed", "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"groupDN": dn, "members": members})
}

func (h *DirectoryHandler) listMemberOf(w http.ResponseWriter, r *http.Request) {
	dn := strings.TrimSpace(r.URL.Query().Get("dn"))
	if dn == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_dn", "message": "dn query parameter is required"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.ConnectTimeout)
	defer cancel()
	groups, err := h.service.ListMemberOf(ctx, dn)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"code": "member_of_failed", "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"distinguishedName": dn, "groups": groups})
}

func (h *DirectoryHandler) addMember(w http.ResponseWriter, r *http.Request) {
	if !h.writesEnabled() {
		writeJSON(w, http.StatusForbidden, map[string]string{"code": "directory_read_only", "message": "directory writes are disabled"})
		return
	}
	groupDN := strings.TrimSpace(r.URL.Query().Get("dn"))
	if groupDN == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_dn", "message": "dn query parameter is required"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	var request struct {
		MemberDN string `json:"memberDN"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil || strings.TrimSpace(request.MemberDN) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_json", "message": "memberDN is required"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.ConnectTimeout)
	defer cancel()
	err := h.service.AddMember(ctx, groupDN, request.MemberDN)
	h.record(r, "directory.member-add", groupDN, err, map[string]any{"memberDN": request.MemberDN})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"code": "member_add_failed", "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "completed", "groupDN": groupDN, "memberDN": request.MemberDN})
}

func (h *DirectoryHandler) removeMember(w http.ResponseWriter, r *http.Request) {
	if !h.writesEnabled() {
		writeJSON(w, http.StatusForbidden, map[string]string{"code": "directory_read_only", "message": "directory writes are disabled"})
		return
	}
	groupDN := strings.TrimSpace(r.URL.Query().Get("dn"))
	if groupDN == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_dn", "message": "dn query parameter is required"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	var request struct {
		MemberDN string `json:"memberDN"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil || strings.TrimSpace(request.MemberDN) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_json", "message": "memberDN is required"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.ConnectTimeout)
	defer cancel()
	err := h.service.RemoveMember(ctx, groupDN, request.MemberDN)
	h.record(r, "directory.member-remove", groupDN, err, map[string]any{"memberDN": request.MemberDN})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"code": "member_remove_failed", "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "completed", "groupDN": groupDN, "memberDN": request.MemberDN})
}

func (h *DirectoryHandler) writesEnabled() bool {
	return strings.EqualFold(h.cfg.Mode, "mock") || h.cfg.WriteEnabled
}

func (h *DirectoryHandler) record(r *http.Request, action, targetID string, operationError error, metadata map[string]any) {
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
	h.audit.Record(audit.Event{
		Actor: "admin-api", Action: action, TargetType: "directory-entry", TargetID: targetID,
		Outcome: outcome, RequestID: middleware.GetReqID(r.Context()), SourceIP: host, Metadata: metadata,
		OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
	})
}

func mapKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
