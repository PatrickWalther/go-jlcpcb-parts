package jlcpcb

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// The category fixtures come from live sort/info responses of 2026-10-02:
// category_info_2929.json for a leaf category and category_info_9.json for
// a first-level category.

// unknownCategoryResponse is the live answer for an unknown category id.
const unknownCategoryResponse = `{"code":500,"data":null,"message":"Internal Server Error","success":false}`

func TestCategoryInfoLeaf(t *testing.T) {
	rec := fixedAPIRecorder(t, loadFixture(t, "category_info_2929.json"))

	info, err := rec.client().Category.Info(context.Background(), 2929)
	if err != nil {
		t.Fatalf("info failed: %v", err)
	}
	want := CategoryInfo{ParentID: 2, ParentName: "Capacitors", LeafID: 2929, LeafName: mlccLeaf}
	if *info != want {
		t.Errorf("info = %+v, want %+v", *info, want)
	}
	if got := info.Category(); got != (Category{Parent: "Capacitors", Leaf: mlccLeaf}) {
		t.Errorf("Category() = %+v", got)
	}

	requests := rec.all()
	if len(requests) != 1 {
		t.Fatalf("server got %d requests, want 1", len(requests))
	}
	if req := requests[0]; req.Method != http.MethodGet || req.Path != categoryInfoPath+"2929" || req.Query != "" || len(req.Body) != 0 {
		t.Errorf("request = %s %s?%s body %q, want GET %s2929", req.Method, req.Path, req.Query, req.Body, categoryInfoPath)
	}
}

func TestCategoryInfoParent(t *testing.T) {
	rec := fixedAPIRecorder(t, loadFixture(t, "category_info_9.json"))

	info, err := rec.client().Category.Info(context.Background(), 9)
	if err != nil {
		t.Fatalf("info failed: %v", err)
	}
	want := CategoryInfo{ParentID: 9, ParentName: "Power Management (PMIC)"}
	if *info != want {
		t.Errorf("info = %+v, want %+v", *info, want)
	}
	if got := info.Category(); got != (Category{Parent: "Power Management (PMIC)"}) {
		t.Errorf("Category() = %+v", got)
	}
}

func TestCategoryInfoUnknownID(t *testing.T) {
	rec := fixedAPIRecorder(t, []byte(unknownCategoryResponse))
	client := rec.client(WithRetryConfig(RetryConfig{MaxRetries: 3, InitialBackoff: 1, MaxBackoff: 1, BackoffMultiplier: 1}))

	_, err := client.Category.Info(context.Background(), 999999999)
	if !errors.Is(err, ErrNotFound) || !errors.Is(err, ErrServer) {
		t.Fatalf("error = %v, want ErrNotFound and ErrServer", err)
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Code != 500 {
		t.Errorf("error = %v, want an APIError with code 500", err)
	}
	// The answer for an unknown id does not change, so Info does not retry.
	if n := len(rec.all()); n != 1 {
		t.Errorf("server got %d requests, want 1", n)
	}
}

func TestCategoryInfoNullData(t *testing.T) {
	rec := fixedAPIRecorder(t, []byte(`{"code":200,"data":null}`))
	if _, err := rec.client().Category.Info(context.Background(), 2929); !errors.Is(err, ErrNotFound) || errors.Is(err, ErrServer) {
		t.Fatalf("error = %v, want ErrNotFound only", err)
	}
}

func TestCategoryInfoRetriesHTTPErrors(t *testing.T) {
	var requests atomic.Int32
	fixture := loadFixture(t, "category_info_2929.json")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requests.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture)
	}))
	defer server.Close()

	client := NewClient(
		WithAPIRoot(server.URL),
		WithHTTPClient(server.Client()),
		WithRetryConfig(RetryConfig{MaxRetries: 2, InitialBackoff: 1, MaxBackoff: 1, BackoffMultiplier: 1}),
		WithoutCache(),
	)
	info, err := client.Category.Info(context.Background(), 2929)
	if err != nil || info.ParentName != "Capacitors" {
		t.Fatalf("info = %+v, %v, want Capacitors after a retry", info, err)
	}
	if n := requests.Load(); n != 2 {
		t.Errorf("server got %d requests, want 2", n)
	}
}

func TestCategoryInfoInvalidID(t *testing.T) {
	rec := fixedAPIRecorder(t, loadFixture(t, "category_info_2929.json"))
	client := rec.client()
	for _, id := range []int{0, -2929} {
		if _, err := client.Category.Info(context.Background(), id); !errors.Is(err, ErrInvalidRequest) {
			t.Errorf("Info(%d) error = %v, want ErrInvalidRequest", id, err)
		}
	}
	if n := len(rec.all()); n != 0 {
		t.Errorf("server got %d requests, want 0", n)
	}
}

func TestCategoryInfoCache(t *testing.T) {
	rec := fixedAPIRecorder(t, loadFixture(t, "category_info_2929.json"))
	cache := newTTLCache()
	client := rec.client(WithCache(cache), WithCacheConfig(DefaultCacheConfig()))

	for i := 0; i < 2; i++ {
		info, err := client.Category.Info(context.Background(), 2929)
		if err != nil || info.LeafID != 2929 {
			t.Fatalf("call %d: info = %+v, %v", i, info, err)
		}
	}
	if n := len(rec.all()); n != 1 {
		t.Errorf("server got %d requests, want 1", n)
	}
	if key, ttl := cache.only(t); key != "category:2929" || ttl != 24*time.Hour {
		t.Errorf("cache key %q, TTL %v, want category:2929 and 24h", key, ttl)
	}
}

func TestCategoryInfoUnknownIDIsNotCached(t *testing.T) {
	rec := fixedAPIRecorder(t, []byte(unknownCategoryResponse))
	cache := NewMemoryCache()
	client := rec.client(WithCache(cache), WithCacheConfig(DefaultCacheConfig()))
	for i := 0; i < 2; i++ {
		if _, err := client.Category.Info(context.Background(), 1); !errors.Is(err, ErrNotFound) {
			t.Fatalf("call %d: error = %v, want ErrNotFound", i, err)
		}
	}
	if n := len(rec.all()); n != 2 {
		t.Errorf("server got %d requests, want 2", n)
	}
}
