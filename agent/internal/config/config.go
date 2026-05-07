package config

import "os"

type Config struct {
	CoreURL  string
	AgentID  string
	Token    string
	Hostname string
}

func Load() *Config {
	hostname, _ := os.Hostname()
	return &Config{
		CoreURL:  getenv("WP_CORE_URL", "ws://127.0.0.1:8080/api/agent/ws"),
		AgentID:  getenv("WP_AGENT_ID", hostname),
		Token:    getenv("WP_AGENT_TOKEN", "dev-shared-token"),
		Hostname: hostname,
	}
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
