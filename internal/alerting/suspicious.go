package alerting

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"gs-api/internal/analytics"
)

const defaultAlertTimeout = 3 * time.Second

type Config struct {
	TeamsWebhookURL          string
	RateLimitBreachThreshold int
	RateLimitBreachWindow    time.Duration
	UnauthorizedIPThreshold  int
	UnauthorizedIPWindow     time.Duration
	IPRequestThreshold       int
	IPRequestWindow          time.Duration
	AlertCooldown            time.Duration
}

type Detector struct {
	cfg        Config
	client     *http.Client
	now        func() time.Time
	mu         sync.Mutex
	keyStates  map[string]*keyState
	ipStates   map[string]*ipState
	alertsWG   sync.WaitGroup
}

type keyState struct {
	rateLimitBreaches []time.Time
	lastAlertAt       time.Time
	lastPath          string
	email             string
}

type ipState struct {
	unauthorized []time.Time
	requests     []time.Time
	last401Alert time.Time
	lastRPSAlert time.Time
	lastPath     string
}

func NewDetector(cfg Config) *Detector {
	return &Detector{
		cfg:      cfg.normalized(),
		client:   &http.Client{Timeout: defaultAlertTimeout},
		now:      time.Now,
		keyStates: make(map[string]*keyState),
		ipStates:  make(map[string]*ipState),
	}
}

func (c Config) normalized() Config {
	c.TeamsWebhookURL = strings.TrimSpace(c.TeamsWebhookURL)
	if c.RateLimitBreachThreshold <= 0 {
		c.RateLimitBreachThreshold = 20
	}
	if c.RateLimitBreachWindow <= 0 {
		c.RateLimitBreachWindow = 2 * time.Minute
	}
	if c.UnauthorizedIPThreshold <= 0 {
		c.UnauthorizedIPThreshold = 50
	}
	if c.UnauthorizedIPWindow <= 0 {
		c.UnauthorizedIPWindow = 5 * time.Minute
	}
	if c.IPRequestThreshold <= 0 {
		c.IPRequestThreshold = 300
	}
	if c.IPRequestWindow <= 0 {
		c.IPRequestWindow = time.Minute
	}
	if c.AlertCooldown <= 0 {
		c.AlertCooldown = 30 * time.Minute
	}
	return c
}

func (d *Detector) Observe(record analytics.Record) {
	if d == nil {
		return
	}

	now := d.now().UTC()

	d.mu.Lock()
	if record.APIKeyID != "" && record.IsRateLimitBreach {
		if message, ok := d.observeRateLimitLocked(now, record); ok {
			d.emitAlertLocked(message)
		}
	}
	if record.IPAddress != "" {
		if message, ok := d.observeIPLocked(now, record); ok {
			d.emitAlertLocked(message)
		}
	}
	d.cleanupLocked(now)
	d.mu.Unlock()
}

func (d *Detector) Shutdown(ctx context.Context) error {
	if d == nil {
		return nil
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		d.alertsWG.Wait()
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (d *Detector) observeRateLimitLocked(now time.Time, record analytics.Record) (string, bool) {
	state := d.keyStates[record.APIKeyID]
	if state == nil {
		state = &keyState{}
		d.keyStates[record.APIKeyID] = state
	}

	state.rateLimitBreaches = append(trimOlderThan(state.rateLimitBreaches, now, d.cfg.RateLimitBreachWindow), now)
	state.lastPath = record.Path
	state.email = record.UserEmail

	if len(state.rateLimitBreaches) < d.cfg.RateLimitBreachThreshold {
		return "", false
	}
	if now.Sub(state.lastAlertAt) < d.cfg.AlertCooldown {
		return "", false
	}

	state.lastAlertAt = now

	return fmt.Sprintf(
		"GS API suspicious traffic alert\n\nSignal: rate-limit breach spike per API key\nAPI key: %s\nEmail: %s\nCount: %d breaches in %s\nLatest path: %s\nLatest status: %d\nTime: %s",
		record.APIKeyID,
		emptyFallback(record.UserEmail, "n/a"),
		len(state.rateLimitBreaches),
		d.cfg.RateLimitBreachWindow,
		emptyFallback(record.Path, "/"),
		record.StatusCode,
		now.Format(time.RFC3339),
	), true
}

func (d *Detector) observeIPLocked(now time.Time, record analytics.Record) (string, bool) {
	state := d.ipStates[record.IPAddress]
	if state == nil {
		state = &ipState{}
		d.ipStates[record.IPAddress] = state
	}

	state.requests = append(trimOlderThan(state.requests, now, d.cfg.IPRequestWindow), now)
	state.lastPath = record.Path

	if record.StatusCode == http.StatusUnauthorized {
		state.unauthorized = append(trimOlderThan(state.unauthorized, now, d.cfg.UnauthorizedIPWindow), now)
		if len(state.unauthorized) >= d.cfg.UnauthorizedIPThreshold && now.Sub(state.last401Alert) >= d.cfg.AlertCooldown {
			state.last401Alert = now
			return fmt.Sprintf(
				"GS API suspicious traffic alert\n\nSignal: unauthorized spike per IP\nIP address: %s\nCount: %d responses with status 401 in %s\nLatest path: %s\nTime: %s",
				record.IPAddress,
				len(state.unauthorized),
				d.cfg.UnauthorizedIPWindow,
				emptyFallback(record.Path, "/"),
				now.Format(time.RFC3339),
			), true
		}
	}

	if len(state.requests) >= d.cfg.IPRequestThreshold && now.Sub(state.lastRPSAlert) >= d.cfg.AlertCooldown {
		state.lastRPSAlert = now
		return fmt.Sprintf(
			"GS API suspicious traffic alert\n\nSignal: high request rate per IP\nIP address: %s\nCount: %d requests in %s\nLatest path: %s\nLatest status: %d\nTime: %s",
			record.IPAddress,
			len(state.requests),
			d.cfg.IPRequestWindow,
			emptyFallback(record.Path, "/"),
			record.StatusCode,
			now.Format(time.RFC3339),
		), true
	}

	return "", false
}

func (d *Detector) cleanupLocked(now time.Time) {
	for key, state := range d.keyStates {
		state.rateLimitBreaches = trimOlderThan(state.rateLimitBreaches, now, d.cfg.RateLimitBreachWindow)
		if len(state.rateLimitBreaches) == 0 && now.Sub(state.lastAlertAt) >= d.cfg.AlertCooldown {
			delete(d.keyStates, key)
		}
	}

	maxIPWindow := d.cfg.IPRequestWindow
	if d.cfg.UnauthorizedIPWindow > maxIPWindow {
		maxIPWindow = d.cfg.UnauthorizedIPWindow
	}
	for key, state := range d.ipStates {
		state.requests = trimOlderThan(state.requests, now, d.cfg.IPRequestWindow)
		state.unauthorized = trimOlderThan(state.unauthorized, now, d.cfg.UnauthorizedIPWindow)
		lastAlertAt := state.lastRPSAlert
		if state.last401Alert.After(lastAlertAt) {
			lastAlertAt = state.last401Alert
		}
		if len(state.requests) == 0 && len(state.unauthorized) == 0 && now.Sub(lastAlertAt) >= maxDuration(d.cfg.AlertCooldown, maxIPWindow) {
			delete(d.ipStates, key)
		}
	}
}

func (d *Detector) emitAlertLocked(message string) {
	d.alertsWG.Add(1)
	go func() {
		defer d.alertsWG.Done()
		if err := d.postTeamsAlert(message); err != nil {
			log.Printf("failed to send suspicious traffic Teams alert: %v", err)
		}
	}()
}

func (d *Detector) postTeamsAlert(message string) error {
	if strings.TrimSpace(d.cfg.TeamsWebhookURL) == "" {
		log.Printf("suspicious traffic alert: %s", message)
		return nil
	}

	payload, err := json.Marshal(map[string]string{
		"@type":    "MessageCard",
		"@context": "http://schema.org/extensions",
		"summary":  "GS API suspicious traffic alert",
		"title":    "GS API suspicious traffic alert",
		"text":     message,
	})
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultAlertTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.cfg.TeamsWebhookURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	return fmt.Errorf("teams webhook returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
}

func trimOlderThan(times []time.Time, now time.Time, window time.Duration) []time.Time {
	if len(times) == 0 {
		return times
	}
	cutoff := now.Add(-window)
	index := 0
	for index < len(times) && times[index].Before(cutoff) {
		index++
	}
	if index == 0 {
		return times
	}
	trimmed := append([]time.Time(nil), times[index:]...)
	return trimmed
}

func emptyFallback(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func maxDuration(a, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
}
