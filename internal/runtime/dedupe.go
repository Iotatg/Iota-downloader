// Package runtime holds the transient runtime state of the bot: request
// deduplication, rate limiting, live statistics and a small key/value store.
package runtime

import (
	"sync"
	"time"
)

// Dedupe collapses duplicate requests for the same resource.
//
// The first caller becomes the leader and performs the work, every other
// caller waits for the leader to finish and then reuses the cached result.
type Dedupe struct {
	mu sync.Mutex

	active       map[string]*dedupeEntry
	completed    map[string]time.Time
	activeTTL    time.Duration
	completedTTL time.Duration
	maxEntries   int
}

type dedupeEntry struct {
	created time.Time
}

// NewDedupe builds a deduplication tracker.
func NewDedupe(activeTTL, completedTTL time.Duration, maxEntries int) *Dedupe {
	if activeTTL <= 0 {
		activeTTL = 15 * time.Minute
	}
	if completedTTL <= 0 {
		completedTTL = 12 * time.Second
	}
	if maxEntries <= 0 {
		maxEntries = 50000
	}
	return &Dedupe{
		active:       map[string]*dedupeEntry{},
		completed:    map[string]time.Time{},
		activeTTL:    activeTTL,
		completedTTL: completedTTL,
		maxEntries:   maxEntries,
	}
}

// Begin registers a request for the key.
//
// It returns true when the caller became the leader and false when another
// caller is already processing the same key. The returned function releases the
// key and must be called when the leader finished.
func (d *Dedupe) Begin(key string) (bool, func()) {
	d.mu.Lock()
	d.pruneLocked()

	if _, ok := d.completed[key]; ok {
		delete(d.completed, key)
		d.mu.Unlock()
		return false, func() {}
	}
	if _, ok := d.active[key]; ok {
		d.mu.Unlock()
		return false, func() {}
	}

	entry := &dedupeEntry{created: time.Now()}
	d.active[key] = entry
	d.mu.Unlock()

	var once sync.Once
	release := func() {
		once.Do(func() {
			d.mu.Lock()
			delete(d.active, key)
			d.completed[key] = time.Now()
			d.pruneLocked()
			d.mu.Unlock()
		})
	}
	return true, release
}

// Complete marks a key as recently processed without holding it active.
func (d *Dedupe) Complete(key string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.completed[key] = time.Now()
	d.pruneLocked()
}

// WasRecentlyCompleted reports whether the key was handled shortly before.
func (d *Dedupe) WasRecentlyCompleted(key string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	expiry, ok := d.completed[key]
	if !ok {
		return false
	}
	if time.Since(expiry) > d.completedTTL {
		delete(d.completed, key)
		return false
	}
	return true
}

// IsActive reports whether a leader currently processes the key.
func (d *Dedupe) IsActive(key string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	_, ok := d.active[key]
	return ok
}

// Len returns the number of tracked keys.
func (d *Dedupe) Len() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.active) + len(d.completed)
}

func (d *Dedupe) pruneLocked() {
	now := time.Now()
	for key, entry := range d.active {
		if now.Sub(entry.created) > d.activeTTL {
			delete(d.active, key)
		}
	}
	for key, at := range d.completed {
		if now.Sub(at) > d.completedTTL {
			delete(d.completed, key)
		}
	}
	if len(d.completed)+len(d.active) > d.maxEntries {
		limit := d.maxEntries / 2
		count := 0
		for key := range d.completed {
			if count >= limit {
				break
			}
			delete(d.completed, key)
			count++
		}
	}
}
