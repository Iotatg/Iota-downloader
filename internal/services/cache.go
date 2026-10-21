package services

import (
	"sync"
	"time"
)

// ResolutionCache keeps the resolved links of the last minutes in memory.
//
// The resolvers of the platforms are the slow part of a download, a repeated
// link can reuse the answer for a short while. The entries expire on their
// own so a fresh link is never answered with a stale media link.
type ResolutionCache struct {
	mu      sync.Mutex
	entries map[string]resolutionEntry
	ttl     time.Duration
	max     int
}

// resolutionEntry is a cached resolution with its expiry.
type resolutionEntry struct {
	resolved *Resolved
	expires  time.Time
}

// NewResolutionCache builds a cache that keeps the answers for the given
// time. A ttl of zero turns the cache off.
func NewResolutionCache(ttl time.Duration, max int) *ResolutionCache {
	if max <= 0 {
		max = 500
	}
	return &ResolutionCache{
		entries: make(map[string]resolutionEntry),
		ttl:     ttl,
		max:     max,
	}
}

// Get returns the cached resolution of a key, nil when there is none.
func (c *ResolutionCache) Get(key string) *Resolved {
	if c == nil || c.ttl <= 0 {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[key]
	if !ok {
		return nil
	}
	if time.Now().After(entry.expires) {
		delete(c.entries, key)
		return nil
	}
	return entry.resolved
}

// Put stores a resolution under a key.
func (c *ResolutionCache) Put(key string, resolved *Resolved) {
	if c == nil || c.ttl <= 0 || resolved == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.entries) >= c.max {
		c.pruneLocked()
	}
	if len(c.entries) >= c.max {
		// The cache is full and nothing expired, the oldest entry is dropped.
		for candidate := range c.entries {
			delete(c.entries, candidate)
			break
		}
	}
	c.entries[key] = resolutionEntry{resolved: resolved, expires: time.Now().Add(c.ttl)}
}

// Len returns the number of cached resolutions.
func (c *ResolutionCache) Len() int {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}

// pruneLocked removes the expired entries of the cache.
func (c *ResolutionCache) pruneLocked() {
	now := time.Now()
	for key, entry := range c.entries {
		if now.After(entry.expires) {
			delete(c.entries, key)
		}
	}
}
