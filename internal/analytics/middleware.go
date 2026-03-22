package analytics

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"gs-api/internal/auth"
)

const redactedValue = "[REDACTED]"

type Enqueuer interface {
	Enqueue(Record) bool
}

type Observer interface {
	Observe(Record)
}

func Middleware(enqueuer Enqueuer, observers ...Observer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if enqueuer == nil && len(observers) == 0 {
			return next
		}

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startedAt := time.Now().UTC()
			r = auth.WithMetadataSink(r)
			method := r.Method
			path := r.URL.Path
			if path == "" {
				path = "/"
			}
			queryParams := sanitizeQueryParams(r.URL.Query())

			recorder := newResponseRecorder(w)
			next.ServeHTTP(recorder, r)
			statusCode := recorder.StatusCode()

			record := Record{
				CreatedAt:         startedAt,
				IPAddress:         clientIP(r.RemoteAddr),
				UserAgent:         r.UserAgent(),
				Method:            method,
				Path:              path,
				QueryParams:       queryParams,
				StatusCode:        statusCode,
				ResponseTimeMS:    time.Since(startedAt).Milliseconds(),
				IsRateLimitBreach: statusCode == http.StatusTooManyRequests,
				ErrorMessage:      extractErrorMessage(statusCode, recorder.Body()),
			}

			if metadata, ok := auth.MetadataFromContext(r.Context()); ok {
				if metadata.APIKeyID != 0 {
					record.APIKeyID = strconv.FormatInt(metadata.APIKeyID, 10)
				}
				record.UserEmail = metadata.Email
				record.UserID = metadata.UserID
			}

			if enqueuer != nil {
				enqueuer.Enqueue(record)
			}
			for _, observer := range observers {
				if observer != nil {
					observer.Observe(record)
				}
			}
		})
	}
}

func sanitizeQueryParams(values url.Values) map[string][]string {
	if len(values) == 0 {
		return nil
	}

	sanitized := make(map[string][]string, len(values))
	for key, rawValues := range values {
		if isSensitiveQueryKey(key) {
			redacted := make([]string, len(rawValues))
			for i := range redacted {
				redacted[i] = redactedValue
			}
			sanitized[key] = redacted
			continue
		}

		copied := make([]string, len(rawValues))
		copy(copied, rawValues)
		sanitized[key] = copied
	}

	return sanitized
}

func isSensitiveQueryKey(key string) bool {
	lower := strings.ToLower(strings.TrimSpace(key))
	switch lower {
	case "access_token", "refresh_token", "token", "api_key", "apikey", "authorization",
		"auth", "secret", "password", "passcode", "client_secret", "id_token", "code":
		return true
	default:
		return strings.Contains(lower, "token") || strings.Contains(lower, "secret") || strings.Contains(lower, "password")
	}
}

func clientIP(remoteAddr string) string {
	if remoteAddr == "" {
		return ""
	}

	host, _, err := net.SplitHostPort(remoteAddr)
	if err == nil {
		return host
	}

	return remoteAddr
}

func extractErrorMessage(statusCode int, body []byte) string {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return ""
	}

	var payload map[string]any
	if err := json.Unmarshal(trimmed, &payload); err == nil {
		for _, key := range []string{"error", "message"} {
			if value, ok := payload[key].(string); ok && strings.TrimSpace(value) != "" {
				return strings.TrimSpace(value)
			}
		}
	}

	if statusCode < http.StatusBadRequest {
		return ""
	}

	if len(trimmed) > maxCapturedBodyBytes {
		trimmed = trimmed[:maxCapturedBodyBytes]
	}

	return strings.TrimSpace(string(trimmed))
}
