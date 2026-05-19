// Package cache provides a TTL-bounded LRU with singleflight deduplication.
// Errors are not cached — failed fetches leave the slot empty so the next
// caller retries instead of seeing a stale "not found".
package cache

import (
	"time"

	lru "github.com/hashicorp/golang-lru/v2/expirable"
	"golang.org/x/sync/singleflight"
)

type TTLCache[V any] struct {
	lru *lru.LRU[string, V]
	sf  singleflight.Group
}

func New[V any](size int, ttl time.Duration) *TTLCache[V] {
	return &TTLCache[V]{
		lru: lru.NewLRU[string, V](size, nil, ttl),
	}
}

func (c *TTLCache[V]) Get(key string, fetch func() (V, error)) (V, error) {
	if v, ok := c.lru.Get(key); ok {
		return v, nil
	}
	v, err, _ := c.sf.Do(key, func() (any, error) {
		if v, ok := c.lru.Get(key); ok {
			return v, nil
		}
		v, err := fetch()
		if err != nil {
			return v, err
		}
		c.lru.Add(key, v)
		return v, nil
	})
	if err != nil {
		var zero V
		return zero, err
	}
	return v.(V), nil
}
