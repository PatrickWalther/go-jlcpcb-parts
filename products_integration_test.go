//go:build integration
// +build integration

package jlcpcb

import (
	"context"
	"testing"
	"time"
)

func TestIntegrationSearchKeywordBasic(t *testing.T) {
	client := NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	resp, err := client.Search.Keyword(ctx, &SearchRequest{
		Keyword:  "led",
		PageSize: 5,
	})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if len(resp.Products) == 0 {
		t.Fatal("expected at least one product")
	}
}

func TestIntegrationSearchKeywordKnownMPN(t *testing.T) {
	client := NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	resp, err := client.Search.Keyword(ctx, &SearchRequest{
		Keyword: "CGJ2B2C0G1H390J050BA",
	})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if resp == nil || len(resp.Products) == 0 {
		t.Fatal("expected product result for known MPN")
	}

	found := false
	for _, p := range resp.Products {
		if p.ComponentCode == "C3900982" || p.ComponentModelEn == "CGJ2B2C0G1H390J050BA" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected known part in results, got %d items", len(resp.Products))
	}
}

func TestIntegrationProductDetails(t *testing.T) {
	client := NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	product, err := client.Product.Details(ctx, "C3900982")
	if err != nil {
		t.Fatalf("product details failed: %v", err)
	}
	if product == nil {
		t.Fatal("expected non-nil product")
	}
	if product.ComponentCode == "" {
		t.Fatal("expected component code")
	}
}

func TestIntegrationSearchAndDetailsFlow(t *testing.T) {
	client := NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	searchResp, err := client.Search.Keyword(ctx, &SearchRequest{
		Keyword:  "resistor",
		PageSize: 1,
	})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(searchResp.Products) == 0 {
		t.Fatal("expected at least one search result")
	}

	code := searchResp.Products[0].ComponentCode
	product, err := client.Product.Details(ctx, code)
	if err != nil {
		t.Fatalf("details failed: %v", err)
	}
	if product.ComponentCode == "" {
		t.Fatal("expected details response with component code")
	}
}
