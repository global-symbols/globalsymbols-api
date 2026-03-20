package analytics

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"gs-api/internal/auth"
	"gs-api/internal/httpx"
)

type stubEnqueuer struct {
	records []Record
}

func (s *stubEnqueuer) Enqueue(record Record) bool {
	s.records = append(s.records, record)
	return true
}

func TestMiddlewareCapturesAnalyticsRecord(t *testing.T) {
	stub := &stubEnqueuer{}
	handler := Middleware(stub)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = auth.WithMetadata(r.Context(), auth.Metadata{
			APIKeyID: 42,
			Email:    "api@example.com",
		})
		httpx.Error(w, http.StatusTooManyRequests, "too many requests")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/languages/active?query=dog&access_token=secret", nil)
	req.RemoteAddr = "203.0.113.9:12345"
	req.Header.Set("User-Agent", "analytics-test")

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status %d, got %d", http.StatusTooManyRequests, recorder.Code)
	}

	if len(stub.records) != 1 {
		t.Fatalf("expected 1 analytics record, got %d", len(stub.records))
	}

	record := stub.records[0]
	if record.APIKeyID != "42" {
		t.Fatalf("expected api_key_id 42, got %q", record.APIKeyID)
	}
	if record.UserEmail != "api@example.com" {
		t.Fatalf("expected user_email to be populated, got %q", record.UserEmail)
	}
	if record.IPAddress != "203.0.113.9" {
		t.Fatalf("expected IP address to be parsed, got %q", record.IPAddress)
	}
	if record.QueryParams["access_token"][0] != redactedValue {
		t.Fatalf("expected access_token to be redacted, got %q", record.QueryParams["access_token"][0])
	}
	if record.QueryParams["query"][0] != "dog" {
		t.Fatalf("expected query param to be preserved, got %q", record.QueryParams["query"][0])
	}
	if !record.IsRateLimitBreach {
		t.Fatal("expected rate-limit breach to be inferred for status 429")
	}
	if record.ErrorMessage != "too many requests" {
		t.Fatalf("expected error message to be extracted, got %q", record.ErrorMessage)
	}
	if record.ResponseTimeMS < 0 {
		t.Fatalf("expected non-negative response time, got %d", record.ResponseTimeMS)
	}
}

func TestMiddlewareSnapshotsRequestMetadataBeforeHandlerMutation(t *testing.T) {
	stub := &stubEnqueuer{}
	handler := Middleware(stub)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Method = ""
		r.URL.Path = ""
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/pictos", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, recorder.Code)
	}

	if len(stub.records) != 1 {
		t.Fatalf("expected 1 analytics record, got %d", len(stub.records))
	}

	record := stub.records[0]
	if record.Method != http.MethodDelete {
		t.Fatalf("expected original method %q, got %q", http.MethodDelete, record.Method)
	}
	if record.Path != "/api/v1/pictos" {
		t.Fatalf("expected original path %q, got %q", "/api/v1/pictos", record.Path)
	}
	if record.StatusCode != http.StatusNoContent {
		t.Fatalf("expected status code %d, got %d", http.StatusNoContent, record.StatusCode)
	}
}

func TestMiddlewareDoesNotStoreSuccessfulResponseBodiesAsErrors(t *testing.T) {
	stub := &stubEnqueuer{}
	handler := Middleware(stub)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload := []map[string]any{
			{
				"id":   1653,
				"text": "hello",
			},
		}
		httpx.JSON(w, http.StatusOK, payload)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/labels/search?query=hello&limit=5", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	if len(stub.records) != 1 {
		t.Fatalf("expected 1 analytics record, got %d", len(stub.records))
	}

	record := stub.records[0]
	if record.ErrorMessage != "" {
		t.Fatalf("expected empty error_message for successful response, got %q", record.ErrorMessage)
	}

	var body []map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("expected successful JSON response body, got decode error: %v", err)
	}
	if len(body) != 1 || body[0]["text"] != "hello" {
		t.Fatalf("expected successful response body to remain intact, got %#v", body)
	}
}
