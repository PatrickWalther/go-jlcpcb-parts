package jlcpcb

import (
	"net/http"
	"strings"
	"time"
)

const (
	defaultBaseURL   = "https://jlcpcb.com/api/overseas-pcb-order/v1/shoppingCart/smtGood"
	defaultTimeout   = 30 * time.Second
	defaultRateLimit = 5.0 // requests per second
	userAgent        = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
)

// service is the base type shared by all services.
type service struct {
	client *Client
}

// CacheConfig contains response cache settings.
type CacheConfig struct {
	Enabled    bool
	SearchTTL  time.Duration
	DetailsTTL time.Duration
}

// DefaultCacheConfig returns the default cache settings.
func DefaultCacheConfig() CacheConfig {
	return CacheConfig{
		Enabled:    true,
		SearchTTL:  5 * time.Minute,
		DetailsTTL: 5 * time.Minute,
	}
}

// Client is a JLCPCB Parts API client.
type Client struct {
	httpClient  *http.Client
	baseURL     string
	rateLimiter *RateLimiter
	cache       Cache
	cacheConfig CacheConfig
	retryConfig RetryConfig

	common  service
	Search  *SearchService
	Product *ProductService
}

// ClientOption is a function that configures a Client.
type ClientOption func(*Client)

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(client *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = client
	}
}

// WithBaseURL sets a custom base URL.
func WithBaseURL(baseURL string) ClientOption {
	return func(c *Client) {
		c.baseURL = strings.TrimRight(baseURL, "/")
	}
}

// WithRateLimit sets a custom rate limit (requests per second).
func WithRateLimit(rps float64) ClientOption {
	return func(c *Client) {
		c.rateLimiter = NewRateLimiter(rps)
	}
}

// WithCache sets a cache for API responses.
func WithCache(cache Cache) ClientOption {
	return func(c *Client) {
		c.cache = cache
	}
}

// WithCacheConfig sets custom cache settings.
func WithCacheConfig(config CacheConfig) ClientOption {
	return func(c *Client) {
		c.cacheConfig = config
	}
}

// WithoutCache disables response caching.
func WithoutCache() ClientOption {
	return func(c *Client) {
		c.cacheConfig.Enabled = false
	}
}

// WithRetryConfig sets the retry configuration.
func WithRetryConfig(config RetryConfig) ClientOption {
	return func(c *Client) {
		c.retryConfig = config
	}
}

// NewClient creates a new JLCPCB Parts API client.
func NewClient(opts ...ClientOption) *Client {
	c := &Client{
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
		baseURL:     defaultBaseURL,
		rateLimiter: NewRateLimiter(defaultRateLimit),
		cacheConfig: DefaultCacheConfig(),
		retryConfig: DefaultRetryConfig(),
	}

	for _, opt := range opts {
		opt(c)
	}

	if c.cacheConfig.Enabled && c.cache == nil {
		c.cache = NewMemoryCache()
	}

	c.common.client = c
	c.Search = (*SearchService)(&c.common)
	c.Product = (*ProductService)(&c.common)

	return c
}

// ClearCache clears all cached responses.
func (c *Client) ClearCache() {
	if c.cache != nil {
		c.cache.Clear()
	}
}
