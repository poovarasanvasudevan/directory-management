package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"directory-management/internal/policies"
	"github.com/go-chi/chi/v5"
)

type PolicyHandler struct {
	store policies.Store
}

func NewPolicyHandler(store policies.Store) *PolicyHandler {
	return &PolicyHandler{store: store}
}

func (h *PolicyHandler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Get("/", h.list)
	r.Post("/", h.create)
	r.Put("/reorder", h.reorder)
	r.Put("/{policyID}", h.update)
	r.Delete("/{policyID}", h.delete)
	r.Post("/{policyID}/clone", h.clone)
	return r
}

func (h *PolicyHandler) list(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.List(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"code": "policy_list_failed", "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policies": items})
}

func (h *PolicyHandler) create(w http.ResponseWriter, r *http.Request) {
	policy, ok := decodePolicy(w, r)
	if !ok {
		return
	}
	created, err := h.store.Create(r.Context(), policy)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"code": "policy_create_failed", "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (h *PolicyHandler) update(w http.ResponseWriter, r *http.Request) {
	id, ok := policyID(w, r)
	if !ok {
		return
	}
	policy, ok := decodePolicy(w, r)
	if !ok {
		return
	}
	policy.ID = id
	updated, err := h.store.Update(r.Context(), policy)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeJSON(w, http.StatusNotFound, map[string]string{"code": "not_found", "message": "policy not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"code": "policy_update_failed", "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (h *PolicyHandler) delete(w http.ResponseWriter, r *http.Request) {
	id, ok := policyID(w, r)
	if !ok {
		return
	}
	if err := h.store.Delete(r.Context(), id); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeJSON(w, http.StatusNotFound, map[string]string{"code": "not_found", "message": "policy not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"code": "policy_delete_failed", "message": err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *PolicyHandler) clone(w http.ResponseWriter, r *http.Request) {
	id, ok := policyID(w, r)
	if !ok {
		return
	}
	cloned, err := h.store.Clone(r.Context(), id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeJSON(w, http.StatusNotFound, map[string]string{"code": "not_found", "message": "policy not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"code": "policy_clone_failed", "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, cloned)
}

func (h *PolicyHandler) reorder(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
	var request struct {
		OrderedIDs []int64 `json:"orderedIds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil || len(request.OrderedIDs) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_json", "message": "orderedIds is required"})
		return
	}
	if err := h.store.Reorder(r.Context(), request.OrderedIDs); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"code": "policy_reorder_failed", "message": err.Error()})
		return
	}
	items, err := h.store.List(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"code": "policy_list_failed", "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policies": items})
}

func decodePolicy(w http.ResponseWriter, r *http.Request) (policies.Policy, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	var policy policies.Policy
	if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_json", "message": "request body is not valid JSON"})
		return policies.Policy{}, false
	}
	policy.Name, policy.Scope, policy.Domain = strings.TrimSpace(policy.Name), strings.TrimSpace(policy.Scope), strings.TrimSpace(policy.Domain)
	if policy.Name == "" || len(policy.Name) > 100 || policy.Scope == "" || policy.Domain == "" || len(policy.Permissions) == 0 {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"code": "invalid_policy", "message": "name, scope, domain, and at least one permission are required"})
		return policies.Policy{}, false
	}
	if policy.Status != "Active" && policy.Status != "Paused" && policy.Status != "Review" {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"code": "invalid_status", "message": "status must be Active, Paused, or Review"})
		return policies.Policy{}, false
	}
	if policy.OUDNs == nil {
		policy.OUDNs = []string{}
	}
	if policy.GroupDNs == nil {
		policy.GroupDNs = []string{}
	}
	return policy, true
}

func policyID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "policyID"), 10, 64)
	if err != nil || id < 1 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_id", "message": "policy ID is invalid"})
		return 0, false
	}
	return id, true
}
