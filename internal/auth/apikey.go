package auth

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"gs-api/internal/httpx"
)

type contextKey string

const (
	apiKeyContextKey       contextKey = "apiKeyMetadata"
	metadataSinkContextKey contextKey = "apiKeyMetadataSink"
)

type Metadata struct {
	APIKeyID int64
	Email    string
	UserID   string
}

// APIKeyMiddleware validates the API key against the Rails api_keys table.
func APIKeyMiddleware(db *sql.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rawKey := extractAPIKey(r)
			if rawKey == "" {
				httpx.Error(w, http.StatusUnauthorized, `A valid, active API key is required. Provide it in the Authorization header as "ApiKey <key>" or in the X-Api-Key header.`)
				return
			}

			keyDigest := digest(rawKey)
			var metadata Metadata
			err := db.QueryRow(
				`SELECT id, email FROM api_keys WHERE key_digest = ? AND revoked_at IS NULL AND activated_at IS NOT NULL`,
				keyDigest,
			).Scan(&metadata.APIKeyID, &metadata.Email)
			if err != nil {
				if err == sql.ErrNoRows {
					httpx.Error(w, http.StatusUnauthorized, `A valid, active API key is required. Provide it in the Authorization header as "ApiKey <key>" or in the X-Api-Key header.`)
					return
				}
				httpx.Error(w, http.StatusInternalServerError, "Internal server error")
				return
			}

			// Best-effort update of last_used_at (ignore errors).
			go func() {
				_, _ = db.Exec(`UPDATE api_keys SET last_used_at = ? WHERE id = ?`, time.Now().UTC(), metadata.APIKeyID)
			}()

			ctx := WithMetadata(r.Context(), metadata)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func WithMetadata(ctx context.Context, metadata Metadata) context.Context {
	if sink, ok := ctx.Value(metadataSinkContextKey).(*Metadata); ok && sink != nil {
		*sink = metadata
	}
	return context.WithValue(ctx, apiKeyContextKey, metadata)
}

func WithMetadataSink(r *http.Request) *http.Request {
	if _, ok := r.Context().Value(metadataSinkContextKey).(*Metadata); ok {
		return r
	}

	sink := &Metadata{}
	ctx := context.WithValue(r.Context(), metadataSinkContextKey, sink)
	return r.WithContext(ctx)
}

func MetadataFromContext(ctx context.Context) (Metadata, bool) {
	if metadata, ok := ctx.Value(apiKeyContextKey).(Metadata); ok && metadata.hasData() {
		return metadata, true
	}

	if sink, ok := ctx.Value(metadataSinkContextKey).(*Metadata); ok && sink != nil && sink.hasData() {
		return *sink, true
	}

	return Metadata{}, false
}

func MetadataContextKey() any {
	return apiKeyContextKey
}

func extractAPIKey(r *http.Request) string {
	// X-Api-Key takes precedence if present
	if v := strings.TrimSpace(r.Header.Get("X-Api-Key")); v != "" {
		return v
	}

	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if auth == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(auth), "apikey ") {
		return strings.TrimSpace(auth[len("ApiKey "):])
	}
	return auth
}

// digest mimics Rails' use of a SHA-256 digest for the API key.
func digest(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func (m Metadata) hasData() bool {
	return m.APIKeyID != 0 || m.Email != "" || m.UserID != ""
}
