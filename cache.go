package jlcpcb

import (
	"encoding/json"
	"sync"
	"time"
)

// Cache interface defines caching behavior.
type Cache interface {
	Get(key string) ([]byte, bool)
	Set(key string, value []byte, ttl time.Duration)
	Delete(key string)
	Clear()
}

// MemoryCache is an in-memory cache implementation.
type MemoryCache struct {
	mu    sync.RWMutex
	items map[string]*cacheItem
}

type cacheItem struct {
	data      []byte
	expiresAt time.Time
}

// NewMemoryCache creates a new in-memory cache.
func NewMemoryCache() *MemoryCache {
	return &MemoryCache{
		items: make(map[string]*cacheItem),
	}
}

// Get retrieves a value from the cache.
func (mc *MemoryCache) Get(key string) ([]byte, bool) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	item, ok := mc.items[key]
	if !ok {
		return nil, false
	}

	if time.Now().After(item.expiresAt) {
		return nil, false
	}

	return item.data, true
}

// Set stores a value in the cache with a TTL.
func (mc *MemoryCache) Set(key string, value []byte, ttl time.Duration) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.items[key] = &cacheItem{
		data:      value,
		expiresAt: time.Now().Add(ttl),
	}
}

// Delete removes a value from the cache.
func (mc *MemoryCache) Delete(key string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	delete(mc.items, key)
}

// Clear removes all values from the cache.
func (mc *MemoryCache) Clear() {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.items = make(map[string]*cacheItem)
}

// cachedValue decodes the cached value of key into v. It returns false when
// the cache is off, when the key is not in the cache, or when the value does
// not decode.
func (c *Client) cachedValue(key string, v interface{}) bool {
	if !c.cacheConfig.Enabled || c.cache == nil {
		return false
	}
	cached, ok := c.cache.Get(key)
	if !ok {
		return false
	}
	return json.Unmarshal(cached, v) == nil
}

// cacheValue stores v as JSON under key for ttl.
func (c *Client) cacheValue(key string, v interface{}, ttl time.Duration) {
	if !c.cacheConfig.Enabled || c.cache == nil {
		return
	}
	if data, err := json.Marshal(v); err == nil {
		c.cache.Set(key, data, ttl)
	}
}
