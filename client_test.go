package jlcpcb

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

func TestNewClientDefaults(t *testing.T) {
	client := NewClient()

	if client.baseURL != defaultBaseURL {
		t.Fatalf("expected base URL %q, got %q", defaultBaseURL, client.baseURL)
	}
	if client.apiRoot != defaultAPIRoot {
		t.Fatalf("expected API root %q, got %q", defaultAPIRoot, client.apiRoot)
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

func TestClientEndpointOptions(t *testing.T) {
	tests := []struct {
		name     string
		opts     []ClientOption
		wantBase string
		wantRoot string
	}{
		{
			name:     "defaults",
			wantBase: "https://jlcpcb.com/api/overseas-pcb-order/v1/shoppingCart/smtGood",
			wantRoot: "https://jlcpcb.com/api",
		},
		{
			name:     "API root moves the search",
			opts:     []ClientOption{WithAPIRoot("http://127.0.0.1:8080/api/")},
			wantBase: "http://127.0.0.1:8080/api/overseas-pcb-order/v1/shoppingCart/smtGood",
			wantRoot: "http://127.0.0.1:8080/api",
		},
		{
			name:     "base URL of a test server keeps the default root",
			opts:     []ClientOption{WithBaseURL("http://127.0.0.1:8080/")},
			wantBase: "http://127.0.0.1:8080",
			wantRoot: "https://jlcpcb.com/api",
		},
		{
			name:     "base URL of a full API proxy gives the root",
			opts:     []ClientOption{WithBaseURL("https://proxy.example/jlc/overseas-pcb-order/v1/shoppingCart/smtGood/")},
			wantBase: "https://proxy.example/jlc/overseas-pcb-order/v1/shoppingCart/smtGood",
			wantRoot: "https://proxy.example/jlc",
		},
		{
			name:     "base URL and API root",
			opts:     []ClientOption{WithBaseURL("http://search.test"), WithAPIRoot("http://root.test")},
			wantBase: "http://search.test",
			wantRoot: "http://root.test",
		},
		{
			name:     "API root wins over a proxy base URL",
			opts:     []ClientOption{WithAPIRoot("http://root.test"), WithBaseURL("https://proxy.example/jlc/overseas-pcb-order/v1/shoppingCart/smtGood")},
			wantBase: "https://proxy.example/jlc/overseas-pcb-order/v1/shoppingCart/smtGood",
			wantRoot: "http://root.test",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.opts...)
			if client.baseURL != tt.wantBase || client.apiRoot != tt.wantRoot {
				t.Errorf("base URL %q, root %q, want %q, %q", client.baseURL, client.apiRoot, tt.wantBase, tt.wantRoot)
			}
		})
	}
}

func TestWithAPIRootServesAllEndpoints(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case componentDetailPath:
			_, _ = w.Write([]byte(`{"code":200,"data":{"componentCode":"C1525","lcscComponentId":1877}}`))
		case compareDetailsPath:
			_, _ = w.Write([]byte(`{"code":200,"data":[{"urlSuffix":"x/C1525","componentDetailVo":{"componentCode":"C1525","lcscComponentId":1877}}]}`))
		default:
			_, _ = w.Write([]byte(`{"code":200,"data":{"componentPageInfo":{"total":0,"list":[]}}}`))
		}
	}))
	defer server.Close()

	client := NewClient(WithAPIRoot(server.URL), WithHTTPClient(server.Client()), WithoutCache())
	ctx := context.Background()
	if _, err := client.Search.Keyword(ctx, &SearchRequest{Keyword: "C1525"}); err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if _, err := client.Product.Detail(ctx, "C1525"); err != nil {
		t.Fatalf("detail failed: %v", err)
	}
	if _, err := client.Product.DetailsByIDs(ctx, []int64{1877}); err != nil {
		t.Fatalf("batch detail failed: %v", err)
	}

	want := []string{smtGoodPath + "/selectSmtComponentList/v2", componentDetailPath, compareDetailsPath}
	if !reflect.DeepEqual(paths, want) {
		t.Errorf("paths = %v, want %v", paths, want)
	}
}
