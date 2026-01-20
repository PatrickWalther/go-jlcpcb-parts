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
		UrlSuffix: "6597989-MPM3506AGQVZ/C5676715",
	}

	expectedURL := "https://jlcpcb.com/parts/details/6597989-MPM3506AGQVZ/C5676715"
	actualURL := product.GetProductURL()

	if actualURL != expectedURL {
		t.Errorf("expected URL %s, got %s", expectedURL, actualURL)
	}
}
