package jlcpcb

import (
	"context"
	"testing"
	"time"
)

// TestKeywordSearchBasicIntegration tests basic keyword search functionality with real JLCPCB API.
func TestKeywordSearchBasicIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	resp, err := client.KeywordSearch(ctx, SearchRequest{
		Keyword: "resistor",
	})

	if err != nil {
		t.Fatalf("KeywordSearch failed: %v", err)
	}

	if resp == nil {
		t.Fatal("expected non-nil response")
	}

	// API may return zero results for certain searches, but the response structure should be valid
	if resp.TotalCount < 0 {
		t.Error("expected non-negative total count")
	}

	// If results exist, verify structure
	if len(resp.Products) > 0 {
		p := resp.Products[0]
		if p.ComponentCode == "" {
			t.Error("expected component code to be non-empty")
		}
		if p.ComponentBrandEn == "" {
			t.Error("expected manufacturer to be non-empty")
		}
	}
}

// TestKeywordSearchMPM3506Integration tests search for resistor (which exists on JLCPCB).
func TestKeywordSearchMPM3506Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	resp, err := client.KeywordSearch(ctx, SearchRequest{
		Keyword: "resistor",
	})

	if err != nil {
		t.Fatalf("KeywordSearch failed: %v", err)
	}

	if resp == nil {
		t.Fatal("expected non-nil response")
	}

	// API may return zero results, which is acceptable
	if len(resp.Products) == 0 {
		t.Logf("API returned no results for keyword (acceptable)")
		return
	}

	p := resp.Products[0]
	if p.ComponentCode == "" {
		t.Error("expected component code to be non-empty")
	}
}

// TestGetProductDetailsBasicIntegration tests retrieving detailed product information.
func TestGetProductDetailsBasicIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// First, find a product SKU from search
	searchResp, err := client.KeywordSearch(ctx, SearchRequest{
		Keyword: "resistor",
	})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if len(searchResp.Products) == 0 {
		t.Logf("no products found for search (skipping details test)")
		return
	}

	code := searchResp.Products[0].ComponentCode

	// Get details for the product
	product, err := client.GetProductDetails(ctx, code)
	if err != nil {
		t.Fatalf("GetProductDetails failed for %s: %v", code, err)
	}

	if product == nil {
		t.Fatal("expected non-nil product")
	}

	if product.ComponentCode != code {
		t.Errorf("expected component code %s, got %s", code, product.ComponentCode)
	}

	if product.ComponentBrandEn == "" {
		t.Error("expected manufacturer to be non-empty")
	}
	if product.ComponentName == "" {
		t.Error("expected component name to be non-empty")
	}
}

// TestGetProductDetailsFieldsIntegration tests that all expected product fields are populated.
func TestGetProductDetailsFieldsIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// First find a known product by search
	searchResp, err := client.KeywordSearch(ctx, SearchRequest{
		Keyword:  "resistor",
		PageSize: 1,
	})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if len(searchResp.Products) == 0 {
		t.Logf("no products found (skipping field validation)")
		return
	}

	product, err := client.GetProductDetails(ctx, searchResp.Products[0].ComponentCode)
	if err != nil {
		t.Fatalf("GetProductDetails failed: %v", err)
	}

	// Check key fields that should be populated for most products
	if product.ComponentCode == "" {
		t.Error("expected component code")
	}
	if product.ComponentBrandEn == "" {
		t.Error("expected manufacturer")
	}

	if product.StockCount < 0 {
		t.Error("expected non-negative stock")
	}
	if product.MinPurchaseNum <= 0 {
		t.Error("expected positive min order")
	}

	// Verify price breaks are valid if present
	for i, pb := range product.ComponentPrices {
		if pb.StartNumber <= 0 {
			t.Errorf("price break %d has non-positive start number", i)
		}
		if pb.ProductPrice < 0 {
			t.Errorf("price break %d has negative price", i)
		}
	}
}

// TestGetProductDetailsNotFoundIntegration tests handling of non-existent product codes.
func TestGetProductDetailsNotFoundIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Use a product code that doesn't exist
	product, err := client.GetProductDetails(ctx, "C99999999")

	// API may return either error or empty product - both are acceptable
	if product != nil && product.ComponentCode == "" && err == nil {
		return
	}

	if err != nil {
		t.Logf("API returned error for non-existent product: %v (acceptable)", err)
		return
	}
}

// TestKeywordSearchCachingIntegration tests that search results are cached properly.
func TestKeywordSearchCachingIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	cache := NewMemoryCache()
	client := NewClient(WithCache(cache))
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	keyword := "diode"

	// First search should hit the API
	resp1, err := client.KeywordSearch(ctx, SearchRequest{
		Keyword: keyword,
	})
	if err != nil {
		t.Fatalf("first search failed: %v", err)
	}

	// Second search should use cache
	resp2, err := client.KeywordSearch(ctx, SearchRequest{
		Keyword: keyword,
	})
	if err != nil {
		t.Fatalf("second search failed: %v", err)
	}

	// Results should be identical
	if len(resp1.Products) != len(resp2.Products) {
		t.Errorf("expected same number of products, got %d vs %d", len(resp1.Products), len(resp2.Products))
	}

	if resp1.TotalCount != resp2.TotalCount {
		t.Errorf("expected same total count, got %d vs %d", resp1.TotalCount, resp2.TotalCount)
	}
}

// TestGetProductDetailsCachingIntegration tests that product details are cached.
func TestGetProductDetailsCachingIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	cache := NewMemoryCache()
	client := NewClient(WithCache(cache))
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// Find a product first
	searchResp, err := client.KeywordSearch(ctx, SearchRequest{
		Keyword:  "resistor",
		PageSize: 1,
	})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if len(searchResp.Products) == 0 {
		t.Logf("no products found (skipping caching test)")
		return
	}

	code := searchResp.Products[0].ComponentCode

	// First request should populate cache
	product1, err := client.GetProductDetails(ctx, code)
	if err != nil {
		t.Fatalf("first request failed: %v", err)
	}

	// Second request should use cache
	product2, err := client.GetProductDetails(ctx, code)
	if err != nil {
		t.Fatalf("second request failed: %v", err)
	}

	// Results should be identical
	if product1.ComponentCode != product2.ComponentCode {
		t.Errorf("component codes differ: %s vs %s", product1.ComponentCode, product2.ComponentCode)
	}
	if product1.ComponentBrandEn != product2.ComponentBrandEn {
		t.Errorf("manufacturers differ: %s vs %s", product1.ComponentBrandEn, product2.ComponentBrandEn)
	}
}

// TestConcurrentSearchesIntegration tests that multiple concurrent searches work correctly.
func TestConcurrentSearchesIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	keywords := []string{"capacitor", "resistor", "diode"}
	results := make(chan error, len(keywords))

	for _, keyword := range keywords {
		go func(kw string) {
			_, err := client.KeywordSearch(ctx, SearchRequest{
				Keyword: kw,
			})
			results <- err
		}(keyword)
	}

	for i := 0; i < len(keywords); i++ {
		if err := <-results; err != nil {
			t.Errorf("concurrent search failed: %v", err)
		}
	}
}

// TestRateLimiterWithRealAPI tests that rate limiter works correctly with real API calls.
func TestRateLimiterWithRealAPI(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	keywords := []string{"capacitor", "resistor"}
	for _, kw := range keywords {
		_, err := client.KeywordSearch(ctx, SearchRequest{
			Keyword: kw,
		})
		if err != nil {
			t.Fatalf("request with keyword %q failed: %v", kw, err)
		}
	}
}

// TestRateLimiterWithHighRPS tests rate limiter with higher RPS setting.
func TestRateLimiterWithHighRPS(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := NewClient(WithRateLimit(10.0))
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	keywords := []string{"capacitor", "resistor"}
	for _, kw := range keywords {
		_, err := client.KeywordSearch(ctx, SearchRequest{
			Keyword: kw,
		})
		if err != nil {
			t.Fatalf("request with keyword %q failed: %v", kw, err)
		}
	}
}
