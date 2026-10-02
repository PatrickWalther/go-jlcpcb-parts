//go:build integration
// +build integration

package jlcpcb

import (
	"context"
	"errors"
	"slices"
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

// TestIntegrationDetailRouteA checks the exact part detail: C1525 has the
// part id 1877, and an unknown code gives ErrNotFound.
func TestIntegrationDetailRouteA(t *testing.T) {
	client := contractClient()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	start := time.Now()
	detail, err := client.Product.Detail(ctx, "c1525")
	if err != nil {
		t.Fatalf("detail failed after %s: %v", time.Since(start), err)
	}
	t.Logf("detail C1525: id %d, %d price tiers (%s)", detail.LCSCComponentID, len(detail.Prices), time.Since(start))

	if detail.ComponentCode != "C1525" || detail.LCSCComponentID != 1877 {
		t.Errorf("detail = %q id %d, want C1525 id 1877", detail.ComponentCode, detail.LCSCComponentID)
	}
	if parent, _ := detail.Category(); parent != "Capacitors" {
		t.Errorf("parent category = %q, want Capacitors", parent)
	}
	if len(detail.Prices) == 0 {
		t.Error("expected a price ladder")
	}

	if _, err := client.Product.Detail(ctx, "C999999999"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown code error = %v, want ErrNotFound", err)
	}
}

// TestIntegrationDetailsByIDsCompare checks the batch part detail: the id
// 1877 returns C1525 with a pre-order ladder and the URL suffix.
func TestIntegrationDetailsByIDsCompare(t *testing.T) {
	client := contractClient()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	start := time.Now()
	details, err := client.Product.DetailsByIDs(ctx, []int64{1877})
	if err != nil {
		t.Fatalf("batch detail failed after %s: %v", time.Since(start), err)
	}
	detail := details[1877]
	if detail == nil {
		t.Fatalf("batch detail has no record for id 1877: %v", details)
	}
	t.Logf("batch detail 1877: %s, %d buy tiers, URL suffix %q (%s)",
		detail.ComponentCode, len(detail.BuyPrices), detail.URLSuffix, time.Since(start))

	if detail.ComponentCode != "C1525" {
		t.Errorf("component code = %q, want C1525", detail.ComponentCode)
	}
	if len(detail.BuyPrices) == 0 {
		t.Error("expected a pre-order ladder (buyPrices)")
	}
	sorted := func(breaks []PriceBreak) bool {
		return slices.IsSortedFunc(breaks, func(a, b PriceBreak) int { return a.StartNumber - b.StartNumber })
	}
	if !sorted(detail.Prices) || !sorted(detail.BuyPrices) {
		t.Errorf("ladders are not sorted: %+v, %+v", detail.Prices, detail.BuyPrices)
	}
	if detail.URLSuffix == "" {
		t.Error("expected the URL suffix of the batch response")
	}
	product := detail.Product()
	if parent, leaf := product.Category(); parent != "Capacitors" || leaf != "Multilayer Ceramic Capacitors MLCC - SMD/SMT" {
		t.Errorf("Product().Category() = (%q, %q)", parent, leaf)
	}
	if product.ComponentID != 1877 || product.LibraryType() != LibraryTypeBasic {
		t.Errorf("Product() = id %d, library type %q, want 1877 and basic", product.ComponentID, product.LibraryType())
	}
}

// TestIntegrationAssemblyCalculators checks the J6 reference row: the
// attrition calculator answers 190 and the order quantity calculator
// answers 100190. The local estimate gives the same values.
func TestIntegrationAssemblyCalculators(t *testing.T) {
	client := contractClient()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	rows := []PlacementRow{{
		Side:                AssemblySideSingle,
		Boards:              1000,
		PerBoard:            100,
		LossNumber:          10,
		LeastPatchNumber:    20,
		EncapsulationNumber: 10000,
	}}

	start := time.Now()
	attrition, err := client.Assembly.Attrition(ctx, rows)
	if err != nil {
		t.Fatalf("attrition failed after %s: %v", time.Since(start), err)
	}
	t.Logf("attrition: %v (%s)", attrition, time.Since(start))
	if len(attrition) != 1 || attrition[0] != 190 {
		t.Errorf("attrition = %v, want [190]", attrition)
	}
	if estimate := EstimateAttrition(rows[0], DefaultWastageCoefficient); len(attrition) == 1 && estimate != attrition[0] {
		t.Errorf("EstimateAttrition = %d, calculator = %d", estimate, attrition[0])
	}

	start = time.Now()
	qty, err := client.Assembly.OrderQuantities(ctx, rows)
	if err != nil {
		t.Fatalf("order quantities failed after %s: %v", time.Since(start), err)
	}
	t.Logf("order quantities: %v (%s)", qty, time.Since(start))
	if len(qty) != 1 || qty[0] != 100190 {
		t.Errorf("order quantities = %v, want [100190]", qty)
	}
}

// TestIntegrationFacetsViewSimilar checks the facet query of the C1525
// "View Similar" link: the server counts the basic part C1525, and the
// capacitance facet maps "0.1uF" to "100nF".
func TestIntegrationFacetsViewSimilar(t *testing.T) {
	client := contractClient()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	start := time.Now()
	facets, err := client.Search.Facets(ctx, &FacetRequest{
		ParentID: 2,
		LeafID:   2929,
		Packages: []string{"0402"},
		Attributes: []AttributeFilter{
			{Name: "Voltage Rating", Values: []string{"16V"}},
			{Name: "Capacitance", Values: []string{"100nF"}},
			{Name: "Temperature Coefficient", Values: []string{"X7R"}},
		},
	})
	if err != nil {
		t.Fatalf("facets failed after %s: %v", time.Since(start), err)
	}
	t.Logf("facets: total %d, counts %+v, presale %v, %d params (%s)",
		facets.Total, facets.Counts, facets.Presale, len(facets.Params), time.Since(start))

	if facets.Total <= 0 || facets.Counts.Basic < 1 {
		t.Errorf("total %d, basic %d, want both above 0", facets.Total, facets.Counts.Basic)
	}
	capacitance, ok := facets.Param("Capacitance")
	if !ok {
		t.Fatal("no Capacitance facet")
	}
	if got := capacitance.Canonical("0.1uF"); !slices.Contains(got, "100nF") {
		t.Errorf("Canonical(0.1uF) = %v, want 100nF", got)
	}
}

// TestIntegrationCategoryInfo checks the category name lookup: 2929 is a
// leaf of "Capacitors", and an unknown id gives ErrNotFound.
func TestIntegrationCategoryInfo(t *testing.T) {
	client := contractClient()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	start := time.Now()
	info, err := client.Category.Info(ctx, 2929)
	if err != nil {
		t.Fatalf("category info failed after %s: %v", time.Since(start), err)
	}
	t.Logf("category 2929: %+v (%s)", *info, time.Since(start))
	if info.ParentName != "Capacitors" || info.ParentID != 2 || info.LeafID != 2929 || info.LeafName != "Multilayer Ceramic Capacitors MLCC - SMD/SMT" {
		t.Errorf("info = %+v", *info)
	}

	if _, err := client.Category.Info(ctx, 999999999); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown id error = %v, want ErrNotFound", err)
	}
}
