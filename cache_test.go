package jlcpcb

import (
	"testing"
	"time"
)

// TestMemoryCacheSet tests basic cache set operation.
func TestMemoryCacheSet(t *testing.T) {
	cache := NewMemoryCache()
	defer cache.Clear()

	key := "test:key"
	value := []byte("test value")

	cache.Set(key, value, 1*time.Minute)

	// Verify it's retrievable
	retrieved, ok := cache.Get(key)
	if !ok {
		t.Fatal("expected to find value in cache")
	}

	if string(retrieved) != string(value) {
		t.Errorf("expected value %s, got %s", value, retrieved)
	}
}

// TestMemoryCacheGet tests basic cache get operation.
func TestMemoryCacheGet(t *testing.T) {
	cache := NewMemoryCache()
	defer cache.Clear()

	key := "test:key"
	value := []byte("test value")

	cache.Set(key, value, 1*time.Minute)

	retrieved, ok := cache.Get(key)
	if !ok {
		t.Fatal("expected to find value in cache")
	}

	if string(retrieved) != string(value) {
		t.Errorf("expected value %s, got %s", value, retrieved)
	}
}

// TestMemoryCacheGetMissing tests cache get for missing key.
func TestMemoryCacheGetMissing(t *testing.T) {
	cache := NewMemoryCache()

	_, ok := cache.Get("nonexistent")
	if ok {
		t.Fatal("expected cache miss for nonexistent key")
	}
}

// TestMemoryCacheDelete tests cache delete operation.
func TestMemoryCacheDelete(t *testing.T) {
	cache := NewMemoryCache()
	defer cache.Clear()

	key := "test:key"
	cache.Set(key, []byte("value"), 1*time.Minute)

	// Verify it exists
	_, ok := cache.Get(key)
	if !ok {
		t.Fatal("expected value in cache after set")
	}

	cache.Delete(key)

	_, ok = cache.Get(key)
	if ok {
		t.Fatal("expected cache miss after delete")
	}
}

// TestMemoryCacheTTL tests that expired entries are not returned.
func TestMemoryCacheTTL(t *testing.T) {
	cache := NewMemoryCache()
	defer cache.Clear()

	key := "test:key"
	cache.Set(key, []byte("value"), 100*time.Millisecond)

	// Should be available immediately
	_, ok := cache.Get(key)
	if !ok {
		t.Fatal("expected value in cache immediately after set")
	}

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	_, ok = cache.Get(key)
	if ok {
		t.Fatal("expected cache miss after TTL expiration")
	}
}

// TestMemoryCacheMultipleEntries tests cache with multiple entries.
func TestMemoryCacheMultipleEntries(t *testing.T) {
	cache := NewMemoryCache()
	defer cache.Clear()

	entries := map[string][]byte{
		"key1": []byte("value1"),
		"key2": []byte("value2"),
		"key3": []byte("value3"),
	}

	for key, value := range entries {
		cache.Set(key, value, 1*time.Minute)
	}

	for key, expectedValue := range entries {
		value, ok := cache.Get(key)
		if !ok {
			t.Errorf("expected to find key %s in cache", key)
			continue
		}
		if string(value) != string(expectedValue) {
			t.Errorf("expected value %s for key %s, got %s", expectedValue, key, value)
		}
	}
}

// TestMemoryCacheClear tests clearing all cache entries.
func TestMemoryCacheClear(t *testing.T) {
	cache := NewMemoryCache()

	// Add multiple entries
	for i := 0; i < 5; i++ {
		cache.Set("key"+string(rune(i)), []byte("value"), 1*time.Minute)
	}

	cache.Clear()

	// Verify all entries are cleared
	_, ok := cache.Get("key0")
	if ok {
		t.Fatal("expected cache to be empty after clear")
	}
}

// TestMemoryCacheOverwrite tests overwriting existing cache entries.
func TestMemoryCacheOverwrite(t *testing.T) {
	cache := NewMemoryCache()
	defer cache.Clear()

	key := "test:key"

	cache.Set(key, []byte("value1"), 1*time.Minute)

	// Overwrite with new value
	cache.Set(key, []byte("value2"), 1*time.Minute)

	value, ok := cache.Get(key)
	if !ok {
		t.Fatal("expected to find value in cache")
	}

	if string(value) != "value2" {
		t.Errorf("expected new value value2, got %s", value)
	}
}

// TestMemoryCacheEmptyValue tests storing empty values.
func TestMemoryCacheEmptyValue(t *testing.T) {
	cache := NewMemoryCache()
	defer cache.Clear()

	key := "test:key"
	cache.Set(key, []byte(""), 1*time.Minute)

	value, ok := cache.Get(key)
	if !ok {
		t.Fatal("expected to find empty value in cache")
	}

	if len(value) != 0 {
		t.Errorf("expected empty value, got %v", value)
	}
}

// TestCacheInterface tests that MemoryCache implements Cache interface.
func TestCacheInterface(t *testing.T) {
	var _ Cache = (*MemoryCache)(nil)
}
