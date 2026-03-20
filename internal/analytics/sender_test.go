package analytics

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSenderFlushesPendingBatchOnShutdown(t *testing.T) {
	var (
		gotAuthHeader string
		gotPath       string
		gotBody       []Record
		mu            sync.Mutex
	)

	directus := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		mu.Lock()
		gotAuthHeader = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		mu.Unlock()

		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer directus.Close()

	sender := NewSender(Config{
		DirectusURL:          directus.URL,
		DirectusServiceToken: "secret-token",
		BatchSize:            10,
		FlushInterval:        time.Hour,
		MaxQueueSize:         10,
	})

	ok := sender.Enqueue(Record{
		Method:     http.MethodGet,
		Path:       "/api/v1/languages/active",
		StatusCode: http.StatusOK,
	})
	if !ok {
		t.Fatal("expected enqueue to succeed")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sender.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if gotAuthHeader != "Bearer secret-token" {
		t.Fatalf("expected bearer token auth header, got %q", gotAuthHeader)
	}
	if gotPath != directusItemsEndpoint {
		t.Fatalf("expected Directus bulk path %q, got %q", directusItemsEndpoint, gotPath)
	}
		if len(gotBody) != 1 {
			t.Fatalf("expected 1 record in flushed batch, got %d", len(gotBody))
	}
}

func TestSenderAlertsTeamsWhenBatchIsLost(t *testing.T) {
	var (
		directusCalls int
		alertBodies   []string
		mu            sync.Mutex
	)

	directus := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		directusCalls++
		mu.Unlock()

		http.Error(w, "directus unavailable", http.StatusServiceUnavailable)
	}))
	defer directus.Close()

	teams := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		body, _ := io.ReadAll(r.Body)

		mu.Lock()
		alertBodies = append(alertBodies, string(body))
		mu.Unlock()

		w.WriteHeader(http.StatusOK)
	}))
	defer teams.Close()

	sender := NewSender(Config{
		DirectusURL:          directus.URL,
		DirectusServiceToken: "secret-token",
		TeamsWebhookURL:      teams.URL,
		BatchSize:            1,
		FlushInterval:        time.Hour,
		MaxQueueSize:         10,
	})

	ok := sender.Enqueue(Record{
		Method:     http.MethodGet,
		Path:       "/api/v1/languages/active",
		StatusCode: http.StatusOK,
	})
	if !ok {
		t.Fatal("expected enqueue to succeed")
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		mu.Lock()
		gotAlert := len(alertBodies) > 0
		mu.Unlock()
		if gotAlert {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for Teams alert")
		}
		time.Sleep(25 * time.Millisecond)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sender.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if directusCalls != defaultMaxAttempts {
		t.Fatalf("expected %d Directus attempts, got %d", defaultMaxAttempts, directusCalls)
	}
	if len(alertBodies) == 0 {
		t.Fatal("expected at least one Teams alert")
	}
	if !strings.Contains(alertBodies[0], "analytics batch loss detected") {
		t.Fatalf("expected loss alert body, got %q", alertBodies[0])
	}
}
