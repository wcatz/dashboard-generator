package server

import (
	"sync"
	"time"
)

// VariableCache caches variable values to avoid repeated Prometheus queries.
type VariableCache struct {
	mu    sync.RWMutex
	cache map[string]VariableCacheEntry
	ttl   time.Duration
}

// VariableCacheEntry holds cached values with expiration.
type VariableCacheEntry struct {
	Values    []string
	ExpiresAt time.Time
}

// NewVariableCache creates a new cache with the given TTL (default: 5 minutes).
func NewVariableCache(ttl time.Duration) *VariableCache {
	if ttl == 0 {
		ttl = 5 * time.Minute
	}
	return &VariableCache{
		cache: make(map[string]VariableCacheEntry),
		ttl:   ttl,
	}
}

// Get retrieves cached values if they exist and haven't expired.
func (vc *VariableCache) Get(key string) ([]string, bool) {
	vc.mu.RLock()
	defer vc.mu.RUnlock()
	
	entry, ok := vc.cache[key]
	if !ok || time.Now().After(entry.ExpiresAt) {
		return nil, false
	}
	return entry.Values, true
}

// Set stores values in the cache with the configured TTL.
func (vc *VariableCache) Set(key string, values []string) {
	vc.mu.Lock()
	defer vc.mu.Unlock()
	
	vc.cache[key] = VariableCacheEntry{
		Values:    values,
		ExpiresAt: time.Now().Add(vc.ttl),
	}
}

// Clear removes all cached entries.
func (vc *VariableCache) Clear() {
	vc.mu.Lock()
	defer vc.mu.Unlock()
	vc.cache = make(map[string]VariableCacheEntry)
}

// Invalidate removes a specific cache entry.
func (vc *VariableCache) Invalidate(key string) {
	vc.mu.Lock()
	defer vc.mu.Unlock()
	delete(vc.cache, key)
}
