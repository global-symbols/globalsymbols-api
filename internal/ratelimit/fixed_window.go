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
	mu          sync.Mutex
	limit       int
	window      time.Duration
	idleTTL     time.Duration
	entries     map[int64]entry
	lastCleanup time.Time
	now         func() time.Time
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

func NewFixedWindowLimiter(limit int, window time.Duration) *FixedWindowLimiter {
	return &FixedWindowLimiter{
		limit:   limit,
		window:  window,
		idleTTL: window * 3,
		entries: make(map[int64]entry),
		now:     time.Now,
	}
}

func (l *FixedWindowLimiter) Enabled() bool {
	return l != nil && l.limit > 0 && l.window > 0
}

func (l *FixedWindowLimiter) Check(key int64) Decision {
	if !l.Enabled() || key == 0 {
		return Decision{Allowed: true}
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

	if current.count >= l.limit {
		l.entries[key] = current
		return Decision{
			Allowed:    false,
			Limit:      l.limit,
			Remaining:  0,
			RetryAfter: windowEnd.Sub(now),
			ResetAt:    windowEnd,
		}
	}

	current.count++
	l.entries[key] = current

	return Decision{
		Allowed:   true,
		Limit:     l.limit,
		Remaining: l.limit - current.count,
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

			decision := limiter.Check(metadata.APIKeyID)
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
