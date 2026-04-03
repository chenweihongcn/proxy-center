package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config defines runtime options loaded from environment variables.
type Config struct {
	DBPath       string
	HTTPListen   string
	SOCKSListen  string
	WebListen    string
	AdminUser    string
	AdminPass    string
	EgressMode   string
	EgressAddr   string
	EgressUser   string
	EgressPass   string
	EgressPool   string
	DialTimeout  time.Duration
	HealthTicker time.Duration
	LogDomains   bool
	SessionGrace time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		DBPath:       getEnv("PROXY_DB_PATH", "./data/proxy-center.db"),
		HTTPListen:   getEnv("PROXY_HTTP_LISTEN", ":8080"),
		SOCKSListen:  getEnv("PROXY_SOCKS_LISTEN", ":1080"),
		WebListen:    getEnv("PROXY_WEB_LISTEN", ":8090"),
		AdminUser:    getEnv("PROXY_ADMIN_USER", "admin"),
		AdminPass:    getEnv("PROXY_ADMIN_PASS", "change-me-now"),
		EgressMode:   strings.ToLower(getEnv("PROXY_EGRESS_MODE", "direct")),
		EgressAddr:   getEnv("PROXY_EGRESS_ADDR", ""),
		EgressUser:   getEnv("PROXY_EGRESS_USER", ""),
		EgressPass:   getEnv("PROXY_EGRESS_PASS", ""),
		EgressPool:   getEnv("PROXY_EGRESS_POOL", ""),
		DialTimeout:  getDurationEnv("PROXY_DIAL_TIMEOUT", 15*time.Second),
		HealthTicker: getDurationEnv("PROXY_HEALTH_TICK", 10*time.Second),
		LogDomains:   getBoolEnv("PROXY_LOG_DOMAINS", true),
		SessionGrace: getDurationEnv("PROXY_SESSION_GRACE", 20*time.Second),
	}

	switch cfg.EgressMode {
	case "direct", "http-upstream", "socks5-upstream", "pool":
	default:
		return Config{}, fmt.Errorf("invalid PROXY_EGRESS_MODE: %s", cfg.EgressMode)
	}

	if (cfg.EgressMode == "http-upstream" || cfg.EgressMode == "socks5-upstream") && cfg.EgressAddr == "" {
		return Config{}, fmt.Errorf("PROXY_EGRESS_ADDR is required for mode %s", cfg.EgressMode)
	}
	if cfg.EgressMode == "pool" && strings.TrimSpace(cfg.EgressPool) == "" {
		return Config{}, fmt.Errorf("PROXY_EGRESS_POOL is required for mode pool")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func getBoolEnv(key string, fallback bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return parsed
}

func getDurationEnv(key string, fallback time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}
