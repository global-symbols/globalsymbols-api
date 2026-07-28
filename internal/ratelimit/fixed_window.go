package ratelimit

import (
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"gs-api/internal/auth"
	"gs-api/internal/httpx"
)

type FixedWindowLimiter struct {
	mu           sync.Mutex
	defaultLimit int
	window       time.Duration
	idleTTL      time.Duration
	entries      map[int64]entry
	lastCleanup  time.Time
	now          func() time.Time
}

type entry struct {
	windowStart time.Time
	count       int
	lastSeen    time.Time
}

type Decision struct {
	Allowed    bool
	Limit      int
	Remaining  int
	RetryAfter time.Duration
	ResetAt    time.Time
}

// NewFixedWindowLimiter creates a limiter with a process-wide default RPM.
// Per-request limits are passed to Check (from api_keys.rate_limit_per_minute or default).
func NewFixedWindowLimiter(defaultLimit int, window time.Duration) *FixedWindowLimiter {
	return &FixedWindowLimiter{
		defaultLimit: defaultLimit,
		window:       window,
		idleTTL:      window * 3,
		entries:      make(map[int64]entry),
		now:          time.Now,
	}
}

// Enabled reports whether process-wide rate limiting is active.
// When defaultLimit <= 0, the middleware is a no-op for all keys (global kill switch).
func (l *FixedWindowLimiter) Enabled() bool {
	return l != nil && l.defaultLimit > 0 && l.window > 0
}

// DefaultLimit returns the process-wide default RPM (RATE_LIMIT_PER_MINUTE).
func (l *FixedWindowLimiter) DefaultLimit() int {
	if l == nil {
		return 0
	}
	return l.defaultLimit
}

// Check applies a fixed window limit for key using the given per-minute limit.
// limit <= 0 means unlimited for this key (always allowed).
func (l *FixedWindowLimiter) Check(key int64, limit int) Decision {
	if !l.Enabled() || key == 0 {
		return Decision{Allowed: true}
	}
	if limit <= 0 {
		return Decision{Allowed: true, Limit: 0, Remaining: 0}
	}

	now := l.now().UTC()

	l.mu.Lock()
	defer l.mu.Unlock()

	l.cleanup(now)

	current := l.entries[key]
	windowEnd := current.windowStart.Add(l.window)
	if current.windowStart.IsZero() || !now.Before(windowEnd) {
		current = entry{
			windowStart: now,
			lastSeen:    now,
		}
		windowEnd = current.windowStart.Add(l.window)
	}

	current.lastSeen = now

	if current.count >= limit {
		l.entries[key] = current
		return Decision{
			Allowed:    false,
			Limit:      limit,
			Remaining:  0,
			RetryAfter: windowEnd.Sub(now),
			ResetAt:    windowEnd,
		}
	}

	current.count++
	l.entries[key] = current

	return Decision{
		Allowed:   true,
		Limit:     limit,
		Remaining: limit - current.count,
		ResetAt:   windowEnd,
	}
}

func (l *FixedWindowLimiter) cleanup(now time.Time) {
	if len(l.entries) == 0 {
		l.lastCleanup = now
		return
	}
	if !l.lastCleanup.IsZero() && now.Sub(l.lastCleanup) < l.window {
		return
	}

	for key, current := range l.entries {
		if now.Sub(current.lastSeen) >= l.idleTTL {
			delete(l.entries, key)
		}
	}

	l.lastCleanup = now
}

func APIKeyMiddleware(limiter *FixedWindowLimiter) func(http.Handler) http.Handler {
	if !limiter.Enabled() {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			metadata, ok := auth.MetadataFromContext(r.Context())
			if !ok || metadata.APIKeyID == 0 {
				next.ServeHTTP(w, r)
				return
			}

			limit := auth.EffectiveRateLimit(limiter.DefaultLimit(), metadata.RateLimitPerMinute)
			decision := limiter.Check(metadata.APIKeyID, limit)
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(decision.Limit))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(decision.Remaining))
			if !decision.ResetAt.IsZero() {
				w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(decision.ResetAt.Unix(), 10))
			}

			if decision.Allowed {
				next.ServeHTTP(w, r)
				return
			}

			retryAfterSeconds := int(decision.RetryAfter / time.Second)
			if decision.RetryAfter%time.Second != 0 {
				retryAfterSeconds++
			}
			if retryAfterSeconds < 1 {
				retryAfterSeconds = 1
			}

			w.Header().Set("Retry-After", strconv.Itoa(retryAfterSeconds))
			httpx.Error(w, http.StatusTooManyRequests, fmt.Sprintf("Rate limit exceeded. Try again in %d seconds.", retryAfterSeconds))
		})
	}
}
