package runtime

import (
	"sync"
	"time"
)

// RateLimiter is a sliding window limiter that keeps a separate window per
// key. It is used to protect the bot and its download queue from bursts.
type RateLimiter struct {
	mu sync.Mutex

	windows    map[string][]time.Time
	limit      int
	window     time.Duration
	maxTracked int
}

// NewRateLimiter builds a limiter allowing limit events per window.
func NewRateLimiter(limit int, window time.Duration, maxTracked int) *RateLimiter {
	if limit <= 0 {
		limit = 1
	}
	if window <= 0 {
		window = time.Second
	}
	if maxTracked <= 0 {
		maxTracked = 50000
	}
	return &RateLimiter{
		windows:    map[string][]time.Time{},
		limit:      limit,
		window:     window,
		maxTracked: maxTracked,
	}
}

// Allow reports whether an event for the key fits into the current window.
func (r *RateLimiter) Allow(key string) bool {
	now := time.Now()
	cutoff := now.Add(-r.window)

	r.mu.Lock()
	defer r.mu.Unlock()

	events := r.windows[key]
	filtered := make([]time.Time, 0, len(events)+1)
	for _, at := range events {
		if at.After(cutoff) {
			filtered = append(filtered, at)
		}
	}
	if len(filtered) >= r.limit {
		r.windows[key] = filtered
		return false
	}
	r.windows[key] = append(filtered, now)

	if len(r.windows) > r.maxTracked {
		for tracked := range r.windows {
			if len(r.windows) <= r.maxTracked {
				break
			}
			delete(r.windows, tracked)
		}
	}
	return true
}

// Remaining returns how many events are still allowed for the key.
func (r *RateLimiter) Remaining(key string) int {
	cutoff := time.Now().Add(-r.window)
	r.mu.Lock()
	defer r.mu.Unlock()

	count := 0
	for _, at := range r.windows[key] {
		if at.After(cutoff) {
			count++
		}
	}
	remaining := r.limit - count
	if remaining < 0 {
		return 0
	}
	return remaining
}

// Reset clears the window of a key.
func (r *RateLimiter) Reset(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.windows, key)
}
