package jlcpcb

import (
	"net/http"
	"strings"
	"time"
)

const (
	// defaultAPIRoot is the root of the JLCPCB web API.
	defaultAPIRoot = "https://jlcpcb.com/api"
	// smtGoodPath is the path of the parts service below the API root. The
	// search endpoint and compareComponentDetails are below this path.
	smtGoodPath = "/overseas-pcb-order/v1/shoppingCart/smtGood"
	// defaultBaseURL is the default base URL of the search endpoint.
	defaultBaseURL   = defaultAPIRoot + smtGoodPath
	defaultTimeout   = 30 * time.Second
	defaultRateLimit = 5.0 // requests per second
	userAgent        = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
)

// service is the base type shared by all services.
type service struct {
	client *Client
}

const (
	// defaultFacetsTTL is the default cache time of a facet answer.
	defaultFacetsTTL = 15 * time.Minute
	// defaultCategoryTTL is the default cache time of a category name.
	defaultCategoryTTL = 24 * time.Hour
)

// CacheConfig contains response cache settings.
type CacheConfig struct {
	Enabled    bool
	SearchTTL  time.Duration
	DetailsTTL time.Duration
	// FacetsTTL is the cache time of SearchService.Facets. 0 uses the
	// default of 15 minutes.
	FacetsTTL time.Duration
	// CategoryTTL is the cache time of CategoryService.Info. 0 uses the
	// default of 24 hours.
	CategoryTTL time.Duration
}

// DefaultCacheConfig returns the default cache settings.
func DefaultCacheConfig() CacheConfig {
	return CacheConfig{
		Enabled:     true,
		SearchTTL:   5 * time.Minute,
		DetailsTTL:  5 * time.Minute,
		FacetsTTL:   defaultFacetsTTL,
		CategoryTTL: defaultCategoryTTL,
	}
}

// facetsTTL returns FacetsTTL, or the default when FacetsTTL is 0 or less.
func (cc CacheConfig) facetsTTL() time.Duration {
	if cc.FacetsTTL <= 0 {
		return defaultFacetsTTL
	}
	return cc.FacetsTTL
}

// categoryTTL returns CategoryTTL, or the default when CategoryTTL is 0 or
// less.
func (cc CacheConfig) categoryTTL() time.Duration {
	if cc.CategoryTTL <= 0 {
		return defaultCategoryTTL
	}
	return cc.CategoryTTL
}

// Client is a JLCPCB Parts API client.
type Client struct {
	httpClient  *http.Client
	baseURL     string
	apiRoot     string
	rateLimiter *RateLimiter
	cache       Cache
	cacheConfig CacheConfig
	retryConfig RetryConfig

	common   service
	Search   *SearchService
	Product  *ProductService
	Assembly *AssemblyService
	Category *CategoryService
}

// ClientOption is a function that configures a Client.
type ClientOption func(*Client)

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(client *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = client
	}
}

// WithBaseURL sets a custom base URL for the search endpoint. The client
// sends a search to the base URL plus "/selectSmtComponentList/v2".
//
// WithBaseURL does not change the API root of the detail endpoints, with one
// exception: when the base URL ends with the default parts service path
// "/overseas-pcb-order/v1/shoppingCart/smtGood" and WithAPIRoot is not set,
// the client uses the part before this path as the API root. Thus a proxy of
// the full API needs only WithBaseURL.
func WithBaseURL(baseURL string) ClientOption {
	return func(c *Client) {
		c.baseURL = strings.TrimRight(baseURL, "/")
	}
}

// WithAPIRoot sets the root of the JLCPCB web API. The default root is
// "https://jlcpcb.com/api". ProductService.Detail,
// ProductService.DetailsByIDs, SearchService.Facets, AssemblyService and
// CategoryService send their requests below this root.
//
// When WithBaseURL is not set, the search endpoint also moves below this
// root. Thus one test server can serve all endpoints. WithBaseURL overrides
// the search base URL.
func WithAPIRoot(root string) ClientOption {
	return func(c *Client) {
		c.apiRoot = strings.TrimRight(root, "/")
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
		rateLimiter: NewRateLimiter(defaultRateLimit),
		cacheConfig: DefaultCacheConfig(),
		retryConfig: DefaultRetryConfig(),
	}

	for _, opt := range opts {
		opt(c)
	}
	c.resolveEndpoints()

	if c.cacheConfig.Enabled && c.cache == nil {
		c.cache = NewMemoryCache()
	}

	c.common.client = c
	c.Search = (*SearchService)(&c.common)
	c.Product = (*ProductService)(&c.common)
	c.Assembly = (*AssemblyService)(&c.common)
	c.Category = (*CategoryService)(&c.common)

	return c
}

// resolveEndpoints sets the API root and the search base URL that the
// options did not set. An explicit option always wins.
func (c *Client) resolveEndpoints() {
	if c.apiRoot == "" {
		c.apiRoot = defaultAPIRoot
		if root, ok := strings.CutSuffix(c.baseURL, smtGoodPath); ok && root != "" {
			c.apiRoot = root
		}
	}
	if c.baseURL == "" {
		c.baseURL = c.apiRoot + smtGoodPath
	}
}

// ClearCache clears all cached responses.
func (c *Client) ClearCache() {
	if c.cache != nil {
		c.cache.Clear()
	}
}
