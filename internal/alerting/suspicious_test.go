package alerting

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"gs-api/internal/analytics"
)

func TestDetectorAlertsOnRateLimitBreachSpikePerKey(t *testing.T) {
	var (
		mu         sync.Mutex
		alertBodies []string
	)

	teams := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		alertBodies = append(alertBodies, string(body))
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer teams.Close()

	now := time.Date(2026, time.March, 22, 12, 0, 0, 0, time.UTC)
	detector := NewDetector(Config{
		TeamsWebhookURL:          teams.URL,
		RateLimitBreachThreshold: 2,
		RateLimitBreachWindow:    2 * time.Minute,
		UnauthorizedIPThreshold:  50,
		UnauthorizedIPWindow:     5 * time.Minute,
		IPRequestThreshold:       300,
		IPRequestWindow:          time.Minute,
		AlertCooldown:            30 * time.Minute,
	})
	detector.now = func() time.Time { return now }

	record := analytics.Record{
		APIKeyID:          "42",
		UserEmail:         "api@example.com",
		Path:              "/api/v2/labels/search",
		StatusCode:        http.StatusTooManyRequests,
		IsRateLimitBreach: true,
	}

	detector.Observe(record)
	detector.Observe(record)

	waitForAlerts(t, &mu, &alertBodies, 1)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := detector.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(alertBodies) != 1 {
		t.Fatalf("expected 1 Teams alert, got %d", len(alertBodies))
	}
	if !strings.Contains(alertBodies[0], "rate-limit breach spike per API key") {
		t.Fatalf("expected rate-limit breach alert body, got %q", alertBodies[0])
	}
	if !strings.Contains(alertBodies[0], "42") {
		t.Fatalf("expected API key ID in alert body, got %q", alertBodies[0])
	}
}

func TestDetectorAlertsOnUnauthorizedSpikePerIP(t *testing.T) {
	var (
		mu         sync.Mutex
		alertBodies []string
	)

	teams := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		alertBodies = append(alertBodies, string(body))
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer teams.Close()

	now := time.Date(2026, time.March, 22, 12, 0, 0, 0, time.UTC)
	detector := NewDetector(Config{
		TeamsWebhookURL:          teams.URL,
		RateLimitBreachThreshold: 20,
		RateLimitBreachWindow:    2 * time.Minute,
		UnauthorizedIPThreshold:  2,
		UnauthorizedIPWindow:     5 * time.Minute,
		IPRequestThreshold:       300,
		IPRequestWindow:          time.Minute,
		AlertCooldown:            30 * time.Minute,
	})
	detector.now = func() time.Time { return now }

	record := analytics.Record{
		IPAddress:  "203.0.113.9",
		Path:       "/api/v2/languages/active",
		StatusCode: http.StatusUnauthorized,
	}

	detector.Observe(record)
	detector.Observe(record)

	waitForAlerts(t, &mu, &alertBodies, 1)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := detector.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(alertBodies) != 1 {
		t.Fatalf("expected 1 Teams alert, got %d", len(alertBodies))
	}
	if !strings.Contains(alertBodies[0], "unauthorized spike per IP") {
		t.Fatalf("expected unauthorized spike alert body, got %q", alertBodies[0])
	}
	if !strings.Contains(alertBodies[0], "203.0.113.9") {
		t.Fatalf("expected IP address in alert body, got %q", alertBodies[0])
	}
}

func TestDetectorSuppressesRepeatedAlertsDuringCooldown(t *testing.T) {
	var (
		mu         sync.Mutex
		alertBodies []string
	)

	teams := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		alertBodies = append(alertBodies, string(body))
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer teams.Close()

	now := time.Date(2026, time.March, 22, 12, 0, 0, 0, time.UTC)
	detector := NewDetector(Config{
		TeamsWebhookURL:          teams.URL,
		RateLimitBreachThreshold: 2,
		RateLimitBreachWindow:    2 * time.Minute,
		UnauthorizedIPThreshold:  50,
		UnauthorizedIPWindow:     5 * time.Minute,
		IPRequestThreshold:       300,
		IPRequestWindow:          time.Minute,
		AlertCooldown:            30 * time.Minute,
	})
	detector.now = func() time.Time { return now }

	record := analytics.Record{
		APIKeyID:          "42",
		Path:              "/api/v2/labels/search",
		StatusCode:        http.StatusTooManyRequests,
		IsRateLimitBreach: true,
	}

	detector.Observe(record)
	detector.Observe(record)
	waitForAlerts(t, &mu, &alertBodies, 1)

	now = now.Add(5 * time.Minute)
	detector.Observe(record)
	detector.Observe(record)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := detector.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(alertBodies) != 1 {
		t.Fatalf("expected cooldown to suppress repeat alert, got %d alerts", len(alertBodies))
	}
}

func waitForAlerts(t *testing.T, mu *sync.Mutex, bodies *[]string, expected int) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for {
		mu.Lock()
		count := len(*bodies)
		mu.Unlock()
		if count >= expected {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %d alerts, got %d", expected, count)
		}
		time.Sleep(25 * time.Millisecond)
	}
}
