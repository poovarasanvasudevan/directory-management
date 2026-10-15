package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"directory-management/internal/api"
	"directory-management/internal/audit"
	"directory-management/internal/config"
	"directory-management/internal/configstore"
	"directory-management/internal/database"
	"directory-management/internal/directory"
	"directory-management/internal/policies"
	"directory-management/internal/reports"
	"directory-management/internal/server"
)

func main() {
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config.yaml"
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		slog.Error("load configuration", "error", err)
		os.Exit(1)
	}
	if address := os.Getenv("SERVER_ADDRESS"); address != "" {
		cfg.Server.Address = address
	}

	db, err := database.Open(cfg.Database.DSN)
	if err != nil {
		slog.Error("open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	migrationsPath := os.Getenv("MIGRATIONS_PATH")
	if migrationsPath == "" {
		migrationsPath = "file://" + filepath.ToSlash(filepath.Join("migrations"))
	}
	if err := database.Migrate(db, migrationsPath); err != nil {
		slog.Error("run migrations", "error", err)
		os.Exit(1)
	}

	var directoryService directory.Service = directory.NewMockService()
	if strings.EqualFold(cfg.ActiveDirectory.Mode, "ldap") {
		directoryService = directory.NewLDAPService(cfg.ActiveDirectory)
	}
	slog.Info("directory provider ready",
		"mode", cfg.ActiveDirectory.Mode,
		"name", cfg.ActiveDirectory.Name,
		"host", cfg.ActiveDirectory.Host,
		"security", cfg.ActiveDirectory.Security,
		"baseDN", cfg.ActiveDirectory.BaseDN,
		"writeEnabled", cfg.ActiveDirectory.WriteEnabled,
	)

	policyStore := policies.NewMySQLStore(db)
	auditStore := audit.NewMySQLStore(db)
	configStore := configstore.NewMySQLStore(db)

	directoryHandler := api.NewDirectoryHandler(cfg.ActiveDirectory, directoryService, auditStore)
	reportsHandler := api.NewReportsHandler(reports.NewDirectoryService(directoryService, auditStore, cfg.ActiveDirectory.Name))
	capabilitiesHandler := api.NewCapabilitiesHandler(configStore)
	policyHandler := api.NewPolicyHandler(policyStore)
	settingsHandler := api.NewSettingsHandler(configStore)
	configurationHandler := api.NewConfigurationHandler(configStore, auditStore)
	usersHandler := api.NewUsersHandler(cfg.ActiveDirectory, directoryService, auditStore, policyStore)
	auditHandler := api.NewAuditHandler(auditStore)
	dashboardHandler := api.NewDashboardHandler(cfg.ActiveDirectory, directoryService, auditStore, policyStore)

	handler := server.NewRouter(server.Dependencies{
		DirectoryAPI:     directoryHandler.Routes(),
		ReportsAPI:       reportsHandler.Routes(),
		CapabilitiesAPI:  capabilitiesHandler.Routes(),
		PoliciesAPI:      policyHandler.Routes(),
		SettingsAPI:      settingsHandler.Routes(),
		ConfigurationAPI: configurationHandler.Routes(),
		UsersAPI:         usersHandler.Routes(),
		AuditAPI:         auditHandler.Routes(),
		DashboardAPI:     dashboardHandler.Routes(),
		RequestTimeout:   cfg.Server.RequestTimeout,
	})
	httpServer := &http.Server{
		Addr:              cfg.Server.Address,
		Handler:           handler,
		ReadTimeout:       cfg.Server.ReadTimeout,
		WriteTimeout:      cfg.Server.WriteTimeout,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go func() {
		slog.Info("Directory Desk listening", "address", cfg.Server.Address, "environment", cfg.App.Environment)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP server failed", "error", err)
			stop()
		}
	}()
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", "error", err)
	}
}
