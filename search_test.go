package jlcpcb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
)

// mlccLeaf is the leaf category of surface-mount ceramic capacitors.
const mlccLeaf = "Multilayer Ceramic Capacitors MLCC - SMD/SMT"

// emptySearchResponse is a valid search response without rows.
const emptySearchResponse = `{"code":200,"message":null,"data":{"componentPageInfo":{"total":0,"pageSize":50,"pageNum":1,"pages":0,"list":[]}}}`

// bodyRecorder is a test server that records the raw body of each request
// and answers with a fixed response.
type bodyRecorder struct {
	mu     sync.Mutex
	bodies [][]byte
	server *httptest.Server
}

func newBodyRecorder(t *testing.T, response []byte) *bodyRecorder {
	t.Helper()
	rec := &bodyRecorder{}
	rec.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}
		rec.mu.Lock()
		rec.bodies = append(rec.bodies, body)
		rec.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(response)
	}))
	t.Cleanup(rec.server.Close)
	return rec
}

func (rec *bodyRecorder) client() *Client {
	return NewClient(WithBaseURL(rec.server.URL), WithHTTPClient(rec.server.Client()), WithoutCache())
}

func (rec *bodyRecorder) last(t *testing.T) []byte {
	t.Helper()
	rec.mu.Lock()
	defer rec.mu.Unlock()
	if len(rec.bodies) == 0 {
		t.Fatal("the server got no request")
	}
	return rec.bodies[len(rec.bodies)-1]
}

func (rec *bodyRecorder) count() int {
	rec.mu.Lock()
	defer rec.mu.Unlock()
	return len(rec.bodies)
}

func decodeJSONObject(t *testing.T, data []byte) map[string]interface{} {
	t.Helper()
	var out map[string]interface{}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("decode JSON %s: %v", data, err)
	}
	return out
}

// assertJSONBody compares a request body with the expected JSON object. The
// comparison ignores key order and white space, but not extra or missing keys.
func assertJSONBody(t *testing.T, got []byte, want map[string]interface{}) {
	t.Helper()
	gotObj := decodeJSONObject(t, got)
	if !reflect.DeepEqual(gotObj, want) {
		wantJSON, _ := json.Marshal(want)
		t.Errorf("request body mismatch\n got: %s\nwant: %s", got, wantJSON)
	}
}

// keywordBody is the body of SearchRequest{Keyword: "LM358"}. It is the body
// that v1.3.0 sends for a keyword search without filters.
const keywordBody = `{"keyword":"LM358","currentPage":1,"pageSize":50,"presaleType":"","searchType":2,` +
	`"componentLibraryType":null,"componentAttributeList":[],"componentBrandList":[],` +
	`"componentSpecificationList":[],"paramList":[],"firstSortName":null,"secondSortName":null,` +
	`"searchSource":"search","stockFlag":false}`

func TestSearchRequestGoldenBodies(t *testing.T) {
	tests := []struct {
		name string
		req  SearchRequest
		// changes are the keys that differ from keywordBody. A null value
		// removes the key from the expected body.
		changes string
	}{
		{
			name:    "no options",
			req:     SearchRequest{},
			changes: `{}`,
		},
		{
			name:    "include preferred with base library",
			req:     SearchRequest{ComponentType: ComponentTypeBase, IncludePreferred: true},
			changes: `{"componentLibraryType":"base","preferredComponentFlag":true}`,
		},
		{
			name:    "include preferred alone",
			req:     SearchRequest{IncludePreferred: true},
			changes: `{"preferredComponentFlag":true}`,
		},
		{
			name:    "category",
			req:     SearchRequest{Category: Category{Parent: " Capacitors ", Leaf: mlccLeaf}},
			changes: `{"firstSortName":"Capacitors","secondSortName":"` + mlccLeaf + `"}`,
		},
		{
			name:    "deprecated category aliases",
			req:     SearchRequest{SortPrimary: "Resistors", SortSecondary: "Chip Resistor - Surface Mount"},
			changes: `{"firstSortName":"Resistors","secondSortName":"Chip Resistor - Surface Mount"}`,
		},
		{
			name: "category overrides the deprecated aliases",
			req: SearchRequest{
				Category:      Category{Parent: "Capacitors", Leaf: mlccLeaf},
				SortPrimary:   "Resistors",
				SortSecondary: "Chip Resistor - Surface Mount",
			},
			changes: `{"firstSortName":"Capacitors","secondSortName":"` + mlccLeaf + `"}`,
		},
		{
			name: "attribute filters",
			req: SearchRequest{AttributeFilters: []AttributeFilter{
				{Name: "Capacitance", Values: []string{"100nF", "1uF"}},
				{Name: "Voltage Rating", Values: []string{"25V"}},
			}},
			changes: `{"componentAttributeList":[{"Capacitance":["100nF","1uF"]},{"Voltage Rating":["25V"]}]}`,
		},
		{
			name: "deprecated attributes grouped by name",
			req: SearchRequest{Attributes: []FilterAttribute{
				{Name: "Capacitance", Value: "100nF"},
				{Name: "Voltage Rating", Value: "25V"},
				{Name: "Capacitance", Value: "1uF"},
			}},
			changes: `{"componentAttributeList":[{"Capacitance":["100nF","1uF"]},{"Voltage Rating":["25V"]}]}`,
		},
		{
			name: "attribute sources merged without empty or duplicate values",
			req: SearchRequest{
				Attributes: []FilterAttribute{
					{Name: "Capacitance", Value: "100nF"},
					{Name: "", Value: "ignored"},
					{Name: "Tolerance", Value: " "},
				},
				AttributeFilters: []AttributeFilter{
					{Name: " Capacitance ", Values: []string{"100nF", " 1uF ", ""}},
					{Name: "Voltage Rating"},
					{Name: "Temperature Coefficient", Values: []string{"X7R"}},
				},
			},
			changes: `{"componentAttributeList":[{"Capacitance":["100nF","1uF"]},{"Temperature Coefficient":["X7R"]}]}`,
		},
		{
			name:    "library types",
			req:     SearchRequest{LibraryTypes: []ComponentType{" BASE ", "", ComponentTypeBase, ComponentTypeExpand}},
			changes: `{"componentLibTypes":["base","expand"]}`,
		},
		{
			name:    "presale types",
			req:     SearchRequest{PresaleTypes: []PresaleType{PresaleTypeStock, "Buy", "", PresaleTypeStock}},
			changes: `{"presaleTypes":["stock","buy"]}`,
		},
		{
			name:    "stock only",
			req:     SearchRequest{StockOnly: true},
			changes: `{"stockFlag":true,"presaleType":"stock"}`,
		},
		{
			name:    "stock only with presale types",
			req:     SearchRequest{StockOnly: true, PresaleTypes: []PresaleType{PresaleTypeBuy}},
			changes: `{"stockFlag":true,"presaleTypes":["buy"]}`,
		},
		{
			name:    "stock only with presale type",
			req:     SearchRequest{StockOnly: true, PresaleType: PresaleTypePost},
			changes: `{"stockFlag":true,"presaleType":"post"}`,
		},
		{
			name:    "minimum stock",
			req:     SearchRequest{MinStock: 1000},
			changes: `{"startStockNumber":1000}`,
		},
		{
			name:    "negative minimum stock",
			req:     SearchRequest{MinStock: -5},
			changes: `{}`,
		},
		{
			name:    "economic PCBA",
			req:     SearchRequest{PCBA: PCBAFilterEconomic},
			changes: `{"pcbAType":1}`,
		},
		{
			name:    "standard PCBA",
			req:     SearchRequest{PCBA: PCBAFilterStandard},
			changes: `{"pcbAType":2}`,
		},
		{
			name:    "sort by price ascending",
			req:     SearchRequest{Sort: SortByPrice},
			changes: `{"sortMode":"PRICE_SORT","sortASC":"ASC"}`,
		},
		{
			name:    "sort by stock descending",
			req:     SearchRequest{Sort: SortByStock, SortDescending: true},
			changes: `{"sortMode":"STOCK_SORT","sortASC":"DESC"}`,
		},
		{
			name:    "sort by model",
			req:     SearchRequest{Sort: SortByModel},
			changes: `{"sortMode":"MODEL_SORT","sortASC":"ASC"}`,
		},
		{
			name:    "descending without a sort mode",
			req:     SearchRequest{SortDescending: true},
			changes: `{}`,
		},
		{
			name:    "has datasheet",
			req:     SearchRequest{HasDatasheet: true},
			changes: `{"dateSheet":true}`,
		},
		{
			name:    "category counts",
			req:     SearchRequest{CategoryCounts: true},
			changes: `{"searchType":3}`,
		},
		{
			name:    "page size below the cap",
			req:     SearchRequest{PageSize: 1000, Page: 3},
			changes: `{"pageSize":1000,"currentPage":3}`,
		},
		{
			name:    "page size above the cap",
			req:     SearchRequest{PageSize: 1001},
			changes: `{"pageSize":1000}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := newBodyRecorder(t, []byte(emptySearchResponse))
			req := tt.req
			req.Keyword = "LM358"
			if _, err := rec.client().Search.Keyword(context.Background(), &req); err != nil {
				t.Fatalf("search failed: %v", err)
			}

			want := decodeJSONObject(t, []byte(keywordBody))
			for key, value := range decodeJSONObject(t, []byte(tt.changes)) {
				if value == nil {
					delete(want, key)
					continue
				}
				want[key] = value
			}
			assertJSONBody(t, rec.last(t), want)
		})
	}
}

// TestSearchRequestViewSimilarBody checks the full body of the JLCPCB "View
// Similar" search. The golden body is the live request body that returned
// exactly C1525 and C307331.
func TestSearchRequestViewSimilarBody(t *testing.T) {
	rec := newBodyRecorder(t, loadFixture(t, "search_v2_view_similar.json"))
	resp, err := rec.client().Search.Query(context.Background(), &SearchRequest{
		PageSize:         25,
		Category:         Category{Parent: "Capacitors", Leaf: mlccLeaf},
		Packages:         []string{"0402"},
		AttributeFilters: []AttributeFilter{{Name: "Capacitance", Values: []string{"100nF"}}, {Name: "Temperature Coefficient", Values: []string{"X7R"}}},
		LibraryTypes:     []ComponentType{ComponentTypeBase},
		IncludePreferred: true,
		PresaleTypes:     []PresaleType{PresaleTypeStock},
		PCBA:             PCBAFilterEconomic,
		Sort:             SortByStock,
		SortDescending:   true,
		CategoryCounts:   true,
	})
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	assertJSONBody(t, rec.last(t), decodeJSONObject(t, loadFixture(t, "request_view_similar.json")))

	var codes []string
	for _, p := range resp.Products {
		codes = append(codes, p.ComponentCode)
	}
	if want := []string{"C1525", "C307331"}; !reflect.DeepEqual(codes, want) {
		t.Errorf("products = %v, want %v", codes, want)
	}
	if resp.TotalCount != 2 || resp.Pages != 1 || resp.PageSize != 25 || resp.Page != 1 {
		t.Errorf("page info = total %d, pages %d, size %d, page %d; want 2, 1, 25, 1",
			resp.TotalCount, resp.Pages, resp.PageSize, resp.Page)
	}

	wantCategories := []CategoryCount{{
		ID: 2, ParentID: 0, Name: "Capacitors", Level: 1, Count: 2,
		Children: []CategoryCount{{ID: 2929, ParentID: 2, Name: mlccLeaf, Level: 2, Count: 2}},
	}}
	if !reflect.DeepEqual(resp.Categories, wantCategories) {
		t.Errorf("categories = %+v, want %+v", resp.Categories, wantCategories)
	}
}

func TestSearchQueryWithoutKeyword(t *testing.T) {
	rec := newBodyRecorder(t, []byte(emptySearchResponse))
	client := rec.client()

	if _, err := client.Search.Query(context.Background(), &SearchRequest{
		Keyword:  "   ",
		Category: Category{Parent: "Capacitors", Leaf: mlccLeaf},
	}); err != nil {
		t.Fatalf("query failed: %v", err)
	}
	body := decodeJSONObject(t, rec.last(t))
	if value, ok := body["keyword"]; !ok || value != nil {
		t.Errorf("keyword = %v (present %v), want JSON null", value, ok)
	}

	if _, err := client.Search.Query(context.Background(), &SearchRequest{Keyword: " NE555 "}); err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if body := decodeJSONObject(t, rec.last(t)); body["keyword"] != "NE555" {
		t.Errorf("keyword = %v, want NE555", body["keyword"])
	}

	if _, err := client.Search.Query(context.Background(), nil); !errors.Is(err, ErrInvalidRequest) {
		t.Errorf("Query(nil) error = %v, want ErrInvalidRequest", err)
	}
	if _, err := client.Search.Keyword(context.Background(), &SearchRequest{Category: Category{Parent: "Capacitors"}}); !errors.Is(err, ErrInvalidRequest) {
		t.Errorf("Keyword without a keyword: error = %v, want ErrInvalidRequest", err)
	}
	if got := rec.count(); got != 2 {
		t.Errorf("server got %d requests, want 2", got)
	}
}

func TestSearchRejectedRequest(t *testing.T) {
	rec := newBodyRecorder(t, loadFixture(t, "search_v2_rejected.json"))
	_, err := rec.client().Search.Keyword(context.Background(), &SearchRequest{Keyword: "LM358"})
	if !errors.Is(err, ErrRejected) {
		t.Fatalf("error = %v, want ErrRejected", err)
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Code != 101 {
		t.Fatalf("error = %#v, want an APIError with code 101", err)
	}
	if errors.Is(err, ErrInvalidRequest) || errors.Is(err, ErrServer) {
		t.Errorf("code 101 must map only to ErrRejected, got %v", err)
	}
	if got := rec.count(); got != 1 {
		t.Errorf("server got %d requests, want 1 (no retry for code 101)", got)
	}
}

func TestSearchPastLastPage(t *testing.T) {
	rec := newBodyRecorder(t, loadFixture(t, "search_v2_past_end.json"))
	resp, err := rec.client().Search.Query(context.Background(), &SearchRequest{Keyword: "100nF 0402", Page: 161, PageSize: 100})
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if resp.Products != nil || resp.TotalCount != 0 || resp.Pages != 0 || resp.Categories != nil {
		t.Errorf("past-end page = %+v, want no products, total 0 and pages 0", resp)
	}
}

func TestSearchCacheKeepsCategoriesAndPages(t *testing.T) {
	rec := newBodyRecorder(t, loadFixture(t, "search_v2_view_similar.json"))
	client := NewClient(WithBaseURL(rec.server.URL), WithHTTPClient(rec.server.Client()), WithCache(NewMemoryCache()))
	req := &SearchRequest{Category: Category{Parent: "Capacitors", Leaf: mlccLeaf}, CategoryCounts: true}

	first, err := client.Search.Query(context.Background(), req)
	if err != nil {
		t.Fatalf("first query failed: %v", err)
	}
	second, err := client.Search.Query(context.Background(), req)
	if err != nil {
		t.Fatalf("second query failed: %v", err)
	}
	if rec.count() != 1 {
		t.Fatalf("server got %d requests, want 1", rec.count())
	}
	if !reflect.DeepEqual(first, second) {
		t.Errorf("cached response differs\n got: %+v\nwant: %+v", second, first)
	}
	if len(second.Categories) != 1 || second.Pages != 1 {
		t.Errorf("cached response lost categories or pages: %+v", second)
	}
}

func TestSearchCacheKeyCoversNewOptions(t *testing.T) {
	base := SearchRequest{Keyword: "LM358"}
	variants := map[string]SearchRequest{
		"include preferred": {IncludePreferred: true},
		"category":          {Category: Category{Parent: "Capacitors"}},
		"attribute filters": {AttributeFilters: []AttributeFilter{{Name: "Capacitance", Values: []string{"100nF"}}}},
		"library types":     {LibraryTypes: []ComponentType{ComponentTypeBase}},
		"presale types":     {PresaleTypes: []PresaleType{PresaleTypeBuy}},
		"minimum stock":     {MinStock: 10},
		"PCBA":              {PCBA: PCBAFilterEconomic},
		"sort":              {Sort: SortByPrice},
		"sort descending":   {Sort: SortByPrice, SortDescending: true},
		"has datasheet":     {HasDatasheet: true},
		"category counts":   {CategoryCounts: true},
	}
	keys := map[string]string{cacheKeyForSearch(newSearchRequestBody(&base)): "base"}
	for name, variant := range variants {
		variant.Keyword = base.Keyword
		key := cacheKeyForSearch(newSearchRequestBody(&variant))
		if other, ok := keys[key]; ok {
			t.Errorf("%s has the same cache key as %s", name, other)
		}
		keys[key] = name
	}
}

// pagedServer serves pages of a fixed result set. Each page reports the
// total and the page count. A page past the end returns the live past-end
// shape: list null, total 0 and pages 0.
func pagedServer(t *testing.T, total, pageSize int, requests *atomic.Int32) *httptest.Server {
	t.Helper()
	return pagedServerWithPageCount(t, total, pageSize, true, requests)
}

// pagedServerWithPageCount works like pagedServer. When sendPages is false,
// each page reports the total, but the page count is 0.
func pagedServerWithPageCount(t *testing.T, total, pageSize int, sendPages bool, requests *atomic.Int32) *httptest.Server {
	t.Helper()
	pages := (total + pageSize - 1) / pageSize
	reportedPages := pages
	if !sendPages {
		reportedPages = 0
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		var req searchRequestBody
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request failed: %v", err)
		}
		if req.PageSize != pageSize {
			t.Errorf("pageSize = %d, want %d", req.PageSize, pageSize)
		}
		info := map[string]interface{}{"total": 0, "pageSize": 0, "pageNum": req.CurrentPage, "pages": 0, "list": nil}
		if req.CurrentPage >= 1 && req.CurrentPage <= pages {
			var list []map[string]interface{}
			for i := (req.CurrentPage - 1) * pageSize; i < min(total, req.CurrentPage*pageSize); i++ {
				list = append(list, map[string]interface{}{"componentCode": fmt.Sprintf("C%d", i+1)})
			}
			info = map[string]interface{}{"total": total, "pageSize": pageSize, "pageNum": req.CurrentPage, "pages": reportedPages, "list": list}
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code": 200, "message": nil,
			"data": map[string]interface{}{"componentPageInfo": info},
		})
	}))
	t.Cleanup(server.Close)
	return server
}

func TestSearchPagesWalksAllPages(t *testing.T) {
	var requests atomic.Int32
	server := pagedServer(t, 5, 2, &requests)
	client := NewClient(WithBaseURL(server.URL), WithHTTPClient(server.Client()), WithoutCache())

	req := &SearchRequest{Category: Category{Parent: "Capacitors"}, PageSize: 2}
	var codes []string
	var pageNums []int
	err := client.Search.Pages(context.Background(), req, func(resp *SearchResponse) error {
		pageNums = append(pageNums, resp.Page)
		if resp.TotalCount != 5 || resp.Pages != 3 {
			t.Errorf("page %d: total %d, pages %d; want 5 and 3", resp.Page, resp.TotalCount, resp.Pages)
		}
		for _, p := range resp.Products {
			codes = append(codes, p.ComponentCode)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Pages failed: %v", err)
	}
	if want := []string{"C1", "C2", "C3", "C4", "C5"}; !reflect.DeepEqual(codes, want) {
		t.Errorf("codes = %v, want %v", codes, want)
	}
	if want := []int{1, 2, 3}; !reflect.DeepEqual(pageNums, want) {
		t.Errorf("pages = %v, want %v", pageNums, want)
	}
	if got := requests.Load(); got != 3 {
		t.Errorf("server got %d requests, want 3", got)
	}
	if req.Page != 0 {
		t.Errorf("Pages changed req.Page to %d", req.Page)
	}
}

func TestSearchPagesStartPageAndStop(t *testing.T) {
	var requests atomic.Int32
	server := pagedServer(t, 7, 2, &requests)
	client := NewClient(WithBaseURL(server.URL), WithHTTPClient(server.Client()), WithoutCache())

	var pageNums []int
	err := client.Search.Pages(context.Background(), &SearchRequest{PageSize: 2, Page: 3}, func(resp *SearchResponse) error {
		pageNums = append(pageNums, resp.Page)
		return nil
	})
	if err != nil {
		t.Fatalf("Pages failed: %v", err)
	}
	if want := []int{3, 4}; !reflect.DeepEqual(pageNums, want) {
		t.Errorf("pages = %v, want %v", pageNums, want)
	}

	errStop := errors.New("stop")
	requests.Store(0)
	calls := 0
	err = client.Search.Pages(context.Background(), &SearchRequest{PageSize: 2}, func(*SearchResponse) error {
		calls++
		return errStop
	})
	if !errors.Is(err, errStop) {
		t.Errorf("Pages error = %v, want the error of fn", err)
	}
	if calls != 1 || requests.Load() != 1 {
		t.Errorf("fn calls = %d, requests = %d; want 1 and 1", calls, requests.Load())
	}
}

// TestSearchPagesKeepsFirstPageTotals uses a server whose later pages send
// total 0 and pages 0, as a live page past the end does.
func TestSearchPagesKeepsFirstPageTotals(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		var req searchRequestBody
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request failed: %v", err)
		}
		info := map[string]interface{}{"total": 0, "pages": 0, "pageNum": req.CurrentPage, "list": nil}
		switch req.CurrentPage {
		case 1:
			info = map[string]interface{}{"total": 4, "pages": 3, "pageNum": 1, "list": []map[string]string{{"componentCode": "C1"}, {"componentCode": "C2"}}}
		case 2:
			info["list"] = []map[string]string{{"componentCode": "C3"}}
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"code": 200, "data": map[string]interface{}{"componentPageInfo": info}})
	}))
	defer server.Close()
	client := NewClient(WithBaseURL(server.URL), WithHTTPClient(server.Client()), WithoutCache())

	var seen []string
	err := client.Search.Pages(context.Background(), &SearchRequest{PageSize: 2}, func(resp *SearchResponse) error {
		seen = append(seen, fmt.Sprintf("%d:%d/%d", resp.Page, resp.TotalCount, resp.Pages))
		return nil
	})
	if err != nil {
		t.Fatalf("Pages failed: %v", err)
	}
	if want := []string{"1:4/3", "2:4/3"}; !reflect.DeepEqual(seen, want) {
		t.Errorf("pages = %v, want %v", seen, want)
	}
	// Page 3 has no products, so the walk stops there.
	if got := requests.Load(); got != 3 {
		t.Errorf("server got %d requests, want 3", got)
	}
}

// TestSearchPagesTotalWithoutPageCount uses a server that sends the total
// but no page count. Pages then stops when it has all rows, or at an empty
// page when the walk starts after page 1.
func TestSearchPagesTotalWithoutPageCount(t *testing.T) {
	var requests atomic.Int32
	server := pagedServerWithPageCount(t, 5, 2, false, &requests)
	client := NewClient(WithBaseURL(server.URL), WithHTTPClient(server.Client()), WithoutCache())

	walk := func(start int) ([]string, []int) {
		t.Helper()
		requests.Store(0)
		var codes []string
		var pageNums []int
		err := client.Search.Pages(context.Background(), &SearchRequest{PageSize: 2, Page: start}, func(resp *SearchResponse) error {
			pageNums = append(pageNums, resp.Page)
			if resp.TotalCount != 5 || resp.Pages != 0 {
				t.Errorf("page %d: total %d, pages %d; want 5 and 0", resp.Page, resp.TotalCount, resp.Pages)
			}
			for _, p := range resp.Products {
				codes = append(codes, p.ComponentCode)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("Pages from page %d failed: %v", start, err)
		}
		return codes, pageNums
	}

	// From page 1, the walk stops after the last row and does not request
	// page 4.
	codes, pageNums := walk(1)
	if want := []string{"C1", "C2", "C3", "C4", "C5"}; !reflect.DeepEqual(codes, want) {
		t.Errorf("from page 1: codes = %v, want %v", codes, want)
	}
	if want := []int{1, 2, 3}; !reflect.DeepEqual(pageNums, want) {
		t.Errorf("from page 1: pages = %v, want %v", pageNums, want)
	}
	if got := requests.Load(); got != 3 {
		t.Errorf("from page 1: server got %d requests, want 3", got)
	}

	// From page 2, the walk sees fewer rows than the total, so it stops at
	// the empty page 4.
	codes, pageNums = walk(2)
	if want := []string{"C3", "C4", "C5"}; !reflect.DeepEqual(codes, want) {
		t.Errorf("from page 2: codes = %v, want %v", codes, want)
	}
	if want := []int{2, 3}; !reflect.DeepEqual(pageNums, want) {
		t.Errorf("from page 2: pages = %v, want %v", pageNums, want)
	}
	if got := requests.Load(); got != 3 {
		t.Errorf("from page 2: server got %d requests, want 3", got)
	}
}

func TestSearchPagesWithoutPageInfo(t *testing.T) {
	rec := newBodyRecorder(t, []byte(`{"code":200,"data":{"componentPageInfo":{"list":[{"componentCode":"C1"}]}}}`))
	calls := 0
	err := rec.client().Search.Pages(context.Background(), &SearchRequest{}, func(*SearchResponse) error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatalf("Pages failed: %v", err)
	}
	if calls != 1 || rec.count() != 1 {
		t.Errorf("fn calls = %d, requests = %d; want 1 and 1", calls, rec.count())
	}
}

func TestSearchPagesEmptyFirstPage(t *testing.T) {
	rec := newBodyRecorder(t, loadFixture(t, "search_v2_past_end.json"))
	err := rec.client().Search.Pages(context.Background(), &SearchRequest{Keyword: "none"}, func(*SearchResponse) error {
		t.Error("fn must not run for a page without products")
		return nil
	})
	if err != nil {
		t.Fatalf("Pages failed: %v", err)
	}
}

func TestSearchPagesInvalidInput(t *testing.T) {
	client := NewClient()
	if err := client.Search.Pages(context.Background(), nil, func(*SearchResponse) error { return nil }); !errors.Is(err, ErrInvalidRequest) {
		t.Errorf("nil request: error = %v, want ErrInvalidRequest", err)
	}
	if err := client.Search.Pages(context.Background(), &SearchRequest{}, nil); !errors.Is(err, ErrInvalidRequest) {
		t.Errorf("nil fn: error = %v, want ErrInvalidRequest", err)
	}
}

func TestSearchPagesRequestError(t *testing.T) {
	rec := newBodyRecorder(t, loadFixture(t, "search_v2_rejected.json"))
	err := rec.client().Search.Pages(context.Background(), &SearchRequest{}, func(*SearchResponse) error { return nil })
	if !errors.Is(err, ErrRejected) {
		t.Errorf("error = %v, want ErrRejected", err)
	}
}
