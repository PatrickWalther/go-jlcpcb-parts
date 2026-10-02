package jlcpcb

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"sync"
	"testing"
	"time"
)

// The facet fixtures come from live filterComponentAttribute responses of
// 2026-10-02. The brand, package and category lists are trimmed, and some
// attributes are removed.
//
//   - facets_view_similar_C1525.json: the "View Similar" body of C1525
//     (request_facets_view_similar.json).
//   - facets_now_condition_voltage.json: the same body with nowCondition
//     "Voltage Rating".
//   - facets_ldo_C6186.json: SOT-223 LDOs with 3.3V and 1A.
//   - facets_resistor_10k.json: 0402 chip resistors with 10kΩ.
//   - facets_keyword_parent_params.json: a keyword query. The server sends
//     the attributes in parentParamList and parentParamRangeList.
//   - facets_overview.json: the whole library. The server sends the
//     categories only in productTypeList.

// viewSimilarFacetRequest is the facet request of the "View Similar" link
// of C1525.
func viewSimilarFacetRequest() *FacetRequest {
	return &FacetRequest{
		ParentID: 2,
		LeafID:   2929,
		Packages: []string{"0402"},
		Attributes: []AttributeFilter{
			{Name: "Voltage Rating", Values: []string{"16V"}},
			{Name: "Capacitance", Values: []string{"100nF"}},
			{Name: "Temperature Coefficient", Values: []string{"X7R"}},
		},
	}
}

func facetsFromFixture(t *testing.T, name string, req *FacetRequest) *Facets {
	t.Helper()
	rec := fixedAPIRecorder(t, loadFixture(t, name))
	facets, err := rec.client().Search.Facets(context.Background(), req)
	if err != nil {
		t.Fatalf("facets from %s failed: %v", name, err)
	}
	return facets
}

func TestFacetRequestViewSimilarBody(t *testing.T) {
	rec := fixedAPIRecorder(t, loadFixture(t, "facets_view_similar_C1525.json"))
	if _, err := rec.client().Search.Facets(context.Background(), viewSimilarFacetRequest()); err != nil {
		t.Fatalf("facets failed: %v", err)
	}

	requests := rec.all()
	if len(requests) != 1 {
		t.Fatalf("server got %d requests, want 1", len(requests))
	}
	req := requests[0]
	if req.Method != http.MethodPost || req.Path != filterComponentAttributePath || req.ContentType != "application/json" {
		t.Errorf("request = %s %s (%s), want POST %s", req.Method, req.Path, req.ContentType, filterComponentAttributePath)
	}
	// The body is byte for byte the body of the live request.
	want := bytes.TrimSpace(loadFixture(t, "request_facets_view_similar.json"))
	if !bytes.Equal(req.Body, want) {
		t.Errorf("body mismatch\n got: %s\nwant: %s", req.Body, want)
	}
}

func TestFacetRequestGoldenBodies(t *testing.T) {
	// baseBody is the body of an empty FacetRequest.
	baseQuery := func() map[string]interface{} {
		return map[string]interface{}{
			"componentBrandList":         []interface{}{},
			"componentSpecificationList": []interface{}{},
			"componentTypeIdList":        []interface{}{},
			"orderLibraryTypeList":       []interface{}{},
			"packageTypeList":            []interface{}{},
			"productTypeIdList":          []interface{}{},
		}
	}
	tests := []struct {
		name string
		req  FacetRequest
		// base are the baseQueryDto keys that differ from an empty request.
		base map[string]interface{}
		// top are the top-level keys that differ from an empty request.
		top map[string]interface{}
	}{
		{
			name: "empty request queries the whole library",
		},
		{
			name: "parent category alone uses catalog level 1",
			req:  FacetRequest{ParentID: 2},
			base: map[string]interface{}{"productTypeIdList": []interface{}{2.0}},
			top:  map[string]interface{}{"catalogLevel": 1.0},
		},
		{
			name: "leaf category uses catalog level 2",
			req:  FacetRequest{ParentID: 2, LeafID: 2929},
			base: map[string]interface{}{"productTypeIdList": []interface{}{2.0}, "componentTypeIdList": []interface{}{2929.0}},
			top:  map[string]interface{}{"catalogLevel": 2.0},
		},
		{
			name: "keyword",
			req:  FacetRequest{Keyword: " AMS1117-3.3 "},
			base: map[string]interface{}{"filterType": 1.0, "keyword": "AMS1117-3.3"},
			top:  map[string]interface{}{"catalogLevel": 1.0, "queryString": "AMS1117-3.3"},
		},
		{
			name: "library types go to orderLibraryTypeList and componentLibTypes",
			req:  FacetRequest{LibraryTypes: []ComponentType{"BASE", ComponentTypeBase, ""}, IncludePreferred: true},
			base: map[string]interface{}{
				"orderLibraryTypeList":   []interface{}{"base"},
				"componentLibTypes":      []interface{}{"base"},
				"preferredComponentFlag": true,
			},
		},
		{
			name: "availability, PCBA type and datasheet",
			req:  FacetRequest{PresaleTypes: []PresaleType{PresaleTypeStock, PresaleTypeBuy}, PCBA: PCBAFilterEconomic, HasDatasheet: true},
			base: map[string]interface{}{"presaleTypes": []interface{}{"stock", "buy"}, "pcbAType": 1.0, "dateSheet": true},
		},
		{
			name: "packages and brands are trimmed without duplicates",
			req:  FacetRequest{Packages: []string{" 0402", "0402", "", "0603"}, Brands: []string{"Hunan Xiangyee in S&T", " "}},
			base: map[string]interface{}{
				"componentSpecificationList": []interface{}{"0402", "0603"},
				"componentBrandList":         []interface{}{"Hunan Xiangyee in S&T"},
			},
		},
		{
			name: "attributes are grouped by name",
			req: FacetRequest{Attributes: []AttributeFilter{
				{Name: "Voltage Rating", Values: []string{"16V", "25V"}},
				{Name: "Capacitance", Values: []string{"100nF", ""}},
				{Name: "Voltage Rating", Values: []string{"25V", "50V"}},
				{Name: " ", Values: []string{"1"}},
			}},
			top: map[string]interface{}{"paramList": []interface{}{
				map[string]interface{}{"paramName": "Voltage Rating", "paramValueList": []interface{}{"16V", "25V", "50V"}},
				map[string]interface{}{"paramName": "Capacitance", "paramValueList": []interface{}{"100nF"}},
			}},
		},
		{
			name: "facet for a row",
			req:  FacetRequest{FacetFor: " " + FacetForLibraryType + " "},
			top:  map[string]interface{}{"nowCondition": "partsType"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := fixedAPIRecorder(t, loadFixture(t, "facets_view_similar_C1525.json"))
			req := tt.req
			if _, err := rec.client().Search.Facets(context.Background(), &req); err != nil {
				t.Fatalf("facets failed: %v", err)
			}

			base := baseQuery()
			for k, v := range tt.base {
				base[k] = v
			}
			want := map[string]interface{}{
				"baseQueryDto": base,
				"catalogLevel": 0.0,
				"nowCondition": "",
				"paramList":    []interface{}{},
			}
			for k, v := range tt.top {
				want[k] = v
			}
			assertJSONBody(t, rec.all()[0].Body, want)
		})
	}
}

func TestFacetsDecodeViewSimilar(t *testing.T) {
	facets := facetsFromFixture(t, "facets_view_similar_C1525.json", viewSimilarFacetRequest())

	if facets.Total != 151 || facets.CountsAreFlags {
		t.Errorf("total %d, flags %v, want 151 and false", facets.Total, facets.CountsAreFlags)
	}
	wantCounts := LibraryCounts{Basic: 1, Preferred: 0, Extended: 150, Economic: 151, Standard: 151, Datasheet: 143, Photo: 75}
	if facets.Counts != wantCounts {
		t.Errorf("counts = %+v, want %+v", facets.Counts, wantCounts)
	}
	wantPresale := map[PresaleType]int{PresaleTypeStock: 60, PresaleTypeBuy: 71, PresaleTypePost: 20}
	if !reflect.DeepEqual(facets.Presale, wantPresale) {
		t.Errorf("presale = %v, want %v", facets.Presale, wantPresale)
	}
	wantCategories := []CategoryCount{{
		ID: 2, Name: "Capacitors", Level: 1, Count: 151,
		Children: []CategoryCount{{ID: 2929, ParentID: 2, Name: mlccLeaf, Level: 2, Count: 151}},
	}}
	if !reflect.DeepEqual(facets.Categories, wantCategories) {
		t.Errorf("categories = %+v, want %+v", facets.Categories, wantCategories)
	}
	if !reflect.DeepEqual(facets.Packages, []Bucket{{Value: "0402", Name: "0402", Count: 151}}) {
		t.Errorf("packages = %+v", facets.Packages)
	}
	if len(facets.Brands) != 5 || facets.Brands[3] != (Bucket{Value: "CCTC", Name: "CCTC", Count: 4}) {
		t.Errorf("brands = %+v", facets.Brands)
	}

	var names []string
	for _, p := range facets.Params {
		names = append(names, p.Name)
	}
	wantNames := []string{"Capacitance", "Operating Temperature", "Temperature Coefficient", "Tolerance", "Voltage Rating"}
	if !reflect.DeepEqual(names, wantNames) {
		t.Errorf("params = %v, want %v", names, wantNames)
	}

	capacitance, ok := facets.Param("capacitance")
	if !ok {
		t.Fatal("no Capacitance facet")
	}
	if !capacitance.Range || capacitance.UnitScale["uF"] != 1e6 || capacitance.UnitScale["pF"] != 1 {
		t.Errorf("capacitance range %v, unit scale %v", capacitance.Range, capacitance.UnitScale)
	}
	if !reflect.DeepEqual(capacitance.Units, []string{"fF", "pF", "nF", "uF", "mF", "F"}) {
		t.Errorf("capacitance units = %v", capacitance.Units)
	}
	if want := []ParamValue{{Value: "100nF", Count: 151, DocCount: 151, Norm: 100000}}; !reflect.DeepEqual(capacitance.Values, want) {
		t.Errorf("capacitance values = %+v, want %+v", capacitance.Values, want)
	}

	// The server counts a part twice for a symmetric "±" value.
	tolerance, _ := facets.Param("Tolerance")
	if want := (ParamValue{Value: "±10%", Count: 85, DocCount: 170, IntervalStart: -10, IntervalEnd: 10}); tolerance.Values[0] != want {
		t.Errorf("tolerance value = %+v, want %+v", tolerance.Values[0], want)
	}
	sum, docSum := 0, 0
	for _, v := range tolerance.Values {
		sum += v.Count
		docSum += v.DocCount
	}
	// 2 of the 151 parts have no tolerance.
	if sum != 149 || docSum != 2*sum {
		t.Errorf("tolerance counts sum to %d and %d, want 149 and 298", sum, docSum)
	}

	coefficient, _ := facets.Param("Temperature Coefficient")
	if coefficient.Range || coefficient.UnitScale != nil || len(coefficient.Units) != 0 {
		t.Errorf("temperature coefficient = %+v, want a text facet", coefficient)
	}
	if _, ok := facets.Param("Resistance"); ok {
		t.Error("Param found an attribute that is not in the answer")
	}
}

func TestFacetsCountsAreFlags(t *testing.T) {
	req := viewSimilarFacetRequest()
	req.FacetFor = "Voltage Rating"
	facets := facetsFromFixture(t, "facets_now_condition_voltage.json", req)

	if !facets.CountsAreFlags {
		t.Error("CountsAreFlags = false, want true for a request with FacetFor")
	}
	// Only the total stays filtered. The voltage facet shows every voltage.
	if facets.Total != 151 || facets.Counts.Extended != 1 || facets.Counts.Basic != 1 {
		t.Errorf("total %d, counts %+v", facets.Total, facets.Counts)
	}
	voltage, _ := facets.Param("Voltage Rating")
	if len(voltage.Values) != 6 || voltage.Values[2].Value != "16V" || voltage.Values[2].Count != 151 {
		t.Errorf("voltage values = %+v", voltage.Values)
	}
}

func TestFacetsKeywordParentParams(t *testing.T) {
	facets := facetsFromFixture(t, "facets_keyword_parent_params.json", &FacetRequest{
		Keyword: "100nF 0402",
		Brands:  []string{"Hunan Xiangyee in S&T"},
	})

	if facets.Total != 1 || len(facets.Params) != 3 {
		t.Fatalf("total %d, %d params, want 1 and 3", facets.Total, len(facets.Params))
	}
	tolerance, ok := facets.Param("Tolerance")
	if !ok || len(tolerance.Values) != 1 || tolerance.Values[0].Count != 1 || tolerance.UnitScale["%"] != 1 {
		t.Errorf("tolerance = %+v", tolerance)
	}
	if got := tolerance.Canonical("±20%"); !reflect.DeepEqual(got, []string{"±20%"}) {
		t.Errorf("Canonical(±20%%) = %v", got)
	}
}

func TestFacetsOverviewCategoriesFromList(t *testing.T) {
	facets := facetsFromFixture(t, "facets_overview.json", &FacetRequest{})

	if facets.Counts.Basic != 351 || facets.Counts.Preferred != 1235 || facets.Counts.MechanicalAssembly != 79962 {
		t.Errorf("counts = %+v", facets.Counts)
	}
	// The server sends a bucket without a class. Presale does not keep it.
	if len(facets.Presale) != 3 || facets.Presale[PresaleTypeStock] != 740959 {
		t.Errorf("presale = %v", facets.Presale)
	}
	if len(facets.Categories) != 3 || !reflect.DeepEqual(facets.Categories[0], CategoryCount{ID: 23, Name: "Amplifiers/Comparators", Level: 1, Count: 37526}) {
		t.Errorf("categories = %+v", facets.Categories)
	}
	if len(facets.Params) != 0 {
		t.Errorf("params = %+v, want none", facets.Params)
	}
}

func TestFacetsInvalidRequestSendsNoRequest(t *testing.T) {
	rec := fixedAPIRecorder(t, loadFixture(t, "facets_view_similar_C1525.json"))
	client := rec.client()
	for name, req := range map[string]*FacetRequest{
		"nil":                nil,
		"negative parent id": {ParentID: -1},
		"negative leaf id":   {ParentID: 2, LeafID: -2929},
	} {
		if _, err := client.Search.Facets(context.Background(), req); !errors.Is(err, ErrInvalidRequest) {
			t.Errorf("%s: error = %v, want ErrInvalidRequest", name, err)
		}
	}
	if n := len(rec.all()); n != 0 {
		t.Errorf("server got %d requests, want 0", n)
	}
}

func TestFacetsRejected(t *testing.T) {
	rec := fixedAPIRecorder(t, loadFixture(t, "search_v2_rejected.json"))
	if _, err := rec.client().Search.Facets(context.Background(), viewSimilarFacetRequest()); !errors.Is(err, ErrRejected) {
		t.Fatalf("error = %v, want ErrRejected", err)
	}
}

func TestFacetsNullData(t *testing.T) {
	rec := fixedAPIRecorder(t, []byte(`{"code":200,"data":null}`))
	if facets, err := rec.client().Search.Facets(context.Background(), viewSimilarFacetRequest()); err == nil {
		t.Fatalf("facets = %+v, want an error", facets)
	}
}

// ttlCache is a MemoryCache that records the TTL of each key.
type ttlCache struct {
	*MemoryCache
	mu   sync.Mutex
	ttls map[string]time.Duration
}

func newTTLCache() *ttlCache {
	return &ttlCache{MemoryCache: NewMemoryCache(), ttls: make(map[string]time.Duration)}
}

func (c *ttlCache) Set(key string, value []byte, ttl time.Duration) {
	c.mu.Lock()
	c.ttls[key] = ttl
	c.mu.Unlock()
	c.MemoryCache.Set(key, value, ttl)
}

func (c *ttlCache) only(t *testing.T) (string, time.Duration) {
	t.Helper()
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.ttls) != 1 {
		t.Fatalf("cache has %d keys, want 1: %v", len(c.ttls), c.ttls)
	}
	for key, ttl := range c.ttls {
		return key, ttl
	}
	return "", 0
}

func TestFacetsCache(t *testing.T) {
	rec := fixedAPIRecorder(t, loadFixture(t, "facets_view_similar_C1525.json"))
	cache := newTTLCache()
	client := rec.client(WithCache(cache), WithCacheConfig(DefaultCacheConfig()))
	ctx := context.Background()

	first, err := client.Search.Facets(ctx, viewSimilarFacetRequest())
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}
	second, err := client.Search.Facets(ctx, viewSimilarFacetRequest())
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}
	if n := len(rec.all()); n != 1 {
		t.Fatalf("server got %d requests, want 1", n)
	}
	if !reflect.DeepEqual(first, second) {
		t.Errorf("cached facets differ\nfirst:  %+v\nsecond: %+v", first, second)
	}
	if _, ttl := cache.only(t); ttl != 15*time.Minute {
		t.Errorf("facets TTL = %v, want 15m", ttl)
	}

	// Each request field is part of the cache key.
	other := viewSimilarFacetRequest()
	other.FacetFor = "Capacitance"
	if _, err := client.Search.Facets(ctx, other); err != nil {
		t.Fatalf("third call failed: %v", err)
	}
	if n := len(rec.all()); n != 2 {
		t.Errorf("server got %d requests, want 2", n)
	}
}

func TestFacetsCacheTTLOption(t *testing.T) {
	rec := fixedAPIRecorder(t, loadFixture(t, "facets_view_similar_C1525.json"))
	cache := newTTLCache()
	cfg := CacheConfig{Enabled: true, FacetsTTL: time.Minute}
	if _, err := rec.client(WithCache(cache), WithCacheConfig(cfg)).Search.Facets(context.Background(), viewSimilarFacetRequest()); err != nil {
		t.Fatalf("facets failed: %v", err)
	}
	if _, ttl := cache.only(t); ttl != time.Minute {
		t.Errorf("facets TTL = %v, want 1m", ttl)
	}
}

func TestParamFacetCanonical(t *testing.T) {
	similar := facetsFromFixture(t, "facets_view_similar_C1525.json", viewSimilarFacetRequest())
	ldo := facetsFromFixture(t, "facets_ldo_C6186.json", &FacetRequest{ParentID: 9, LeafID: 2956})
	resistor := facetsFromFixture(t, "facets_resistor_10k.json", &FacetRequest{ParentID: 1, LeafID: 2980})

	param := func(f *Facets, name string) ParamFacet {
		t.Helper()
		p, ok := f.Param(name)
		if !ok {
			t.Fatalf("no facet %q", name)
		}
		return p
	}
	allTemperatures := []string{"-40℃~+125℃", "-40℃~+125℃@(Ta)", "-40℃~+125℃@(Tj)", "-40℃~+125℃@(TJ)"}

	tests := []struct {
		facet ParamFacet
		input string
		want  []string
	}{
		{param(similar, "Capacitance"), "0.1uF", []string{"100nF"}},
		{param(similar, "Capacitance"), "0.1 µF", []string{"100nF"}},
		{param(similar, "Capacitance"), "100000pF", []string{"100nF"}},
		{param(similar, "Capacitance"), "100n", []string{"100nF"}},
		{param(similar, "Capacitance"), "100NF", []string{"100nF"}},
		{param(similar, "Capacitance"), "100nf", []string{"100nF"}},
		{param(similar, "Capacitance"), "1uF", nil},
		{param(similar, "Capacitance"), "0.1xF", nil},
		{param(similar, "Capacitance"), "", nil},
		{param(similar, "Voltage Rating"), "16", []string{"16V"}},
		{param(similar, "Voltage Rating"), "16000mV", []string{"16V"}},
		{param(similar, "Temperature Coefficient"), "x7r", []string{"X7R"}},
		{param(similar, "Temperature Coefficient"), "0", nil},
		{param(similar, "Tolerance"), "±10%", []string{"±10%"}},
		{param(similar, "Tolerance"), "+/-10 %", []string{"±10%"}},
		{param(similar, "Tolerance"), "±100‰", []string{"±10%"}},
		{param(similar, "Tolerance"), "10%", nil},
		{param(ldo, "Output Voltage"), "3300mV", []string{"3.3V"}},
		{param(ldo, "Output Voltage"), "3.3", []string{"3.3V"}},
		{param(ldo, "Output Current"), "1000mA", []string{"1A"}},
		{param(ldo, "Number of Outputs"), "1", []string{"1"}},
		{param(ldo, "Number of Outputs"), "-", []string{"-"}},
		{param(ldo, "Operating Temperature"), "-40°C~+125°C", allTemperatures},
		{param(ldo, "Operating Temperature"), "-40~125℃", allTemperatures},
		{param(ldo, "Operating Temperature"), "-40℃~+125℃@(Tj)", allTemperatures},
		{param(ldo, "Operating Temperature"), "-40℃", nil},
		{param(resistor, "Resistance"), "10k", []string{"10kΩ"}},
		{param(resistor, "Resistance"), "10K", []string{"10kΩ"}},
		{param(resistor, "Resistance"), "10 kOhm", []string{"10kΩ"}},
		{param(resistor, "Resistance"), "0.01M", []string{"10kΩ"}},
		{param(resistor, "Resistance"), "10000", []string{"10kΩ"}},
		{param(resistor, "Resistance"), "10m", nil},
		{param(resistor, "Tolerance"), "±1%", []string{"±1%"}},
		{param(resistor, "Temperature Coefficient"), "±100ppm/°C", []string{"±100ppm/℃"}},
		{param(resistor, "Temperature Coefficient"), "±100ppm/K", []string{"±100ppm/℃"}},
		{param(resistor, "Temperature Coefficient"), "±0.1ppb/℃", nil},
	}
	for _, tt := range tests {
		if got := tt.facet.Canonical(tt.input); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s.Canonical(%q) = %q, want %q", tt.facet.Name, tt.input, got, tt.want)
		}
	}
}

func TestParamFacetCanonicalHandBuilt(t *testing.T) {
	frequency := ParamFacet{
		Name:      "Frequency",
		Range:     true,
		UnitScale: map[string]float64{"Hz": 1, "kHz": 1e3, "MHz": 1e6, "GHz": 1e9},
		Values: []ParamValue{
			{Value: "0Hz"},
			{Value: "100kHz", Norm: 1e5},
			{Value: "1.8MHz", Norm: 1.8e6},
			{Value: "-"},
		},
	}
	tests := []struct {
		input string
		want  []string
	}{
		{"1.8M", []string{"1.8MHz"}},
		{"1.8m", nil},    // milli does not fold into mega
		{"1.8mhz", nil},  // "mhz" is millihertz, not megahertz
		{"1.8 mHz", nil}, // millihertz
		{"1.8MHZ", []string{"1.8MHz"}},
		{"1.8 MHz", []string{"1.8MHz"}},
		{"1800kHz", []string{"1.8MHz"}},
		{"0.1MHz", []string{"100kHz"}},
		{"0", []string{"0Hz"}},
		{"0kHz", []string{"0Hz"}},
		{"-", []string{"-"}},
		{"fast", nil},
	}
	for _, tt := range tests {
		if got := frequency.Canonical(tt.input); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("Canonical(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// TestParamFacetCanonicalMilliMega uses a resistance facet with milliohm
// and megaohm values, as the full chip resistor leaf has. A milli value must
// never match a mega value.
func TestParamFacetCanonicalMilliMega(t *testing.T) {
	resistance := ParamFacet{
		Name:      "Resistance",
		Range:     true,
		UnitScale: map[string]float64{"mΩ": 0.001, "Ω": 1, "kΩ": 1e3, "MΩ": 1e6},
		Values: []ParamValue{
			{Value: "1mΩ", Norm: 0.001},
			{Value: "100mΩ", Norm: 0.1},
			{Value: "1MΩ", Norm: 1e6},
			{Value: "100MΩ", Norm: 1e8},
		},
	}
	// noScale has the same values, but the server sent no unit factors.
	// Canonical then cannot read the input as a number and compares text.
	noScale := resistance
	noScale.UnitScale = nil

	tests := []struct {
		input string
		want  []string
	}{
		{"1mΩ", []string{"1mΩ"}},
		{"1MΩ", []string{"1MΩ"}},
		{"100mΩ", []string{"100mΩ"}},
		{"100MΩ", []string{"100MΩ"}},
		{"1 mOhm", []string{"1mΩ"}},
		{"1 MOhm", []string{"1MΩ"}},
		{"0.1", []string{"100mΩ"}},
		{"1000000", []string{"1MΩ"}},
		{"1mω", []string{"1mΩ"}},
		{"1Mω", []string{"1MΩ"}},
	}
	for _, tt := range tests {
		if got := resistance.Canonical(tt.input); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("Canonical(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}

	textTests := []struct {
		input string
		want  []string
	}{
		{"1mΩ", []string{"1mΩ"}},
		{"1MΩ", []string{"1MΩ"}},
		{"1mω", []string{"1mΩ"}},
		{"100MΩ", []string{"100MΩ"}},
		{"100mω", []string{"100mΩ"}},
	}
	for _, tt := range textTests {
		if got := noScale.Canonical(tt.input); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("without unit factors: Canonical(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestEqualFoldText(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"X7R", "x7r", true},
		{"SMD", "smd", true},
		{"Automotive", "AUTOMOTIVE", true},
		{"100nF", "100NF", true},
		{"1mΩ", "1MΩ", false},
		{"1.8MHz", "1.8mhz", false},
		{"±10mV", "±10MV", false},
		{"+/-10mV", "+/-10MV", false},
		{"-40℃~+125℃", "-40℃~+125℃", true},
		{"100nF", "100pF", false},
	}
	for _, tt := range tests {
		if got := equalFoldText(tt.a, tt.b); got != tt.want {
			t.Errorf("equalFoldText(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestFacetsJSONRoundTrip(t *testing.T) {
	facets := facetsFromFixture(t, "facets_view_similar_C1525.json", viewSimilarFacetRequest())
	data, err := json.Marshal(facets)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back Facets
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reflect.DeepEqual(&back, facets) {
		t.Errorf("round trip differs\n got: %+v\nwant: %+v", back, *facets)
	}
}
