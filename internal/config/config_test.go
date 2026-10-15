package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadExpandsPasswordEnvironmentVariable(t *testing.T) {
	t.Setenv("AD_BIND_PASSWORD", "test-secret")
	path := filepath.Join(t.TempDir(), "config.yaml")
	data := []byte("activeDirectory:\n  host: ad.example.com\n  baseDN: DC=example,DC=com\n  bindDN: CN=svc,DC=example,DC=com\n  bindPassword: ${AD_BIND_PASSWORD}\n")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ActiveDirectory.BindPassword != "test-secret" {
		t.Fatal("password environment variable was not expanded")
	}
}

func TestLoadAppliesRuntimeEnvironmentOverrides(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("HTTP_ADDR", "127.0.0.1:4199")
	t.Setenv("ADMIN_API_TOKEN", "test-admin-token")
	t.Setenv("DIRECTORY_MODE", "mock")
	t.Setenv("DIRECTORY_WRITE_ENABLED", "true")
	path := filepath.Join(t.TempDir(), "config.yaml")
	data := []byte("app:\n  environment: production\n  adminAPIToken: file-token\nserver:\n  address: :3144\nactiveDirectory:\n  mode: ldap\n  host: ad.example.com\n  security: ldaps\n  baseDN: DC=example,DC=com\n  bindDN: CN=svc,DC=example,DC=com\n")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.App.Environment != "test" || cfg.App.AdminAPIToken != "test-admin-token" || cfg.Server.Address != "127.0.0.1:4199" {
		t.Fatalf("application overrides were not applied: %#v %#v", cfg.App, cfg.Server)
	}
	if cfg.ActiveDirectory.Mode != "mock" || !cfg.ActiveDirectory.WriteEnabled {
		t.Fatalf("directory overrides were not applied: %#v", cfg.ActiveDirectory)
	}
}
