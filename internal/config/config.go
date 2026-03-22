package config

import (
	"os"
	"strconv"
	"time"

	"gs-api/internal/alerting"
	"gs-api/internal/analytics"
)

type Config struct {
	DBDSN        string
	Port         string
	ImageBaseURL string
	RailsBaseURL string
	RateLimitPerMinute int
	Alerts       alerting.Config
	Analytics    analytics.Config
}

func FromEnv() Config {
	return Config{
		DBDSN:        getenv("DB_DSN", "gs-repo-dev:gs-repo-dev@tcp(localhost:3307)/gs-repo-dev?parseTime=true&charset=utf8mb4&loc=UTC"),
		Port:         getenv("PORT", "8080"),
		ImageBaseURL: getenv("API_IMAGE_BASE_URL", "http://localhost:3000"),
		RailsBaseURL: getenv("RAILS_API_BASE_URL", "http://localhost:3000"),
		RateLimitPerMinute: getenvInt("RATE_LIMIT_PER_MINUTE", 100),
		Alerts: alerting.Config{
			TeamsWebhookURL:          getenv("TEAMS_WEBHOOK_URL", ""),
			RateLimitBreachThreshold: getenvInt("SUSPICIOUS_RATE_LIMIT_BREACH_THRESHOLD", 20),
			RateLimitBreachWindow:    time.Duration(getenvInt("SUSPICIOUS_RATE_LIMIT_BREACH_WINDOW_MS", 120000)) * time.Millisecond,
			UnauthorizedIPThreshold:  getenvInt("SUSPICIOUS_UNAUTHORIZED_IP_THRESHOLD", 50),
			UnauthorizedIPWindow:     time.Duration(getenvInt("SUSPICIOUS_UNAUTHORIZED_IP_WINDOW_MS", 300000)) * time.Millisecond,
			IPRequestThreshold:       getenvInt("SUSPICIOUS_IP_REQUEST_THRESHOLD", 300),
			IPRequestWindow:          time.Duration(getenvInt("SUSPICIOUS_IP_REQUEST_WINDOW_MS", 60000)) * time.Millisecond,
			AlertCooldown:            time.Duration(getenvInt("SUSPICIOUS_ALERT_COOLDOWN_MS", 1800000)) * time.Millisecond,
		},
		Analytics: analytics.Config{
			DirectusURL:          getenv("DIRECTUS_URL", ""),
			DirectusServiceToken: getenv("DIRECTUS_SERVICE_TOKEN", ""),
			TeamsWebhookURL:      getenv("TEAMS_WEBHOOK_URL", ""),
			BatchSize:            getenvInt("ANALYTICS_BATCH_SIZE", 100),
			FlushInterval:        time.Duration(getenvInt("ANALYTICS_FLUSH_INTERVAL_MS", 500)) * time.Millisecond,
			MaxQueueSize:         getenvInt("ANALYTICS_MAX_QUEUE_SIZE", 10000),
		},
	}
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getenvInt(key string, def int) int {
	value := os.Getenv(key)
	if value == "" {
		return def
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return def
	}

	return parsed
}
