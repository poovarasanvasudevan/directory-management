package server

import (
	"encoding/json"
	"net/http"
	"time"

	"directory-management/frontend"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Dependencies struct {
	DirectoryAPI     http.Handler
	ReportsAPI       http.Handler
	CapabilitiesAPI  http.Handler
	PoliciesAPI      http.Handler
	SettingsAPI      http.Handler
	ConfigurationAPI http.Handler
	UsersAPI         http.Handler
	AuditAPI         http.Handler
	DashboardAPI     http.Handler
	RequestTimeout   time.Duration
}

func NewRouter(deps Dependencies) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Recoverer)
	r.Use(middleware.Timeout(deps.RequestTimeout))
	r.Use(securityHeaders)
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { writeStatus(w, http.StatusOK, "ok") })
	r.Get("/readyz", func(w http.ResponseWriter, _ *http.Request) { writeStatus(w, http.StatusOK, "ready") })
	r.Route("/api/v1", func(r chi.Router) {
		if deps.DirectoryAPI != nil {
			r.Mount("/admin/directories", deps.DirectoryAPI)
		}
		if deps.ReportsAPI != nil {
			r.Mount("/admin/reports", deps.ReportsAPI)
		}
		if deps.CapabilitiesAPI != nil {
			r.Mount("/admin/capabilities", deps.CapabilitiesAPI)
		}
		if deps.PoliciesAPI != nil {
			r.Mount("/admin/policies", deps.PoliciesAPI)
		}
		if deps.SettingsAPI != nil {
			r.Mount("/admin/settings", deps.SettingsAPI)
		}
		if deps.ConfigurationAPI != nil {
			r.Mount("/admin/configuration", deps.ConfigurationAPI)
		}
		if deps.UsersAPI != nil {
			r.Mount("/admin/users", deps.UsersAPI)
		}
		if deps.AuditAPI != nil {
			r.Mount("/admin/audit-events", deps.AuditAPI)
		}
		if deps.DashboardAPI != nil {
			r.Mount("/admin/dashboard", deps.DashboardAPI)
		}
	})
	r.Handle("/*", frontend.Handler())
	return r
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'")
		next.ServeHTTP(w, r)
	})
}

func writeStatus(w http.ResponseWriter, status int, state string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": state})
}
