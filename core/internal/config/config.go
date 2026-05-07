package config

import (
	"os"
	"strings"
)

type Config struct {
	Env              string
	HTTPAddr         string
	WSPath           string
	AgentToken       string
	JWTSecret        string
	DataDir          string
	AuditDir         string
	AdminUser        string
	AdminPass        string
	AllowedOrigins   []string
	TLSCert          string
	TLSKey           string
	FSAllowSensitive bool
}

func Load() *Config {
	return &Config{
		Env:              getenv("WP_ENV", "development"),
		HTTPAddr:         getenv("WP_HTTP_ADDR", ":8080"),
		WSPath:           getenv("WP_WS_PATH", "/api/agent/ws"),
		AgentToken:       getenv("WP_AGENT_TOKEN", "dev-shared-token"),
		JWTSecret:        getenv("WP_JWT_SECRET", "dev-secret-change-me"),
		DataDir:          getenv("WP_DATA_DIR", "./data"),
		AuditDir:         getenv("WP_AUDIT_DIR", "./data/audit"),
		AdminUser:        os.Getenv("WP_ADMIN_USER"),
		AdminPass:        os.Getenv("WP_ADMIN_PASS"),
		AllowedOrigins:   splitCSV(os.Getenv("WP_ALLOWED_ORIGINS")),
		TLSCert:          os.Getenv("WP_TLS_CERT"),
		TLSKey:           os.Getenv("WP_TLS_KEY"),
		FSAllowSensitive: os.Getenv("WP_FS_ALLOW_SENSITIVE") == "true",
	}
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
