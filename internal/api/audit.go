package api

import (
	"net/http"
	"strconv"

	"directory-management/internal/audit"
	"github.com/go-chi/chi/v5"
)

type AuditHandler struct{ store audit.Store }

func NewAuditHandler(store audit.Store) *AuditHandler { return &AuditHandler{store: store} }

func (h *AuditHandler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Get("/", h.list)
	return r
}

func (h *AuditHandler) list(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	events := h.store.List(audit.Query{Search: r.URL.Query().Get("q"), Action: r.URL.Query().Get("action"), Outcome: r.URL.Query().Get("outcome"), Limit: limit})
	writeJSON(w, http.StatusOK, map[string]any{"events": events, "count": len(events)})
}
