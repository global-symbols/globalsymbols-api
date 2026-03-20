package analytics

import (
	"strings"
	"time"
)

const (
	defaultBatchSize      = 100
	defaultFlushInterval  = 500 * time.Millisecond
	defaultMaxQueueSize   = 10000
	defaultHTTPTimeout    = 5 * time.Second
	defaultRetryBackoff   = 250 * time.Millisecond
	defaultMaxAttempts    = 2
	defaultAlertTimeout   = 3 * time.Second
	directusItemsEndpoint = "/items/api_request_logs"
)

type Config struct {
	DirectusURL          string
	DirectusServiceToken string
	TeamsWebhookURL      string
	BatchSize            int
	FlushInterval        time.Duration
	MaxQueueSize         int
}

func (c Config) Enabled() bool {
	return strings.TrimSpace(c.DirectusURL) != "" && strings.TrimSpace(c.DirectusServiceToken) != ""
}

func (c Config) normalized() Config {
	if c.BatchSize <= 0 {
		c.BatchSize = defaultBatchSize
	}
	if c.FlushInterval <= 0 {
		c.FlushInterval = defaultFlushInterval
	}
	if c.MaxQueueSize <= 0 {
		c.MaxQueueSize = defaultMaxQueueSize
	}

	c.DirectusURL = strings.TrimSpace(strings.TrimRight(c.DirectusURL, "/"))
	c.DirectusServiceToken = strings.TrimSpace(c.DirectusServiceToken)
	c.TeamsWebhookURL = strings.TrimSpace(c.TeamsWebhookURL)

	return c
}
