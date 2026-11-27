package cache

import (
	"fmt"
	"sync"
	"time"
)

type entry[V any] struct {
	value     V
	expiresAt time.Time
}

type Cache[V any] struct {
	mu      sync.RWMutex
	entries map[string]entry[V]
	maxSize int64
	ttl     time.Duration
}

func NewCache[V any](maxSize int64, ttlInSec int64) (*Cache[V], error) {
	if maxSize <= 0 {
		return nil, fmt.Errorf("NewCache: maxSize should be > 0, %d given", maxSize)
	}

	return &Cache[V]{
		entries: make(map[string]entry[V]),
		maxSize: maxSize,
		ttl:     time.Duration(ttlInSec) * time.Second,
	}, nil
}

func (c *Cache[V]) Set(key string, value V) bool {
	if c.ttl <= 0 {
		return false
	}

	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.entries[key]; !exists && int64(len(c.entries)) >= c.maxSize {
		for k, e := range c.entries {
			if now.After(e.expiresAt) {
				delete(c.entries, k)
			}
		}
		if int64(len(c.entries)) >= c.maxSize {
			for k := range c.entries {
				delete(c.entries, k)
				break
			}
		}
	}

	c.entries[key] = entry[V]{
		value:     value,
		expiresAt: now.Add(c.ttl),
	}
	return true
}

func (c *Cache[V]) Get(key string) (V, bool) {
	c.mu.RLock()
	e, found := c.entries[key]
	c.mu.RUnlock()

	if !found || time.Now().After(e.expiresAt) {
		var zero V
		return zero, false
	}

	return e.value, true
}

func (c *Cache[V]) Clear() {
	c.mu.Lock()
	c.entries = make(map[string]entry[V])
	c.mu.Unlock()
}
