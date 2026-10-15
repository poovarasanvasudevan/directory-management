package api

import (
	"encoding/csv"
	"net/http"
	"regexp"

	"directory-management/internal/reports"
	"github.com/go-chi/chi/v5"
)

var reportIDPattern = regexp.MustCompile(`^[a-z0-9-]{1,64}$`)

type ReportsHandler struct{ service reports.Service }

func NewReportsHandler(service reports.Service) *ReportsHandler {
	return &ReportsHandler{service: service}
}
func (h *ReportsHandler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Get("/", h.catalog)
	r.Get("/{reportID}", h.run)
	r.Get("/{reportID}/export", h.export)
	return r
}
func (h *ReportsHandler) catalog(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.Catalog(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"code": "report_catalog_failed", "message": "report catalog is unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"reports": items})
}
func (h *ReportsHandler) run(w http.ResponseWriter, r *http.Request) {
	result, ok := h.result(w, r)
	if ok {
		writeJSON(w, http.StatusOK, result)
	}
}
func (h *ReportsHandler) export(w http.ResponseWriter, r *http.Request) {
	result, ok := h.result(w, r)
	if !ok {
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+result.Report.ID+`.csv"`)
	writer := csv.NewWriter(w)
	_ = writer.Write(result.Columns)
	for _, row := range result.Rows {
		_ = writer.Write(row)
	}
	writer.Flush()
}
func (h *ReportsHandler) result(w http.ResponseWriter, r *http.Request) (reports.Result, bool) {
	id := chi.URLParam(r, "reportID")
	if !reportIDPattern.MatchString(id) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_report", "message": "invalid report identifier"})
		return reports.Result{}, false
	}
	result, err := h.service.Run(r.Context(), id, r.URL.Query().Get("q"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"code": "report_not_found", "message": "report was not found"})
		return reports.Result{}, false
	}
	return result, true
}
