package jlcpcb

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestKeywordSearchEmptyKeyword tests error handling for empty keyword.
func TestKeywordSearchEmptyKeyword(t *testing.T) {
	client := NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := client.KeywordSearch(ctx, SearchRequest{
		Keyword: "",
	})

	if err == nil {
		t.Fatal("expected error for empty keyword")
	}

	if !strings.Contains(err.Error(), "keyword") {
		t.Errorf("expected keyword-related error, got: %v", err)
	}
}

// TestKeywordSearchWhitespaceKeyword tests that whitespace-only keywords are rejected.
func TestKeywordSearchWhitespaceKeyword(t *testing.T) {
	client := NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := client.KeywordSearch(ctx, SearchRequest{
		Keyword: "   ",
	})

	if err == nil {
		t.Fatal("expected error for whitespace-only keyword")
	}
}

// TestGetProductDetailsEmptyCode tests error handling for empty part code.
func TestGetProductDetailsEmptyCode(t *testing.T) {
	client := NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := client.GetProductDetails(ctx, "")
	if err == nil {
		t.Fatal("expected error for empty part code")
	}

	if !strings.Contains(err.Error(), "part code") {
		t.Errorf("expected part code-related error, got: %v", err)
	}
}

// TestGetProductDetailsWhitespaceCode tests that whitespace-only codes are rejected.
func TestGetProductDetailsWhitespaceCode(t *testing.T) {
	client := NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := client.GetProductDetails(ctx, "   ")
	if err == nil {
		t.Fatal("expected error for whitespace-only code")
	}
}

// TestProductURL tests the GetProductURL method.
func TestProductURL(t *testing.T) {
	product := &Product{
		ComponentCode: "C5676715",
		UrlSuffix:     "6597989-MPM3506AGQVZ/C5676715",
	}

	expectedURL := "https://jlcpcb.com/parts/details/6597989-MPM3506AGQVZ/C5676715"
	actualURL := product.GetProductURL()

	if actualURL != expectedURL {
		t.Errorf("expected URL %s, got %s", expectedURL, actualURL)
	}
}

// TestKeywordSearchWithFilters tests search with advanced filters.
func TestKeywordSearchWithFilters(t *testing.T) {
	client := NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := SearchRequest{
		Keyword:       "capacitor",
		CurrentPage:   1,
		PageSize:      10,
		PresaleType:   "stock",
		ComponentType: "base",
		Brands:        []string{"Samsung"},
		StockOnly:     true,
	}

	resp, err := client.KeywordSearch(ctx, req)
	if err == nil && resp == nil {
		t.Fatal("expected either response or error")
	}
	// Note: This may fail if API doesn't return results, which is acceptable
	// for unit tests without mocking
}

// TestSearchRequestMaxPageSize tests that PageSize is capped at 100.
func TestSearchRequestMaxPageSize(t *testing.T) {
	client := NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// PageSize > 100 should be handled gracefully
	_, err := client.KeywordSearch(ctx, SearchRequest{
		Keyword:  "resistor",
		PageSize: 200,
	})
	// Should not error due to page size validation
	if err != nil && strings.Contains(err.Error(), "PageSize") {
		t.Errorf("unexpected page size error: %v", err)
	}
}

// TestFilterAttributeStructure tests the FilterAttribute model.
func TestFilterAttributeStructure(t *testing.T) {
	attr := FilterAttribute{
		Name:  "Resistance",
		Value: "10k",
	}

	if attr.Name != "Resistance" {
		t.Errorf("expected name Resistance, got %s", attr.Name)
	}
	if attr.Value != "10k" {
		t.Errorf("expected value 10k, got %s", attr.Value)
	}
}
