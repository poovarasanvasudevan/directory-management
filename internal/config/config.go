package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	App             AppConfig             `yaml:"app"`
	Server          ServerConfig          `yaml:"server"`
	Database        DatabaseConfig        `yaml:"database"`
	Redis           RedisConfig           `yaml:"redis"`
	ActiveDirectory ActiveDirectoryConfig `yaml:"activeDirectory"`
}
type AppConfig struct {
	Environment   string `yaml:"environment"`
	AdminAPIToken string `yaml:"adminAPIToken"`
}
type ServerConfig struct {
	Address        string        `yaml:"address"`
	ReadTimeout    time.Duration `yaml:"readTimeout"`
	WriteTimeout   time.Duration `yaml:"writeTimeout"`
	RequestTimeout time.Duration `yaml:"requestTimeout"`
}
type DatabaseConfig struct {
	DSN string `yaml:"dsn"`
}
type RedisConfig struct {
	Address string `yaml:"address"`
}
type ActiveDirectoryConfig struct {
	Mode            string        `yaml:"mode" json:"mode"`
	Name            string        `yaml:"name" json:"name"`
	Host            string        `yaml:"host" json:"host"`
	Port            int           `yaml:"port" json:"port"`
	Security        string        `yaml:"security" json:"security"`
	SkipTLSVerify   bool          `yaml:"skipTLSVerify" json:"-"`
	BaseDN          string        `yaml:"baseDN" json:"baseDN"`
	BindDN          string        `yaml:"bindDN" json:"bindDN"`
	BindPassword    string        `yaml:"bindPassword" json:"-"`
	WriteEnabled    bool          `yaml:"writeEnabled" json:"writeEnabled"`
	UserSearchBase  string        `yaml:"userSearchBase" json:"userSearchBase"`
	GroupSearchBase string        `yaml:"groupSearchBase" json:"groupSearchBase"`
	ConnectTimeout  time.Duration `yaml:"connectTimeout" json:"-"`
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	expanded := os.Expand(string(data), func(key string) string { return os.Getenv(key) })
	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	if raw, ok := os.LookupEnv("APP_ENV"); ok && strings.TrimSpace(raw) != "" {
		cfg.App.Environment = strings.TrimSpace(raw)
	}
	if raw, ok := os.LookupEnv("ADMIN_API_TOKEN"); ok && raw != "" {
		cfg.App.AdminAPIToken = raw
	}
	if raw, ok := os.LookupEnv("HTTP_ADDR"); ok && strings.TrimSpace(raw) != "" {
		cfg.Server.Address = strings.TrimSpace(raw)
	}
	if cfg.Server.Address == "" {
		cfg.Server.Address = ":3144"
	}
	if cfg.Server.ReadTimeout == 0 {
		cfg.Server.ReadTimeout = 15 * time.Second
	}
	if cfg.Server.WriteTimeout == 0 {
		cfg.Server.WriteTimeout = 30 * time.Second
	}
	if cfg.Server.RequestTimeout == 0 {
		cfg.Server.RequestTimeout = 30 * time.Second
	}
	if cfg.App.Environment == "" {
		cfg.App.Environment = "development"
	}
	if cfg.ActiveDirectory.Mode == "" {
		cfg.ActiveDirectory.Mode = "mock"
	}
	if cfg.ActiveDirectory.Port == 0 {
		cfg.ActiveDirectory.Port = 636
	}
	if cfg.ActiveDirectory.Security == "" {
		cfg.ActiveDirectory.Security = "ldaps"
	}
	if cfg.ActiveDirectory.ConnectTimeout == 0 {
		cfg.ActiveDirectory.ConnectTimeout = 10 * time.Second
	}
	if raw, ok := os.LookupEnv("DIRECTORY_MODE"); ok && strings.TrimSpace(raw) != "" {
		cfg.ActiveDirectory.Mode = strings.TrimSpace(raw)
	}
	if raw, ok := os.LookupEnv("DIRECTORY_WRITE_ENABLED"); ok {
		cfg.ActiveDirectory.WriteEnabled = strings.EqualFold(raw, "true") || raw == "1"
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
func (cfg Config) Validate() error {
	ad := cfg.ActiveDirectory
	if ad.Host == "" {
		return fmt.Errorf("activeDirectory.host is required")
	}
	if ad.BaseDN == "" {
		return fmt.Errorf("activeDirectory.baseDN is required")
	}
	if ad.BindDN == "" {
		return fmt.Errorf("activeDirectory.bindDN is required")
	}
	s := strings.ToLower(ad.Security)
	if s != "ldap" && s != "ldaps" && s != "starttls" {
		return fmt.Errorf("activeDirectory.security must be ldap, ldaps, or starttls")
	}
	return nil
}
