package config

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

type Config struct {
	Server        ServerConfig        `yaml:"server"`
	Logging       LoggingConfig       `yaml:"logging"`
	Observability ObservabilityConfig `yaml:"observability"`
	OAuth         OAuthConfig         `yaml:"oauth"`
	Gateway       GatewayConfig       `yaml:"gateway"`
	Auth          AuthConfig          `yaml:"auth"`
	Audit         AuditConfig         `yaml:"audit"`
}

type ServerConfig struct {
	HTTPListen   string        `yaml:"http_listen"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
}

type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

type ObservabilityConfig struct {
	MetricsEnabled bool   `yaml:"metrics_enabled"`
	MetricsPath    string `yaml:"metrics_path"`
}

type OAuthConfig struct {
	Issuer   string `yaml:"issuer"`
	Audience string `yaml:"audience"`
	JWKSURL  string `yaml:"jwks_url"`
}

type GatewayConfig struct {
	BaseURL     string `yaml:"base_url"`
	AdminAPIKey string `yaml:"admin_api_key"`
}

type AuthConfig struct {
	ToolScopeMap map[string]string `yaml:"tool_scope_map"`
}

type AuditConfig struct {
	Backend string `yaml:"backend"`
}

func (c *Config) ApplyDefaults() {
	if c.Server.HTTPListen == "" {
		c.Server.HTTPListen = ":9280"
	}
	if c.Server.ReadTimeout == 0 {
		c.Server.ReadTimeout = 10 * time.Second
	}
	if c.Server.WriteTimeout == 0 {
		c.Server.WriteTimeout = 10 * time.Second
	}
	if c.Logging.Level == "" {
		c.Logging.Level = "info"
	}
	if c.Logging.Format == "" {
		c.Logging.Format = "json"
	}
	if c.Observability.MetricsPath == "" {
		c.Observability.MetricsPath = "/metrics"
	}
	if c.Audit.Backend == "" {
		c.Audit.Backend = "stdout"
	}
}

func (c Config) Validate() error {
	if c.OAuth.Issuer == "" {
		return errors.New("oauth.issuer cannot be empty")
	}
	if c.OAuth.Audience == "" {
		return errors.New("oauth.audience cannot be empty")
	}
	if c.OAuth.JWKSURL == "" {
		return errors.New("oauth.jwks_url cannot be empty")
	}
	if c.Gateway.BaseURL == "" {
		return errors.New("gateway.base_url cannot be empty")
	}
	if c.Gateway.AdminAPIKey == "" {
		return errors.New("gateway.admin_api_key cannot be empty")
	}
	if len(c.Auth.ToolScopeMap) == 0 {
		return errors.New("auth.tool_scope_map cannot be empty")
	}
	for tool, scope := range c.Auth.ToolScopeMap {
		if strings.TrimSpace(tool) == "" || strings.TrimSpace(scope) == "" {
			return fmt.Errorf("invalid tool_scope_map entry: %q=%q", tool, scope)
		}
	}
	switch c.Audit.Backend {
	case "stdout", "sqlite":
	default:
		return fmt.Errorf("unsupported audit.backend: %q", c.Audit.Backend)
	}
	return nil
}
