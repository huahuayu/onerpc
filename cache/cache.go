package cache

import (
	"sync"
	"time"
)

// ICache defines the methods for a generic key-value cache.
type ICache[K comparable, V any] interface {
	Set(key K, value V, ttl ...time.Duration)
	Get(key K) (V, bool)
	Remove(key K)
	Pop(key K) (V, bool)
}

// TTLCache is a generic in-memory key-value cache with optional TTL support.
type TTLCache[K comparable, V any] struct {
	items         map[K]*item[V]
	mu            sync.RWMutex
	cleanInterval *time.Duration
}

type item[V any] struct {
	value  V
	expiry *time.Time
}

var (
	defaultCleanInterval = 5 * time.Minute
)

// New creates a new TTLCache instance
func New[K comparable, V any](cleanInterval ...time.Duration) ICache[K, V] {
	c := &TTLCache[K, V]{
		items: make(map[K]*item[V]),
	}

	if len(cleanInterval) > 0 {
		c.cleanInterval = &cleanInterval[0]
	} else {
		c.cleanInterval = &defaultCleanInterval
	}
	go c.cleanupExpiredItems()

	return c
}

// Set adds or updates a key-value pair in the cache with optional TTL, if no TTL is specified the item will not expire.
func (c *TTLCache[K, V]) Set(key K, value V, ttl ...time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var expiry *time.Time
	if len(ttl) > 0 {
		t := time.Now().Add(ttl[0])
		expiry = &t
	}
	c.items[key] = &item[V]{value: value, expiry: expiry}
}

// Get retrieves the value associated with the given key.
func (c *TTLCache[K, V]) Get(key K) (V, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, found := c.items[key]
	if !found || (item.expiry != nil && item.expiry.Before(time.Now())) {
		var zeroV V
		return zeroV, false
	}
	return item.value, true
}

// Remove deletes the key-value pair with the specified key.
func (c *TTLCache[K, V]) Remove(key K) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.items, key)
}

// Pop removes and returns the value associated with the specified key.
func (c *TTLCache[K, V]) Pop(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	item, found := c.items[key]
	if found {
		delete(c.items, key)
		return item.value, true
	}

	var zeroV V
	return zeroV, false
}

// cleanupExpiredItems periodically removes expired items.
func (c *TTLCache[K, V]) cleanupExpiredItems() {
	ticker := time.NewTicker(*c.cleanInterval)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		for key, item := range c.items {
			if item.expiry != nil && item.expiry.Before(time.Now()) {
				delete(c.items, key)
			}
		}
		c.mu.Unlock()
	}
}
