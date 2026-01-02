package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port                    int
	DatabaseDSN             string
	JellyfinBaseURL         string
	JellyfinAPIKey          string
	BackendAPIKey           string
	PublicBaseURL           string
	SessionHeartbeatTimeout time.Duration
	LogLevel                string
	RateLimitRequests       int
	RateLimitWindow         time.Duration
}

func Load() *Config {
	return &Config{
		Port:                    getEnvInt("JFSHARE_PORT", 8080),
		DatabaseDSN:             getEnv("JFSHARE_DB_DSN", "postgres://jfshare:jfshare@localhost:5432/jfshare?sslmode=disable"),
		JellyfinBaseURL:         getEnv("JFSHARE_JELLYFIN_BASE_URL", "http://localhost:8096"),
		JellyfinAPIKey:          getEnv("JFSHARE_JELLYFIN_API_KEY", ""),
		BackendAPIKey:           getEnv("JFSHARE_BACKEND_API_KEY", ""),
		PublicBaseURL:           getEnv("JFSHARE_PUBLIC_BASE_URL", "http://localhost:8080"),
		SessionHeartbeatTimeout: time.Duration(getEnvInt("JFSHARE_SESSION_HEARTBEAT_TIMEOUT_SECONDS", 120)) * time.Second,
		LogLevel:                getEnv("JFSHARE_LOG_LEVEL", "info"),
		RateLimitRequests:       getEnvInt("JFSHARE_RATE_LIMIT_REQUESTS", 100),
		RateLimitWindow:         time.Duration(getEnvInt("JFSHARE_RATE_LIMIT_WINDOW_SECONDS", 60)) * time.Second,
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}
