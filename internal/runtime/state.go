package runtime

import (
	"sync"
	"time"
)

// StateStore is a small in-memory key/value store with per-entry expiry. It
// backs the inline flows that need to remember a payload between the inline
// query and the button press.
type StateStore struct {
	mu      sync.Mutex
	entries map[string]stateEntry
	ttl     time.Duration
}

type stateEntry struct {
	value     any
	expiresAt time.Time
}

// NewStateStore builds a store whose entries expire after the given duration.
func NewStateStore(ttl time.Duration) *StateStore {
	if ttl <= 0 {
		ttl = 30 * time.Minute
	}
	return &StateStore{entries: map[string]stateEntry{}, ttl: ttl}
}

// Set stores a value.
func (s *StateStore) Set(key string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[key] = stateEntry{value: value, expiresAt: time.Now().Add(s.ttl)}
}

// Get returns a stored value.
func (s *StateStore) Get(key string) (any, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.entries[key]
	if !ok {
		return nil, false
	}
	if time.Now().After(entry.expiresAt) {
		delete(s.entries, key)
		return nil, false
	}
	return entry.value, true
}

// Delete removes a stored value.
func (s *StateStore) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.entries, key)
}

// Len returns the number of stored entries.
func (s *StateStore) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.entries)
}

// Prune removes every expired entry.
func (s *StateStore) Prune() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for key, entry := range s.entries {
		if now.After(entry.expiresAt) {
			delete(s.entries, key)
		}
	}
}
