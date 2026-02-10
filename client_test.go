package jlcpcb

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestNewClientDefaults(t *testing.T) {
	client := NewClient()
	if client == nil {
		t.Fatal("expected non-nil client")
	}

	if client.baseURL != defaultBaseURL {
		t.Fatalf("expected base URL %q, got %q", defaultBaseURL, client.baseURL)
	}
	if client.httpClient == nil {
		t.Fatal("expected http client to be initialized")
	}
	if client.rateLimiter == nil {
		t.Fatal("expected rate limiter to be initialized")
	}
	if client.Search == nil {
		t.Fatal("expected Search service to be initialized")
	}
	if client.Product == nil {
		t.Fatal("expected Product service to be initialized")
	}
	if !client.cacheConfig.Enabled {
		t.Fatal("expected cache to be enabled by default")
	}
	if client.cache == nil {
		t.Fatal("expected default in-memory cache")
	}
}

func TestNewClientWithHTTPClient(t *testing.T) {
	custom := &http.Client{Timeout: 45 * time.Second}
	client := NewClient(WithHTTPClient(custom))
	if client.httpClient != custom {
		t.Fatal("expected custom HTTP client to be used")
	}
}

func TestNewClientWithBaseURLTrimsTrailingSlash(t *testing.T) {
	client := NewClient(WithBaseURL("https://example.com/api/"))
	if client.baseURL != "https://example.com/api" {
		t.Fatalf("expected trailing slash trimmed, got %q", client.baseURL)
	}
}

func TestNewClientWithRateLimit(t *testing.T) {
	client := NewClient(WithRateLimit(2.0))
	if client.rateLimiter == nil {
		t.Fatal("expected non-nil rate limiter")
	}
}

func TestNewClientWithCache(t *testing.T) {
	cache := NewMemoryCache()
	client := NewClient(WithCache(cache))
	if client.cache != cache {
		t.Fatal("expected custom cache to be set")
	}
}

func TestNewClientWithCacheConfig(t *testing.T) {
	cfg := CacheConfig{
		Enabled:    true,
		SearchTTL:  2 * time.Minute,
		DetailsTTL: 3 * time.Minute,
	}
	client := NewClient(WithCacheConfig(cfg))
	if client.cacheConfig.SearchTTL != cfg.SearchTTL {
		t.Fatalf("expected search TTL %v, got %v", cfg.SearchTTL, client.cacheConfig.SearchTTL)
	}
	if client.cacheConfig.DetailsTTL != cfg.DetailsTTL {
		t.Fatalf("expected details TTL %v, got %v", cfg.DetailsTTL, client.cacheConfig.DetailsTTL)
	}
}

func TestWithoutCache(t *testing.T) {
	client := NewClient(WithoutCache())
	if client.cacheConfig.Enabled {
		t.Fatal("expected cache to be disabled")
	}
}

func TestNewClientWithRetryConfig(t *testing.T) {
	cfg := RetryConfig{
		MaxRetries:        5,
		InitialBackoff:    20 * time.Millisecond,
		MaxBackoff:        time.Second,
		BackoffMultiplier: 2.0,
	}
	client := NewClient(WithRetryConfig(cfg))
	if client.retryConfig.MaxRetries != 5 {
		t.Fatalf("expected max retries 5, got %d", client.retryConfig.MaxRetries)
	}
}

func TestClearCache(t *testing.T) {
	cache := NewMemoryCache()
	client := NewClient(WithCache(cache))
	cache.Set("key", []byte("value"), time.Minute)

	client.ClearCache()
	if _, ok := cache.Get("key"); ok {
		t.Fatal("expected cache key to be cleared")
	}
}

func TestContextCancellation(t *testing.T) {
	client := NewClient()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.Search.Keyword(ctx, &SearchRequest{Keyword: "test"})
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
}

func TestContextTimeout(t *testing.T) {
	client := NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()

	_, err := client.Search.Keyword(ctx, &SearchRequest{Keyword: "test"})
	if err == nil {
		t.Fatal("expected context timeout error")
	}
}
