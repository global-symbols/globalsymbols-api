package config

import (
	"os"
	"strconv"
	"time"

	"gs-api/internal/analytics"
)

type Config struct {
	DBDSN        string
	Port         string
	ImageBaseURL string
	RailsBaseURL string
	Analytics    analytics.Config
}

func FromEnv() Config {
	return Config{
		DBDSN:        getenv("DB_DSN", "gs-repo-dev:gs-repo-dev@tcp(localhost:3307)/gs-repo-dev?parseTime=true&charset=utf8mb4&loc=UTC"),
		Port:         getenv("PORT", "8080"),
		ImageBaseURL: getenv("API_IMAGE_BASE_URL", "http://localhost:3000"),
		RailsBaseURL: getenv("RAILS_API_BASE_URL", "http://localhost:3000"),
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
