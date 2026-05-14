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
	HTTPListen    string        `yaml:"http_listen"`
	PublicBaseURL string        `yaml:"public_base_url"`
	MCPEndpoint   string        `yaml:"mcp_endpoint"`
	ReadTimeout   time.Duration `yaml:"read_timeout"`
	WriteTimeout  time.Duration `yaml:"write_timeout"`
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
	Issuer                       string `yaml:"issuer"`
	Audience                     string `yaml:"audience"`
	JWKSURL                      string `yaml:"jwks_url"`
	ProtectedResourceMetadataURL string `yaml:"protected_resource_metadata_url"`
}

type GatewayConfig struct {
	BaseURL     string `yaml:"base_url"`
	AdminAPIKey string `yaml:"admin_api_key"`
}

type AuthConfig struct {
	DefaultChallengeScope string            `yaml:"default_challenge_scope"`
	ScopesSupported       []string          `yaml:"scopes_supported"`
	ToolScopeMap          map[string]string `yaml:"tool_scope_map"`
	StaticBearer          string            `yaml:"static_bearer"`
	StdioClientID         string            `yaml:"stdio_client_id"`
}

type AuditConfig struct {
	Backend     string `yaml:"backend"`
	SQLitePath  string `yaml:"sqlite_path"`
	PostgresURL string `yaml:"postgres_url"`
}

func (c *Config) ApplyDefaults() {
	if c.Server.HTTPListen == "" {
		c.Server.HTTPListen = ":9280"
	}
	if c.Server.PublicBaseURL == "" {
		c.Server.PublicBaseURL = "http://127.0.0.1:9280"
	}
	if c.Server.MCPEndpoint == "" {
		c.Server.MCPEndpoint = "/mcp"
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
	if c.Audit.SQLitePath == "" {
		c.Audit.SQLitePath = "./data/mcp-audit.db"
	}
	if c.OAuth.ProtectedResourceMetadataURL == "" {
		c.OAuth.ProtectedResourceMetadataURL = strings.TrimRight(c.Server.PublicBaseURL, "/") + "/.well-known/oauth-protected-resource"
	}
	if c.Auth.DefaultChallengeScope == "" {
		c.Auth.DefaultChallengeScope = "mcp:tools"
	}
	if len(c.Auth.ScopesSupported) == 0 {
		c.Auth.ScopesSupported = []string{"mcp:tools", "mcp:read", "mcp:write"}
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
	case "stdout", "sqlite", "postgres":
	default:
		return fmt.Errorf("unsupported audit.backend: %q", c.Audit.Backend)
	}
	if c.Audit.Backend == "sqlite" && strings.TrimSpace(c.Audit.SQLitePath) == "" {
		return errors.New("audit.sqlite_path cannot be empty when audit.backend=sqlite")
	}
	if c.Audit.Backend == "postgres" && strings.TrimSpace(c.Audit.PostgresURL) == "" {
		return errors.New("audit.postgres_url cannot be empty when audit.backend=postgres")
	}
	return nil
}
