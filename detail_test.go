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
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// The detail fixtures come from live responses of 2026-10-02. The signed URL
// parameters are replaced with fixed strings, and the image and attribute
// lists are trimmed.
//
//   - detail_route_a_C2040.json: getComponentDetail?componentCode=C2040.
//   - detail_route_a_not_found.json: getComponentDetail for an unknown code.
//   - compare_details.json: compareComponentDetails, 4 records in the order
//     of the server (part code as a string).

// detailRequest is one request that apiRecorder got.
type detailRequest struct {
	Method      string
	Path        string
	Query       string
	ContentType string
	Body        []byte
}

// apiRecorder is a test server that records each request and answers with
// the response of respond.
type apiRecorder struct {
	mu       sync.Mutex
	requests []detailRequest
	server   *httptest.Server
}

func newAPIRecorder(t *testing.T, respond func(req detailRequest) []byte) *apiRecorder {
	t.Helper()
	rec := &apiRecorder{}
	rec.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}
		req := detailRequest{
			Method:      r.Method,
			Path:        r.URL.Path,
			Query:       r.URL.RawQuery,
			ContentType: r.Header.Get("Content-Type"),
			Body:        body,
		}
		rec.mu.Lock()
		rec.requests = append(rec.requests, req)
		rec.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(respond(req))
	}))
	t.Cleanup(rec.server.Close)
	return rec
}

// fixedAPIRecorder answers each request with the same response.
func fixedAPIRecorder(t *testing.T, response []byte) *apiRecorder {
	t.Helper()
	return newAPIRecorder(t, func(detailRequest) []byte { return response })
}

// client returns a client that sends every request to the test server.
func (rec *apiRecorder) client(opts ...ClientOption) *Client {
	base := []ClientOption{
		WithAPIRoot(rec.server.URL),
		WithHTTPClient(rec.server.Client()),
		WithRateLimit(1000),
		WithRetryConfig(RetryConfig{MaxRetries: 0, InitialBackoff: 1, MaxBackoff: 1, BackoffMultiplier: 1}),
		WithoutCache(),
	}
	return NewClient(append(base, opts...)...)
}

func (rec *apiRecorder) all() []detailRequest {
	rec.mu.Lock()
	defer rec.mu.Unlock()
	return slices.Clone(rec.requests)
}

// requestIDs decodes the JSON id list of a batch detail request.
func requestIDs(t *testing.T, body []byte) []string {
	t.Helper()
	var ids []string
	if err := json.Unmarshal(body, &ids); err != nil {
		t.Fatalf("decode batch body %s: %v", body, err)
	}
	return ids
}

func TestDetailRouteA(t *testing.T) {
	rec := fixedAPIRecorder(t, loadFixture(t, "detail_route_a_C2040.json"))

	detail, err := rec.client().Product.Detail(context.Background(), " c2040 ")
	if err != nil {
		t.Fatalf("detail failed: %v", err)
	}

	requests := rec.all()
	if len(requests) != 1 {
		t.Fatalf("server got %d requests, want 1", len(requests))
	}
	req := requests[0]
	if req.Method != http.MethodGet || req.Path != componentDetailPath || req.Query != "componentCode=C2040" || len(req.Body) != 0 {
		t.Errorf("request = %s %s?%s body %q, want GET %s?componentCode=C2040 without a body",
			req.Method, req.Path, req.Query, req.Body, componentDetailPath)
	}

	if detail.LCSCComponentID != 2392 || detail.ComponentCode != "C2040" || detail.ComponentModelEn != "RP2040" {
		t.Errorf("identity = %d %q %q", detail.LCSCComponentID, detail.ComponentCode, detail.ComponentModelEn)
	}
	if parent, leaf := detail.Category(); parent != "Embedded Processors & Controllers" || leaf != "Microcontrollers (MCU/MPU/SOC)" {
		t.Errorf("Category() = (%q, %q)", parent, leaf)
	}
	if detail.ParentCategoryID != 18 || detail.LeafCategoryID != 2584 {
		t.Errorf("category ids = %d, %d, want 18, 2584", detail.ParentCategoryID, detail.LeafCategoryID)
	}
	if detail.AssemblyProcess != "SMT" || detail.AssemblyMode != "smtWeld" || detail.XrayFlag {
		t.Errorf("assembly = %q %q xray %v", detail.AssemblyProcess, detail.AssemblyMode, detail.XrayFlag)
	}
	if detail.MoistureSensitivityLevelEn != "MSL 3" || detail.EccnCode != "EAR99" || detail.WarehouseCode != "sz" {
		t.Errorf("MSL %q, ECCN %q, warehouse %q", detail.MoistureSensitivityLevelEn, detail.EccnCode, detail.WarehouseCode)
	}
	if detail.EncapsulationNumber != 3400 || detail.EncapsulationUnit != "PCS" {
		t.Errorf("encapsulation = %d %q", detail.EncapsulationNumber, detail.EncapsulationUnit)
	}
	if detail.StockCount != 69484 || detail.CanPresaleNumber != 65524 || detail.MinPurchaseNum != 1 || detail.PreMinPurchaseNum != 11 {
		t.Errorf("stock fields = %d %d %d %d", detail.StockCount, detail.CanPresaleNumber, detail.MinPurchaseNum, detail.PreMinPurchaseNum)
	}
	if detail.InitialPrice != 0.9884 || detail.SpecialComponentFee != 0 || !detail.AllowPostFlag || detail.IsBuyComponent != "1" {
		t.Errorf("price and buy fields = %v %v %v %q", detail.InitialPrice, detail.SpecialComponentFee, detail.AllowPostFlag, detail.IsBuyComponent)
	}
	if detail.ComponentProductType != PCBAEligibilityBoth || detail.ComponentStatus != "yes" || detail.ComponentLibraryType != "expand" {
		t.Errorf("type fields = %d %q %q", detail.ComponentProductType, detail.ComponentStatus, detail.ComponentLibraryType)
	}
	if got := startNumbers(detail.Prices); !reflect.DeepEqual(got, []int{1, 10, 30, 100, 500, 1000}) {
		t.Errorf("Prices start numbers = %v", got)
	}
	if detail.BuyPrices != nil {
		t.Errorf("BuyPrices = %v, want nil (route A sends null)", detail.BuyPrices)
	}
	if detail.ProductBigImageAccessID != "" || detail.MinImageAccessID != "" || detail.DataManualFileAccessID != "" {
		t.Errorf("route A access ids = %q %q %q, want empty", detail.ProductBigImageAccessID, detail.MinImageAccessID, detail.DataManualFileAccessID)
	}
	if !strings.Contains(detail.ProductBigImageSignedURL, "x-oss-signature=FIXTURE") || !strings.Contains(detail.DataManualFileSignedURL, "C2040.pdf") {
		t.Errorf("signed URLs = %q, %q", detail.ProductBigImageSignedURL, detail.DataManualFileSignedURL)
	}
	if detail.URLSuffix != "" {
		t.Errorf("URLSuffix = %q, want empty for route A", detail.URLSuffix)
	}
	if len(detail.Attributes) != 3 || detail.Attributes[1] != (Attribute{Name: "CPU Maximum Speed", Value: "133MHz"}) {
		t.Errorf("Attributes = %+v", detail.Attributes)
	}
}

func TestDetailNotFound(t *testing.T) {
	rec := fixedAPIRecorder(t, loadFixture(t, "detail_route_a_not_found.json"))

	detail, err := rec.client().Product.Detail(context.Background(), "C999999999")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Detail = %+v, %v, want ErrNotFound", detail, err)
	}
	if n := len(rec.all()); n != 1 {
		t.Errorf("server got %d requests, want 1", n)
	}
}

func TestDetailNullData(t *testing.T) {
	rec := fixedAPIRecorder(t, []byte(`{"code":200,"data":null,"message":null}`))
	if _, err := rec.client().Product.Detail(context.Background(), "C1525"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}

func TestDetailInvalidCode(t *testing.T) {
	rec := fixedAPIRecorder(t, loadFixture(t, "detail_route_a_C2040.json"))
	client := rec.client()
	for _, code := range []string{"", "   ", "RP2040", "C", "C12a", "1525", "C1525,C2040"} {
		if _, err := client.Product.Detail(context.Background(), code); !errors.Is(err, ErrInvalidRequest) {
			t.Errorf("Detail(%q) error = %v, want ErrInvalidRequest", code, err)
		}
	}
	if n := len(rec.all()); n != 0 {
		t.Errorf("server got %d requests, want 0", n)
	}
}

func TestDetailRejected(t *testing.T) {
	rec := fixedAPIRecorder(t, []byte(`{"code":400,"data":null,"message":"Parameter exception."}`))
	_, err := rec.client().Product.Detail(context.Background(), "C2040")
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("error = %v, want ErrInvalidRequest for envelope code 400", err)
	}
}

func TestDetailCache(t *testing.T) {
	rec := fixedAPIRecorder(t, loadFixture(t, "detail_route_a_C2040.json"))
	client := rec.client(WithCacheConfig(DefaultCacheConfig()))

	first, err := client.Product.Detail(context.Background(), "C2040")
	if err != nil {
		t.Fatalf("first detail failed: %v", err)
	}
	first.Prices[0].ProductPrice = 99
	first.ComponentCode = "changed"

	second, err := client.Product.Detail(context.Background(), "c2040")
	if err != nil {
		t.Fatalf("second detail failed: %v", err)
	}
	if n := len(rec.all()); n != 1 {
		t.Fatalf("server got %d requests, want 1", n)
	}
	if second.ComponentCode != "C2040" || second.Prices[0].ProductPrice != 0.9884 {
		t.Errorf("cached record changed with the first result: %q %v", second.ComponentCode, second.Prices[0].ProductPrice)
	}
}

func TestDetailsByIDsCompare(t *testing.T) {
	rec := fixedAPIRecorder(t, loadFixture(t, "compare_details.json"))

	ids := []int64{5888166, 1877, 0, -5, 3349129, 1877, 3748453, 999999999}
	details, err := rec.client().Product.DetailsByIDs(context.Background(), ids)
	if err != nil {
		t.Fatalf("DetailsByIDs failed: %v", err)
	}

	requests := rec.all()
	if len(requests) != 1 {
		t.Fatalf("server got %d requests, want 1", len(requests))
	}
	req := requests[0]
	if req.Method != http.MethodPost || req.Path != compareDetailsPath || req.Query != "" {
		t.Errorf("request = %s %s?%s, want POST %s", req.Method, req.Path, req.Query, compareDetailsPath)
	}
	if req.ContentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", req.ContentType)
	}
	if want := `["5888166","1877","3349129","3748453","999999999"]`; string(req.Body) != want {
		t.Errorf("body = %s, want %s", req.Body, want)
	}

	var keys []int64
	for id := range details {
		keys = append(keys, id)
	}
	slices.Sort(keys)
	if want := []int64{1877, 3349129, 3748453, 5888166}; !reflect.DeepEqual(keys, want) {
		t.Fatalf("map keys = %v, want %v (999999999 is not found, not an error)", keys, want)
	}

	c1525 := details[1877]
	if c1525.ComponentCode != "C1525" || c1525.LCSCComponentID != 1877 || c1525.URLSuffix != "1877-CL05B104KO5NNNC/C1525" {
		t.Errorf("C1525 identity = %q %d %q", c1525.ComponentCode, c1525.LCSCComponentID, c1525.URLSuffix)
	}
	if parent, leaf := c1525.Category(); parent != "Capacitors" || leaf != mlccLeaf || c1525.ParentCategoryID != 2 || c1525.LeafCategoryID != 2929 {
		t.Errorf("C1525 category = (%q, %q) ids %d, %d", parent, leaf, c1525.ParentCategoryID, c1525.LeafCategoryID)
	}
	wantStarts := []int{1, 1000, 3000, 10000, 50000, 100000}
	if got := startNumbers(c1525.Prices); !reflect.DeepEqual(got, wantStarts) {
		t.Errorf("C1525 Prices start numbers = %v, want %v", got, wantStarts)
	}
	if got := startNumbers(c1525.BuyPrices); !reflect.DeepEqual(got, wantStarts) {
		t.Errorf("C1525 BuyPrices start numbers = %v, want %v (sorted)", got, wantStarts)
	}
	if c1525.BuyPrices[0].ProductPrice != 0.0043 || c1525.BuyPrices[1].ProductPrice != 0.0033 {
		t.Errorf("C1525 BuyPrices lost the price of a tier: %+v", c1525.BuyPrices)
	}
	if c1525.ProductBigImageAccessID != "8552476004850417664" || c1525.MinImageAccessID != "8552476005450338304" || c1525.DataManualFileAccessID != "8579707269996871680" {
		t.Errorf("C1525 access ids = %q %q %q", c1525.ProductBigImageAccessID, c1525.MinImageAccessID, c1525.DataManualFileAccessID)
	}
	if c1525.LossNumber != 10 || c1525.LeastPatchNumber != 20 || c1525.PreMinPurchaseNum != 2654 || c1525.ComponentDesignator != "C" {
		t.Errorf("C1525 ordering fields = %d %d %d %q", c1525.LossNumber, c1525.LeastPatchNumber, c1525.PreMinPurchaseNum, c1525.ComponentDesignator)
	}

	eol := details[3349129]
	if eol.IsBuyComponent != "0" || eol.NoBuyReason != "This product is no longer manufactured." {
		t.Errorf("C2961140 buy fields = %q %q", eol.IsBuyComponent, eol.NoBuyReason)
	}
	if eol.ComponentAlternativesCode != "C2040" || eol.AlternativesLCSCComponentID != 2392 || eol.ReplaceURLSuffix != "RaspberryPi-RP2040/C2040" {
		t.Errorf("C2961140 replacement = %q %d %q", eol.ComponentAlternativesCode, eol.AlternativesLCSCComponentID, eol.ReplaceURLSuffix)
	}
	if eol.EccnCode != "-" || eol.CanPresaleNumber != 0 || eol.MinPurchaseNum != 6 {
		t.Errorf("C2961140 fields = %q %d %d", eol.EccnCode, eol.CanPresaleNumber, eol.MinPurchaseNum)
	}

	tht := details[3748453]
	if tht.AssemblyProcess != "THT" || tht.AssemblyMode != "manualWeld" || tht.EncapsulationNumber != 25 {
		t.Errorf("C3186512 assembly = %q %q %d", tht.AssemblyProcess, tht.AssemblyMode, tht.EncapsulationNumber)
	}

	gan := details[5888166]
	if !gan.XrayFlag || gan.ComponentProductType != PCBAEligibilityStandardOnly || gan.ComponentProductType.AllowsEconomic() {
		t.Errorf("C5200613 xray %v, PCBA type %d", gan.XrayFlag, gan.ComponentProductType)
	}
	if gan.ProductBigImageAccessID != "" || gan.DataManualFileAccessID != "8589835592130596864" {
		t.Errorf("C5200613 access ids = %q %q", gan.ProductBigImageAccessID, gan.DataManualFileAccessID)
	}
	if got := startNumbers(gan.BuyPrices); !reflect.DeepEqual(got, []int{1, 10, 30, 200, 500, 1000}) {
		t.Errorf("C5200613 BuyPrices start numbers = %v", got)
	}
}

// compareRecord returns a minimal batch detail row for a part id. The part
// code is "C" plus the id.
func compareRecord(id int64) map[string]interface{} {
	return map[string]interface{}{
		"urlSuffix": fmt.Sprintf("part-%d/C%d", id, id),
		"componentDetailVo": map[string]interface{}{
			"lcscComponentId": id,
			"componentCode":   fmt.Sprintf("C%d", id),
			"prices": []map[string]interface{}{
				{"startNumber": 100, "endNumber": -1, "productPrice": 0.5},
				{"startNumber": 1, "endNumber": 99, "productPrice": 0.7},
			},
			"buyPrices": []map[string]interface{}{
				{"startNumber": 50, "endNumber": -1, "productPrice": 0.4},
				{"startNumber": 1, "endNumber": 9, "productPrice": 0.6},
				{"startNumber": 10, "endNumber": 49, "productPrice": 0.5},
			},
		},
	}
}

// compareResponse answers a batch detail request like the live server: it
// returns a record for each requested id that known accepts, sorted by part
// code as a string. It also adds the extra ids, which nobody requested. A
// non-numeric id rejects the whole request with code 101.
func compareResponse(t *testing.T, body []byte, known func(int64) bool, extra ...int64) []byte {
	t.Helper()
	var ids []int64
	for _, raw := range requestIDs(t, body) {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return []byte(`{"success":false,"code":101,"message":"rejected","data":null}`)
		}
		if known(id) {
			ids = append(ids, id)
		}
	}
	ids = append(ids, extra...)
	slices.SortFunc(ids, func(a, b int64) int {
		return strings.Compare(fmt.Sprintf("C%d", a), fmt.Sprintf("C%d", b))
	})
	rows := make([]map[string]interface{}, 0, len(ids))
	for _, id := range ids {
		rows = append(rows, compareRecord(id))
	}
	data, err := json.Marshal(map[string]interface{}{"code": 200, "message": nil, "data": rows})
	if err != nil {
		t.Errorf("encode response: %v", err)
	}
	return data
}

// compareServer is a test server that answers with compareResponse.
func compareServer(t *testing.T, known func(int64) bool, extra ...int64) *apiRecorder {
	t.Helper()
	return newAPIRecorder(t, func(req detailRequest) []byte {
		return compareResponse(t, req.Body, known, extra...)
	})
}

func TestDetailsByIDsChunks(t *testing.T) {
	rec := compareServer(t, func(id int64) bool { return id%7 != 0 }, 424242)

	ids := make([]int64, 0, 600)
	for id := int64(1); id <= 600; id++ {
		ids = append(ids, id)
	}
	details, err := rec.client().Product.DetailsByIDs(context.Background(), ids)
	if err != nil {
		t.Fatalf("DetailsByIDs failed: %v", err)
	}

	requests := rec.all()
	if len(requests) != 3 {
		t.Fatalf("server got %d requests, want 3", len(requests))
	}
	var sent []string
	for i, want := range []int{250, 250, 100} {
		chunk := requestIDs(t, requests[i].Body)
		if len(chunk) != want {
			t.Errorf("request %d has %d ids, want %d", i+1, len(chunk), want)
		}
		sent = append(sent, chunk...)
	}
	for i, raw := range sent {
		if raw != strconv.Itoa(i+1) {
			t.Fatalf("id %d of the requests = %q, want %d (request order)", i, raw, i+1)
		}
	}

	if want := 600 - 600/7; len(details) != want {
		t.Errorf("got %d records, want %d", len(details), want)
	}
	if _, ok := details[424242]; ok {
		t.Error("DetailsByIDs kept a record that nobody requested")
	}
	for id, detail := range details {
		if id%7 == 0 {
			t.Errorf("record %d must be missing", id)
		}
		if detail.LCSCComponentID != id || detail.ComponentCode != fmt.Sprintf("C%d", id) || detail.URLSuffix != fmt.Sprintf("part-%d/C%d", id, id) {
			t.Errorf("record %d = %d %q %q", id, detail.LCSCComponentID, detail.ComponentCode, detail.URLSuffix)
		}
	}
}

func TestDetailsByIDsSortsLadders(t *testing.T) {
	rec := compareServer(t, func(int64) bool { return true })
	details, err := rec.client().Product.DetailsByIDs(context.Background(), []int64{42})
	if err != nil {
		t.Fatalf("DetailsByIDs failed: %v", err)
	}
	detail := details[42]
	if detail == nil {
		t.Fatal("record 42 is missing")
	}
	want := []PriceBreak{{StartNumber: 1, EndNumber: 99, ProductPrice: 0.7}, {StartNumber: 100, EndNumber: -1, ProductPrice: 0.5}}
	if !reflect.DeepEqual(detail.Prices, want) {
		t.Errorf("Prices = %+v, want %+v", detail.Prices, want)
	}
	wantBuy := []PriceBreak{
		{StartNumber: 1, EndNumber: 9, ProductPrice: 0.6},
		{StartNumber: 10, EndNumber: 49, ProductPrice: 0.5},
		{StartNumber: 50, EndNumber: -1, ProductPrice: 0.4},
	}
	if !reflect.DeepEqual(detail.BuyPrices, wantBuy) {
		t.Errorf("BuyPrices = %+v, want %+v", detail.BuyPrices, wantBuy)
	}
}

func TestDetailsByIDsEmpty(t *testing.T) {
	rec := compareServer(t, func(int64) bool { return true })
	client := rec.client()
	for _, ids := range [][]int64{nil, {}, {0, -1, -1877}} {
		details, err := client.Product.DetailsByIDs(context.Background(), ids)
		if err != nil || details == nil || len(details) != 0 {
			t.Errorf("DetailsByIDs(%v) = %v, %v, want an empty map", ids, details, err)
		}
	}
	if n := len(rec.all()); n != 0 {
		t.Errorf("server got %d requests, want 0", n)
	}
}

func TestDetailsByIDsCache(t *testing.T) {
	rec := fixedAPIRecorder(t, loadFixture(t, "compare_details.json"))
	client := rec.client(WithCacheConfig(DefaultCacheConfig()))
	ctx := context.Background()

	first, err := client.Product.DetailsByIDs(ctx, []int64{1877, 3349129})
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}
	if len(first) != 2 {
		t.Fatalf("first call returned %d records, want 2", len(first))
	}
	first[1877].BuyPrices[0].ProductPrice = 99

	second, err := client.Product.DetailsByIDs(ctx, []int64{1877, 5888166, 3349129})
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}
	third, err := client.Product.DetailsByIDs(ctx, []int64{3349129, 1877, 5888166})
	if err != nil {
		t.Fatalf("third call failed: %v", err)
	}

	requests := rec.all()
	if len(requests) != 2 {
		t.Fatalf("server got %d requests, want 2", len(requests))
	}
	if got := string(requests[1].Body); got != `["5888166"]` {
		t.Errorf("second request body = %s, want only the id that is not in the cache", got)
	}
	if len(second) != 3 || len(third) != 3 {
		t.Errorf("record counts = %d, %d, want 3, 3", len(second), len(third))
	}
	if got := second[1877].BuyPrices[0].ProductPrice; got != 0.0043 {
		t.Errorf("cached C1525 buy price = %v, want 0.0043 (the caller changed only its copy)", got)
	}
	if !reflect.DeepEqual(second[3349129], third[3349129]) || second[5888166].ComponentCode != "C5200613" {
		t.Errorf("cached records differ: %+v, %+v", second[3349129], third[3349129])
	}
	// The first response also held C3186512 and C5200613, which the first
	// call did not request. The cache must not keep them.
	if _, ok := client.cachedDetail(cacheKeyForDetailID(3748453)); ok {
		t.Error("the cache kept a record that nobody requested")
	}
}

func TestDetailsByIDsRejected(t *testing.T) {
	rec := fixedAPIRecorder(t, loadFixture(t, "search_v2_rejected.json"))
	details, err := rec.client(WithRetryConfig(DefaultRetryConfig())).Product.DetailsByIDs(context.Background(), []int64{1877})
	if !errors.Is(err, ErrRejected) || details != nil {
		t.Fatalf("DetailsByIDs = %v, %v, want nil and ErrRejected", details, err)
	}
	if n := len(rec.all()); n != 1 {
		t.Errorf("server got %d requests, want 1 (no retry for code 101)", n)
	}
}

func TestDetailsByIDsErrorKeepsEarlierChunksInCache(t *testing.T) {
	var mu sync.Mutex
	var calls int
	rec := newAPIRecorder(t, func(req detailRequest) []byte {
		mu.Lock()
		calls++
		n := calls
		mu.Unlock()
		if n == 2 {
			return []byte(`{"code":500,"message":"System error. Please try again later.","data":null}`)
		}
		return compareResponse(t, req.Body, func(int64) bool { return true })
	})
	client := rec.client(WithCacheConfig(DefaultCacheConfig()))

	ids := make([]int64, 0, 300)
	for id := int64(1); id <= 300; id++ {
		ids = append(ids, id)
	}
	details, err := client.Product.DetailsByIDs(context.Background(), ids)
	if !errors.Is(err, ErrServer) || details != nil {
		t.Fatalf("DetailsByIDs = %d records, %v, want nil and ErrServer", len(details), err)
	}
	for _, id := range []int64{1, 250} {
		if _, ok := client.cachedDetail(cacheKeyForDetailID(id)); !ok {
			t.Errorf("record %d of the first request is not in the cache", id)
		}
	}
	if _, ok := client.cachedDetail(cacheKeyForDetailID(251)); ok {
		t.Error("record 251 of the failed request is in the cache")
	}
}

func TestComponentDetailProduct(t *testing.T) {
	rec := fixedAPIRecorder(t, loadFixture(t, "compare_details.json"))
	details, err := rec.client().Product.DetailsByIDs(context.Background(), []int64{1877, 3349129, 3748453, 5888166})
	if err != nil {
		t.Fatalf("DetailsByIDs failed: %v", err)
	}

	// Decode each raw record into a Product too. The fields with the same
	// JSON name must have the same value in ComponentDetail.Product. Only
	// the category order, the part id, the ladders and the URL suffix need
	// a fix.
	var raw struct {
		Data []struct {
			URLSuffix string          `json:"urlSuffix"`
			Record    json.RawMessage `json:"componentDetailVo"`
		} `json:"data"`
	}
	if err := json.Unmarshal(loadFixture(t, "compare_details.json"), &raw); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	for _, row := range raw.Data {
		var want Product
		if err := json.Unmarshal(row.Record, &want); err != nil {
			t.Fatalf("decode record as Product: %v", err)
		}
		var ids struct {
			ID        int64        `json:"lcscComponentId"`
			Prices    []PriceBreak `json:"prices"`
			BuyPrices []PriceBreak `json:"buyPrices"`
		}
		if err := json.Unmarshal(row.Record, &ids); err != nil {
			t.Fatalf("decode record ids: %v", err)
		}
		want.FirstSortName, want.SecondSortName = want.SecondSortName, want.FirstSortName
		want.ComponentID = int(ids.ID)
		want.ComponentPrices = SortPriceBreaks(ids.Prices)
		want.BuyComponentPrices = SortPriceBreaks(ids.BuyPrices)
		want.UrlSuffix = row.URLSuffix

		detail := details[ids.ID]
		if detail == nil {
			t.Fatalf("record %d is missing", ids.ID)
		}
		got := detail.Product()
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: Product() differs from the raw record\n got: %+v\nwant: %+v", detail.ComponentCode, got, want)
		}

		dParent, dLeaf := detail.Category()
		pParent, pLeaf := got.Category()
		if dParent != pParent || dLeaf != pLeaf {
			t.Errorf("%s: Product().Category() = (%q, %q), want (%q, %q)", detail.ComponentCode, pParent, pLeaf, dParent, dLeaf)
		}
	}

	// The search row and the detail record of C1525 must give the same
	// part id and category.
	var wrapper searchResponseWrapper
	if err := json.Unmarshal(loadFixture(t, "search_v2_rows.json"), &wrapper); err != nil {
		t.Fatalf("decode search fixture: %v", err)
	}
	var row *Product
	for i := range wrapper.Data.ComponentPageInfo.Products {
		if p := &wrapper.Data.ComponentPageInfo.Products[i]; p.ComponentCode == "C1525" {
			row = p
		}
	}
	if row == nil {
		t.Fatal("search fixture has no C1525 row")
	}
	product := details[1877].Product()
	rowParent, rowLeaf := row.Category()
	parent, leaf := product.Category()
	if product.ComponentID != row.ComponentID || parent != rowParent || leaf != rowLeaf {
		t.Errorf("detail C1525 = id %d (%q, %q), search row = id %d (%q, %q)",
			product.ComponentID, parent, leaf, row.ComponentID, rowParent, rowLeaf)
	}
	if product.LibraryType() != LibraryTypeBasic || !product.Buyable() {
		t.Errorf("C1525 library type %q, buyable %v", product.LibraryType(), product.Buyable())
	}

	// Product copies the slices.
	product.ComponentPrices[0].ProductPrice = 99
	product.BuyComponentPrices[0].ProductPrice = 99
	product.Attributes[0].Value = "changed"
	if details[1877].Prices[0].ProductPrice == 99 || details[1877].BuyPrices[0].ProductPrice == 99 || details[1877].Attributes[0].Value == "changed" {
		t.Error("a change to the Product changed the ComponentDetail")
	}
}

func TestComponentDetailProductQuote(t *testing.T) {
	rec := fixedAPIRecorder(t, loadFixture(t, "compare_details.json"))
	details, err := rec.client().Product.DetailsByIDs(context.Background(), []int64{5888166})
	if err != nil {
		t.Fatalf("DetailsByIDs failed: %v", err)
	}
	product := details[5888166].Product()

	// C5200613 has stock 0 and CanPresaleNumber 0, so each order is a
	// pre-order on the sorted buy ladder.
	quote := product.PartsOrderQuote(10)
	if !quote.PreOrder || quote.MinQty != 7 {
		t.Errorf("quote = %+v, want a pre-order with minimum 7", quote)
	}
	if got := startNumbers(quote.Ladder); !reflect.DeepEqual(got, []int{1, 10, 30, 200, 500, 1000}) || quote.Ladder[0].ProductPrice != 1.4282 {
		t.Errorf("quote ladder = %+v", quote.Ladder)
	}
}
