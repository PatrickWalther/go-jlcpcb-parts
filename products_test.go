package jlcpcb

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSearchKeywordNilRequest(t *testing.T) {
	client := NewClient()
	_, err := client.Search.Keyword(context.Background(), nil)
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("expected ErrInvalidRequest, got %v", err)
	}
}

func TestSearchKeywordEmptyKeyword(t *testing.T) {
	client := NewClient()
	_, err := client.Search.Keyword(context.Background(), &SearchRequest{Keyword: "   "})
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("expected ErrInvalidRequest, got %v", err)
	}
}

func TestSearchKeywordDefaultPresaleTypeAny(t *testing.T) {
	var captured searchRequestBody
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request failed: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code":    200,
			"message": nil,
			"data": map[string]interface{}{
				"componentPageInfo": map[string]interface{}{
					"total":    1,
					"pageSize": 5,
					"pageNum":  1,
					"list": []map[string]interface{}{
						{"componentCode": "C3900982", "componentModelEn": "CGJ2B2C0G1H390J050BA"},
					},
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL), WithHTTPClient(server.Client()))
	resp, err := client.Search.Keyword(context.Background(), &SearchRequest{
		Keyword:  "CGJ2B2C0G1H390J050BA",
		PageSize: 5,
	})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if captured.PresaleType != "" {
		t.Fatalf("expected empty presale type, got %q", captured.PresaleType)
	}
	if resp.TotalCount != 1 || len(resp.Products) != 1 {
		t.Fatalf("expected one result, got total=%d len=%d", resp.TotalCount, len(resp.Products))
	}
}

func TestSearchKeywordStockOnlySetsPresaleStock(t *testing.T) {
	var captured searchRequestBody
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request failed: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code":    200,
			"message": nil,
			"data": map[string]interface{}{
				"componentPageInfo": map[string]interface{}{
					"total":    0,
					"pageSize": 50,
					"pageNum":  1,
					"list":     []map[string]interface{}{},
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL), WithHTTPClient(server.Client()))
	_, err := client.Search.Keyword(context.Background(), &SearchRequest{
		Keyword:   "led",
		StockOnly: true,
	})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if captured.PresaleType != string(PresaleTypeStock) {
		t.Fatalf("expected presaleType=stock, got %q", captured.PresaleType)
	}
	if !captured.StockFlag {
		t.Fatal("expected stockFlag=true")
	}
}

func TestSearchKeywordCacheKeyIncludesFilters(t *testing.T) {
	var requestCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)

		var req searchRequestBody
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request failed: %v", err)
		}

		code := "C-STOCK"
		if req.PresaleType == string(PresaleTypeBuy) {
			code = "C-BUY"
		}

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code":    200,
			"message": nil,
			"data": map[string]interface{}{
				"componentPageInfo": map[string]interface{}{
					"total":    1,
					"pageSize": 50,
					"pageNum":  1,
					"list": []map[string]interface{}{
						{"componentCode": code},
					},
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(
		WithBaseURL(server.URL),
		WithHTTPClient(server.Client()),
		WithCache(NewMemoryCache()),
	)

	stockResp, err := client.Search.Keyword(context.Background(), &SearchRequest{
		Keyword:     "test",
		PresaleType: PresaleTypeStock,
	})
	if err != nil {
		t.Fatalf("stock search failed: %v", err)
	}
	buyResp, err := client.Search.Keyword(context.Background(), &SearchRequest{
		Keyword:     "test",
		PresaleType: PresaleTypeBuy,
	})
	if err != nil {
		t.Fatalf("buy search failed: %v", err)
	}

	if stockResp.Products[0].ComponentCode == buyResp.Products[0].ComponentCode {
		t.Fatalf("expected different products for stock vs buy filters")
	}
	if requestCount.Load() != 2 {
		t.Fatalf("expected 2 upstream requests, got %d", requestCount.Load())
	}
}

func TestProductDetailsEmptyIdentifier(t *testing.T) {
	client := NewClient()
	_, err := client.Product.Details(context.Background(), " ")
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("expected ErrInvalidRequest, got %v", err)
	}
}

func TestProductDetailsPrefersExactMatch(t *testing.T) {
	target := "CGJ2B2C0G1H390J050BA"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code":    200,
			"message": nil,
			"data": map[string]interface{}{
				"componentPageInfo": map[string]interface{}{
					"total":    2,
					"pageSize": 50,
					"pageNum":  1,
					"list": []map[string]interface{}{
						{"componentCode": "C1111111", "componentModelEn": "OTHER"},
						{"componentCode": "C3900982", "componentModelEn": target},
					},
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL), WithHTTPClient(server.Client()))
	product, err := client.Product.Details(context.Background(), target)
	if err != nil {
		t.Fatalf("product details failed: %v", err)
	}
	if product.ComponentCode != "C3900982" {
		t.Fatalf("expected exact-match component code C3900982, got %s", product.ComponentCode)
	}
}

func TestProductDetailsNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code":    200,
			"message": nil,
			"data": map[string]interface{}{
				"componentPageInfo": map[string]interface{}{
					"total":    0,
					"pageSize": 50,
					"pageNum":  1,
					"list":     []map[string]interface{}{},
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL), WithHTTPClient(server.Client()))
	_, err := client.Product.Details(context.Background(), "C99999999")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestProductDetailsCaching(t *testing.T) {
	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code":    200,
			"message": nil,
			"data": map[string]interface{}{
				"componentPageInfo": map[string]interface{}{
					"total":    1,
					"pageSize": 50,
					"pageNum":  1,
					"list": []map[string]interface{}{
						{
							"componentCode":    "C3900982",
							"componentModelEn": "CGJ2B2C0G1H390J050BA",
							"componentBrandEn": "TDK",
						},
					},
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(
		WithBaseURL(server.URL),
		WithHTTPClient(server.Client()),
		WithCache(NewMemoryCache()),
	)

	first, err := client.Product.Details(context.Background(), "C3900982")
	if err != nil {
		t.Fatalf("first details call failed: %v", err)
	}
	second, err := client.Product.Details(context.Background(), "C3900982")
	if err != nil {
		t.Fatalf("second details call failed: %v", err)
	}

	if first.ComponentCode != second.ComponentCode {
		t.Fatalf("expected cached response to match")
	}
	if requestCount.Load() != 1 {
		t.Fatalf("expected 1 upstream request due to cache hit, got %d", requestCount.Load())
	}
}

func TestSearchKeywordRequestNormalization(t *testing.T) {
	var captured searchRequestBody
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode failed: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code":    200,
			"message": nil,
			"data": map[string]interface{}{
				"componentPageInfo": map[string]interface{}{
					"total":    0,
					"pageSize": 100,
					"pageNum":  1,
					"list":     []map[string]interface{}{},
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL), WithHTTPClient(server.Client()))
	_, err := client.Search.Keyword(context.Background(), &SearchRequest{
		Keyword:  "  led  ",
		Page:     -10,
		PageSize: 999,
	})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if captured.Keyword != "led" {
		t.Fatalf("expected trimmed keyword, got %q", captured.Keyword)
	}
	if captured.CurrentPage != 1 {
		t.Fatalf("expected default page=1, got %d", captured.CurrentPage)
	}
	if captured.PageSize != 100 {
		t.Fatalf("expected max page size clamp=100, got %d", captured.PageSize)
	}
}

func TestSearchKeywordServerContextTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code":    200,
			"message": nil,
			"data": map[string]interface{}{
				"componentPageInfo": map[string]interface{}{
					"total":    0,
					"pageSize": 50,
					"pageNum":  1,
					"list":     []map[string]interface{}{},
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL), WithHTTPClient(server.Client()))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()

	_, err := client.Search.Keyword(ctx, &SearchRequest{Keyword: "led"})
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "context") {
		t.Fatalf("expected context-related error, got %v", err)
	}
}

// fixtureServer serves a fixture and records the pageSize of each request.
func fixtureServer(t *testing.T, fixture string, pageSizes *[]int) *httptest.Server {
	t.Helper()
	body := loadFixture(t, fixture)
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req searchRequestBody
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request failed: %v", err)
		}
		mu.Lock()
		*pageSizes = append(*pageSizes, req.PageSize)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)
	return server
}

func TestProductDetailsPageSize(t *testing.T) {
	tests := []struct {
		identifier string
		fixture    string
		wantCode   string
		wantSize   int
	}{
		{"C7593", "search_C7593.json", "C7593", 10},
		{"c7593", "search_C7593.json", "C7593", 10},
		{"  C2040  ", "search_C2040.json", "C2040", 10},
		{"NE555DR", "search_C7593.json", "C7593", 50},
		{"RP2040", "search_C2040.json", "C2040", 50},
		{"C", "search_C7593.json", "C7593", 50},
		{"C75-93", "search_C7593.json", "C7593", 50},
		{"CL05B104KO5NNNC", "search_C25744.json", "C25744", 50},
	}

	for _, tt := range tests {
		t.Run(tt.identifier, func(t *testing.T) {
			var pageSizes []int
			server := fixtureServer(t, tt.fixture, &pageSizes)
			client := NewClient(WithBaseURL(server.URL), WithHTTPClient(server.Client()), WithoutCache())

			product, err := client.Product.Details(context.Background(), tt.identifier)
			if err != nil {
				t.Fatalf("product details failed: %v", err)
			}
			if product.ComponentCode != tt.wantCode {
				t.Errorf("expected component code %s, got %s", tt.wantCode, product.ComponentCode)
			}
			if len(pageSizes) != 1 || pageSizes[0] != tt.wantSize {
				t.Errorf("expected one request with pageSize=%d, got %v", tt.wantSize, pageSizes)
			}
		})
	}
}

func TestProductDetailsCacheKeepsNewFields(t *testing.T) {
	var pageSizes []int
	server := fixtureServer(t, "search_C2040.json", &pageSizes)
	client := NewClient(
		WithBaseURL(server.URL),
		WithHTTPClient(server.Client()),
		WithCache(NewMemoryCache()),
	)

	first, err := client.Product.Details(context.Background(), "C19400368")
	if err != nil {
		t.Fatalf("first details call failed: %v", err)
	}
	second, err := client.Product.Details(context.Background(), "C19400368")
	if err != nil {
		t.Fatalf("second details call failed: %v", err)
	}

	if len(pageSizes) != 1 {
		t.Fatalf("expected 1 upstream request due to cache hit, got %d", len(pageSizes))
	}
	if !reflect.DeepEqual(first, second) {
		t.Errorf("cached product differs\n got: %+v\nwant: %+v", second, first)
	}
	if second.Buyable() || second.NoBuyReason == "" || second.LibraryType() != LibraryTypeExtended {
		t.Errorf("cached product lost ordering fields: %+v", second)
	}
}

func TestSearchKeywordSendsPackageFilter(t *testing.T) {
	var captured searchRequestBody
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request failed: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code":    200,
			"message": nil,
			"data": map[string]interface{}{
				"componentPageInfo": map[string]interface{}{
					"total":    0,
					"pageSize": 50,
					"pageNum":  1,
					"list":     []map[string]interface{}{},
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL), WithHTTPClient(server.Client()))
	if _, err := client.Search.Keyword(context.Background(), &SearchRequest{
		Keyword:  "18pF",
		Packages: []string{" 0603 ", "", "0402"},
	}); err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(captured.ComponentSpecificationList) != 2 ||
		captured.ComponentSpecificationList[0] != "0603" ||
		captured.ComponentSpecificationList[1] != "0402" {
		t.Fatalf("componentSpecificationList = %v, want [0603 0402]", captured.ComponentSpecificationList)
	}

	if _, err := client.Search.Keyword(context.Background(), &SearchRequest{Keyword: "18pF"}); err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if captured.ComponentSpecificationList == nil || len(captured.ComponentSpecificationList) != 0 {
		t.Fatalf("componentSpecificationList = %v, want an empty list without a package filter", captured.ComponentSpecificationList)
	}
}

func TestSearchKeywordCacheKeyIncludesPackages(t *testing.T) {
	plain := cacheKeyForSearch(&SearchRequest{Keyword: "18pF"}, "")
	filtered := cacheKeyForSearch(&SearchRequest{Keyword: "18pF", Packages: []string{"0603"}}, "")
	if plain == filtered {
		t.Fatal("the cache key must differ when a package filter is set")
	}
}
