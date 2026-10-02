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

// contractClient returns a client for the search contract tests. It sends
// at most 1 request per second and does not cache responses.
func contractClient() *Client {
	return NewClient(WithRateLimit(1), WithoutCache())
}

// TestIntegrationSearchIncludePreferredTotals checks that the server honors
// preferredComponentFlag: basic OR preferred = basic + preferred.
func TestIntegrationSearchIncludePreferredTotals(t *testing.T) {
	client := contractClient()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	total := func(name string, req SearchRequest) int {
		t.Helper()
		req.Keyword = "SOIC-8"
		req.PageSize = 1
		start := time.Now()
		resp, err := client.Search.Keyword(ctx, &req)
		if err != nil {
			t.Fatalf("%s search failed after %s: %v", name, time.Since(start), err)
		}
		t.Logf("%s: total %d (%s)", name, resp.TotalCount, time.Since(start))
		return resp.TotalCount
	}

	basicOrPreferred := total("basic or preferred", SearchRequest{ComponentType: ComponentTypeBase, IncludePreferred: true})
	basic := total("basic", SearchRequest{ComponentType: ComponentTypeBase})
	preferred := total("preferred", SearchRequest{IncludePreferred: true})

	if preferred == 0 {
		t.Errorf("preferred-only search returned no parts")
	}
	if basicOrPreferred != basic+preferred {
		t.Errorf("basic or preferred = %d, want basic %d + preferred %d", basicOrPreferred, basic, preferred)
	}
}

// TestIntegrationSearchAttributeFilters checks the componentAttributeList
// map shape. The server rejects the older shape with envelope code 101.
func TestIntegrationSearchAttributeFilters(t *testing.T) {
	client := contractClient()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	values := []string{"100nF", "1uF"}
	start := time.Now()
	resp, err := client.Search.Query(ctx, &SearchRequest{
		PageSize:         50,
		Category:         Category{Parent: "Capacitors", Leaf: "Multilayer Ceramic Capacitors MLCC - SMD/SMT"},
		Packages:         []string{"0402"},
		LibraryTypes:     []ComponentType{ComponentTypeBase},
		AttributeFilters: []AttributeFilter{{Name: "Capacitance", Values: values}},
	})
	if err != nil {
		t.Fatalf("attribute filter search failed after %s: %v", time.Since(start), err)
	}
	t.Logf("attribute filter search: total %d, pages %d (%s)", resp.TotalCount, resp.Pages, time.Since(start))

	if resp.TotalCount == 0 || len(resp.Products) == 0 {
		t.Fatal("expected basic 0402 capacitors of 100nF or 1uF")
	}
	for _, p := range resp.Products {
		capacitance := ""
		for _, attr := range p.Attributes {
			if attr.Name == "Capacitance" {
				capacitance = attr.Value
			}
		}
		if capacitance != values[0] && capacitance != values[1] {
			t.Errorf("%s has capacitance %q, want one of %v", p.ComponentCode, capacitance, values)
		}
		if parent, leaf := p.Category(); parent != "Capacitors" || leaf != "Multilayer Ceramic Capacitors MLCC - SMD/SMT" {
			t.Errorf("%s has category (%q, %q)", p.ComponentCode, parent, leaf)
		}
	}
}
