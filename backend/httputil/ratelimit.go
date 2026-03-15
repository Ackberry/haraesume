package httputil

import (
	"net/http"
	"sync"
	"time"
)

type userEntry struct {
	lastRequest time.Time
}

// UserIDExtractor returns a user identifier from an HTTP request.
type UserIDExtractor func(r *http.Request) string

type RateLimiter struct {
	mu        sync.Mutex
	entries   map[string]*userEntry
	window    time.Duration
	extractID UserIDExtractor
}

func NewRateLimiter(window time.Duration, extractID UserIDExtractor) *RateLimiter {
	rl := &RateLimiter{
		entries:   make(map[string]*userEntry),
		window:    window,
		extractID: extractID,
	}
	go rl.cleanup()
	return rl
}

func (rl *RateLimiter) Allow(userID string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	entry, exists := rl.entries[userID]
	if !exists {
		rl.entries[userID] = &userEntry{lastRequest: now}
		return true
	}

	if now.Sub(entry.lastRequest) < rl.window {
		return false
	}

	entry.lastRequest = now
	return true
}

func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		cutoff := time.Now().Add(-2 * rl.window)
		for uid, entry := range rl.entries {
			if entry.lastRequest.Before(cutoff) {
				delete(rl.entries, uid)
			}
		}
		rl.mu.Unlock()
	}
}

func WithRateLimit(rl *RateLimiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := rl.extractID(r)
		if !rl.Allow(userID) {
			remaining := rl.retryAfter(userID)
			w.Header().Set("Retry-After", formatSeconds(int(remaining.Seconds())+1))
			WriteError(w, http.StatusTooManyRequests,
				"Rate limit exceeded. Please wait before generating again.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (rl *RateLimiter) retryAfter(userID string) time.Duration {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	entry, exists := rl.entries[userID]
	if !exists {
		return 0
	}
	elapsed := time.Since(entry.lastRequest)
	if elapsed >= rl.window {
		return 0
	}
	return rl.window - elapsed
}

func formatSeconds(n int) string {
	if n <= 0 {
		return "0"
	}
	buf := [20]byte{}
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[pos:])
}
