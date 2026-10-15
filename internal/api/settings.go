package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"directory-management/internal/configstore"
	"github.com/go-chi/chi/v5"
)

type SettingsHandler struct {
	store    configstore.Store
	defaults map[string]map[string]any
}

func NewSettingsHandler(store configstore.Store) *SettingsHandler {
	return &SettingsHandler{
		store: store,
		defaults: map[string]map[string]any{
			"login":           {"showWebDirectory": true, "showMobileDirectory": true, "preventAccountDiscovery": true, "requireCaptcha": false, "audioCaptcha": false, "hideHelp": false},
			"password-policy": {"minimumLength": float64(12), "requireUppercase": true, "requireLowercase": true, "requireNumber": true, "requireSpecial": true, "breachedPasswordCheck": true},
			"anti-hacking":    {"attemptLimit": float64(5), "blockMinutes": float64(15), "sessionMinutes": float64(30), "notifyUser": true, "restrictInactiveUsers": false},
		},
	}
}

func (h *SettingsHandler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Get("/{section}", h.get)
	r.Put("/{section}", h.put)
	return r
}

func (h *SettingsHandler) moduleKey(section string) string { return "settings:" + section }

func (h *SettingsHandler) get(w http.ResponseWriter, r *http.Request) {
	section := strings.TrimSpace(chi.URLParam(r, "section"))
	defaults, known := h.defaults[section]
	profile, err := h.store.Get(r.Context(), h.moduleKey(section))
	if err != nil {
		if !known {
			writeJSON(w, http.StatusNotFound, map[string]string{"code": "not_found", "message": "settings section not found"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"section": section, "settings": cloneSettings(defaults)})
		return
	}
	merged := cloneSettings(defaults)
	for key, value := range profile.Values {
		merged[key] = value
	}
	writeJSON(w, http.StatusOK, map[string]any{"section": section, "settings": merged, "updatedAt": profile.UpdatedAt})
}

func (h *SettingsHandler) put(w http.ResponseWriter, r *http.Request) {
	section := strings.TrimSpace(chi.URLParam(r, "section"))
	if section == "" || len(section) > 64 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_section", "message": "settings section is invalid"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	var request struct {
		Settings map[string]any `json:"settings"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request.Settings == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_json", "message": "settings object is required"})
		return
	}
	profile, err := h.store.Put(r.Context(), h.moduleKey(section), cloneSettings(request.Settings))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"code": "settings_save_failed", "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"section": section, "settings": profile.Values, "updatedAt": profile.UpdatedAt})
}

func cloneSettings(source map[string]any) map[string]any {
	result := make(map[string]any, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}
