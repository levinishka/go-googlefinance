package cache

import (
	"fmt"
	"time"

	"github.com/dgraph-io/ristretto"
)

type Cache struct {
	client *ristretto.Cache
	ttl    time.Duration
}

func NewCache(numCounters int64, ttlInSec int64) (*Cache, error) {
	client, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: numCounters,
		MaxCost:     numCounters,
		BufferItems: 64,
	})
	if err != nil {
		return nil, fmt.Errorf("NewCache: unable to initialize new cache: %v", err)
	}

	return &Cache{
		client: client,
		ttl:    time.Duration(ttlInSec) * time.Second,
	}, nil
}

func (c *Cache) Set(key string, value float64) {
	c.client.SetWithTTL(key, value, 1, c.ttl)
}

func (c *Cache) Get(key string) (float64, bool) {
	if value, found := c.client.Get(key); found {
		return value.(float64), found
	}

	return -1, false
}

func (c *Cache) Clear() {
	c.client.Clear()
}
