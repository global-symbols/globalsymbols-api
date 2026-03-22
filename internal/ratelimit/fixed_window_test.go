package ratelimit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gs-api/internal/auth"
)

func TestFixedWindowLimiterBlocksAfterLimit(t *testing.T) {
	now := time.Date(2026, time.March, 22, 12, 0, 0, 0, time.UTC)
	limiter := NewFixedWindowLimiter(2, time.Minute)
	limiter.now = func() time.Time { return now }

	first := limiter.Check(42)
	if !first.Allowed {
		t.Fatal("expected first request to be allowed")
	}
	if first.Remaining != 1 {
		t.Fatalf("expected remaining quota 1, got %d", first.Remaining)
	}

	second := limiter.Check(42)
	if !second.Allowed {
		t.Fatal("expected second request to be allowed")
	}
	if second.Remaining != 0 {
		t.Fatalf("expected remaining quota 0, got %d", second.Remaining)
	}

	third := limiter.Check(42)
	if third.Allowed {
		t.Fatal("expected third request to be blocked")
	}
	if third.RetryAfter != time.Minute {
		t.Fatalf("expected retry after 1 minute, got %s", third.RetryAfter)
	}
}

func TestFixedWindowLimiterResetsAfterWindow(t *testing.T) {
	now := time.Date(2026, time.March, 22, 12, 0, 0, 0, time.UTC)
	limiter := NewFixedWindowLimiter(1, time.Minute)
	limiter.now = func() time.Time { return now }

	if !limiter.Check(7).Allowed {
		t.Fatal("expected first request to be allowed")
	}
	if limiter.Check(7).Allowed {
		t.Fatal("expected second request in same window to be blocked")
	}

	now = now.Add(time.Minute)

	afterReset := limiter.Check(7)
	if !afterReset.Allowed {
		t.Fatal("expected request after window reset to be allowed")
	}
	if afterReset.Remaining != 0 {
		t.Fatalf("expected remaining quota 0 after reset, got %d", afterReset.Remaining)
	}
}

func TestAPIKeyMiddlewareReturns429WhenLimitExceeded(t *testing.T) {
	now := time.Date(2026, time.March, 22, 12, 0, 0, 0, time.UTC)
	limiter := NewFixedWindowLimiter(1, time.Minute)
	limiter.now = func() time.Time { return now }

	handler := APIKeyMiddleware(limiter)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	requestWithKey := func() *http.Request {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/languages/active", nil)
		ctx := auth.WithMetadata(context.Background(), auth.Metadata{APIKeyID: 42, Email: "api@example.com"})
		return req.WithContext(ctx)
	}

	first := httptest.NewRecorder()
	handler.ServeHTTP(first, requestWithKey())
	if first.Code != http.StatusNoContent {
		t.Fatalf("expected first request to pass through, got %d", first.Code)
	}
	if first.Header().Get("X-RateLimit-Remaining") != "0" {
		t.Fatalf("expected remaining quota header to be 0, got %q", first.Header().Get("X-RateLimit-Remaining"))
	}

	second := httptest.NewRecorder()
	handler.ServeHTTP(second, requestWithKey())
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("expected second request to be rate limited, got %d", second.Code)
	}
	if second.Header().Get("Retry-After") != "60" {
		t.Fatalf("expected Retry-After header to be 60, got %q", second.Header().Get("Retry-After"))
	}
	if body := second.Body.String(); body == "" {
		t.Fatal("expected rate-limited response body to be populated")
	}
}

func TestAPIKeyMiddlewareSkipsRequestsWithoutMetadata(t *testing.T) {
	limiter := NewFixedWindowLimiter(1, time.Minute)

	handler := APIKeyMiddleware(limiter)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/docs", nil)
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected request without metadata to pass through, got %d", recorder.Code)
	}
	if recorder.Header().Get("X-RateLimit-Limit") != "" {
		t.Fatalf("expected rate-limit headers to be omitted, got %q", recorder.Header().Get("X-RateLimit-Limit"))
	}
}
