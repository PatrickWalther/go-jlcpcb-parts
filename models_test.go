package jlcpcb

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
)

// TestFlexFloat64UnmarshalNumber tests unmarshaling numeric JSON values.
func TestFlexFloat64UnmarshalNumber(t *testing.T) {
	data := []byte(`123.45`)

	var f FlexFloat64
	err := json.Unmarshal(data, &f)

	if err != nil {
		t.Fatalf("failed to unmarshal number: %v", err)
	}

	if float64(f) != 123.45 {
		t.Errorf("expected 123.45, got %f", f)
	}
}

// TestFlexFloat64UnmarshalString tests unmarshaling string JSON values.
func TestFlexFloat64UnmarshalString(t *testing.T) {
	data := []byte(`"456.78"`)

	var f FlexFloat64
	err := json.Unmarshal(data, &f)

	if err != nil {
		t.Fatalf("failed to unmarshal string: %v", err)
	}

	if float64(f) != 456.78 {
		t.Errorf("expected 456.78, got %f", f)
	}
}

// TestFlexFloat64UnmarshalZero tests unmarshaling zero values.
func TestFlexFloat64UnmarshalZero(t *testing.T) {
	tests := []struct {
		data     []byte
		expected float64
	}{
		{[]byte(`0`), 0},
		{[]byte(`"0"`), 0},
		{[]byte(`0.0`), 0.0},
		{[]byte(`"0.0"`), 0.0},
	}

	for _, test := range tests {
		var f FlexFloat64
		err := json.Unmarshal(test.data, &f)

		if err != nil {
			t.Errorf("failed to unmarshal %s: %v", test.data, err)
			continue
		}

		if float64(f) != test.expected {
			t.Errorf("expected %f for input %s, got %f", test.expected, test.data, f)
		}
	}
}

// TestFlexFloat64UnmarshalNegative tests unmarshaling negative values.
func TestFlexFloat64UnmarshalNegative(t *testing.T) {
	data := []byte(`"-123.45"`)

	var f FlexFloat64
	err := json.Unmarshal(data, &f)

	if err != nil {
		t.Fatalf("failed to unmarshal negative string: %v", err)
	}

	if float64(f) != -123.45 {
		t.Errorf("expected -123.45, got %f", f)
	}
}

// TestFlexFloat64UnmarshalInvalid tests error handling for invalid values.
func TestFlexFloat64UnmarshalInvalid(t *testing.T) {
	invalidData := [][]byte{
		[]byte(`"not a number"`),
		[]byte(`true`),
		[]byte(`[]`),
	}

	for _, data := range invalidData {
		var f FlexFloat64
		err := json.Unmarshal(data, &f)

		if err == nil {
			t.Errorf("expected error for invalid data %s, but got none", data)
		}
	}
}

// TestProductGetURL tests the GetProductURL method.
func TestProductGetURL(t *testing.T) {
	product := &Product{
		ComponentCode: "C5676715",
		UrlSuffix:     "6597989-MPM3506AGQVZ/C5676715",
	}

	expectedURL := "https://jlcpcb.com/partdetail/6597989-MPM3506AGQVZ/C5676715"
	actualURL := product.GetProductURL()

	if actualURL != expectedURL {
		t.Errorf("expected %s, got %s", expectedURL, actualURL)
	}
}

// TestProductGetURLSpecialCharacters tests GetProductURL with URL suffix fallback.
func TestProductGetURLSpecialCharacters(t *testing.T) {
	product := &Product{
		ComponentCode: "C123456",
	}

	url := product.GetProductURL()

	if url == "" {
		t.Fatal("expected non-empty URL")
	}

	expected := "https://jlcpcb.com/partdetail/C123456"
	if url != expected {
		t.Errorf("expected %s, got %s", expected, url)
	}
}

// TestSearchResponseStructure tests SearchResponse structure.
func TestSearchResponseStructure(t *testing.T) {
	resp := &SearchResponse{
		Products:   []Product{{ComponentCode: "C1"}, {ComponentCode: "C2"}},
		TotalCount: 100,
		PageSize:   10,
		Page:       1,
	}

	if len(resp.Products) != 2 {
		t.Errorf("expected 2 products, got %d", len(resp.Products))
	}

	if resp.TotalCount != 100 {
		t.Errorf("expected total count 100, got %d", resp.TotalCount)
	}

	if resp.PageSize != 10 {
		t.Errorf("expected page size 10, got %d", resp.PageSize)
	}

	if resp.Page != 1 {
		t.Errorf("expected page number 1, got %d", resp.Page)
	}
}

// TestPriceBreakValidation tests PriceBreak structure.
func TestPriceBreakValidation(t *testing.T) {
	pb := PriceBreak{
		StartNumber:  1,
		EndNumber:    49,
		ProductPrice: FlexFloat64(9.99),
	}

	if pb.StartNumber != 1 {
		t.Errorf("expected start number 1, got %d", pb.StartNumber)
	}

	if pb.EndNumber != 49 {
		t.Errorf("expected end number 49, got %d", pb.EndNumber)
	}

	if float64(pb.ProductPrice) != 9.99 {
		t.Errorf("expected price 9.99, got %f", pb.ProductPrice)
	}
}

// TestProductStructure tests complete Product structure.
func TestProductStructure(t *testing.T) {
	product := &Product{
		ComponentCode:            "C5676715",
		ComponentModelEn:         "MPM3506AGQV-Z",
		ComponentBrandEn:         "Monolithic Power Systems",
		ComponentTypeEn:          "DC-DC Power Modules",
		ComponentName:            "MPS MPM3506AGQV-Z",
		ComponentSpecificationEn: "QFN-19(3x5)",
		DataManualUrl:            "https://example.com/datasheet.pdf",
		StockCount:               549,
		MinPurchaseNum:           1,
		ComponentPrices:          []PriceBreak{{StartNumber: 1, EndNumber: 49, ProductPrice: 4.09}},
		Attributes:               []Attribute{{Name: "Output Current(Max)", Value: "600mA"}},
		FirstSortName:            "DC-DC Power Modules",
		SecondSortName:           "Power Modules",
		IsBuyComponent:           "1",
	}

	if product.ComponentCode != "C5676715" {
		t.Error("component code mismatch")
	}
	if product.ComponentBrandEn != "Monolithic Power Systems" {
		t.Error("manufacturer mismatch")
	}
	if product.StockCount != 549 {
		t.Error("stock mismatch")
	}
	if len(product.Attributes) != 1 {
		t.Error("attributes mismatch")
	}
	if product.IsBuyComponent != "1" {
		t.Error("is buy component mismatch")
	}
}

// TestAttributeStructure tests Attribute structure.
func TestAttributeStructure(t *testing.T) {
	attr := Attribute{
		Name:  "Temperature",
		Value: "-40°C to +125°C",
	}

	if attr.Name != "Temperature" {
		t.Errorf("expected attribute name Temperature, got %s", attr.Name)
	}

	if attr.Value != "-40°C to +125°C" {
		t.Errorf("expected attribute value -40°C to +125°C, got %s", attr.Value)
	}
}

// TestSearchRequestStructure tests SearchRequest structure.
func TestSearchRequestStructure(t *testing.T) {
	req := SearchRequest{
		Keyword:       "MPM3506",
		Page:          1,
		PageSize:      20,
		PresaleType:   PresaleTypeAny,
		StockOnly:     true,
		ComponentType: ComponentTypeBase,
	}

	if req.Keyword != "MPM3506" {
		t.Errorf("expected keyword MPM3506, got %s", req.Keyword)
	}

	if req.PageSize != 20 {
		t.Errorf("expected page size 20, got %d", req.PageSize)
	}

	if !req.StockOnly {
		t.Error("expected StockOnly to be true")
	}
}

// loadFixture reads a raw selectSmtComponentList/v2 response from testdata.
func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return data
}

// decodeFixture decodes a fixture and returns its products by component code.
func decodeFixture(t *testing.T, name string) map[string]Product {
	t.Helper()
	var wrapper searchResponseWrapper
	if err := json.Unmarshal(loadFixture(t, name), &wrapper); err != nil {
		t.Fatalf("decode fixture %s: %v", name, err)
	}
	products := make(map[string]Product)
	for _, p := range wrapper.Data.ComponentPageInfo.Products {
		products[p.ComponentCode] = p
	}
	return products
}

// productFields holds the decoded fields that a fixture test checks.
type productFields struct {
	ComponentLibraryType      string
	PreferredComponentFlag    bool
	LossNumber                int
	LeastPatchNumber          int
	CanPresaleNumber          int
	NoBuyReason               string
	EncapsulationNumber       int
	PreMinPurchaseNum         int
	MinPurchaseNum            int
	ComponentAlternativesCode string
	AssemblyComponentFlag     bool
	IsBuyComponent            string
	LibraryType               LibraryType
	Buyable                   bool
}

func fieldsOf(p *Product) productFields {
	return productFields{
		ComponentLibraryType:      p.ComponentLibraryType,
		PreferredComponentFlag:    p.PreferredComponentFlag,
		LossNumber:                p.LossNumber,
		LeastPatchNumber:          p.LeastPatchNumber,
		CanPresaleNumber:          p.CanPresaleNumber,
		NoBuyReason:               p.NoBuyReason,
		EncapsulationNumber:       p.EncapsulationNumber,
		PreMinPurchaseNum:         p.PreMinPurchaseNum,
		MinPurchaseNum:            p.MinPurchaseNum,
		ComponentAlternativesCode: p.ComponentAlternativesCode,
		AssemblyComponentFlag:     p.AssemblyComponentFlag,
		IsBuyComponent:            p.IsBuyComponent,
		LibraryType:               p.LibraryType(),
		Buyable:                   p.Buyable(),
	}
}

// TestProductDecodeLiveFixtures decodes trimmed live API responses.
func TestProductDecodeLiveFixtures(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		code    string
		want    productFields
	}{
		{
			name:    "basic part",
			fixture: "search_C25744.json",
			code:    "C25744",
			want: productFields{
				ComponentLibraryType: "base",
				LossNumber:           10,
				LeastPatchNumber:     20,
				CanPresaleNumber:     19172766,
				EncapsulationNumber:  10000,
				PreMinPurchaseNum:    3923,
				MinPurchaseNum:       1,
				IsBuyComponent:       "1",
				LibraryType:          LibraryTypeBasic,
				Buyable:              true,
			},
		},
		{
			name:    "preferred extended part",
			fixture: "search_C7593.json",
			code:    "C7593",
			want: productFields{
				ComponentLibraryType:   "expand",
				PreferredComponentFlag: true,
				CanPresaleNumber:       221447,
				EncapsulationNumber:    2500,
				PreMinPurchaseNum:      95,
				MinPurchaseNum:         1,
				IsBuyComponent:         "1",
				LibraryType:            LibraryTypePreferred,
				Buyable:                true,
			},
		},
		{
			name:    "extended part",
			fixture: "search_C2040.json",
			code:    "C2040",
			want: productFields{
				ComponentLibraryType: "expand",
				CanPresaleNumber:     67973,
				EncapsulationNumber:  3400,
				PreMinPurchaseNum:    11,
				MinPurchaseNum:       1,
				IsBuyComponent:       "1",
				LibraryType:          LibraryTypeExtended,
				Buyable:              true,
			},
		},
		{
			name:    "negative pre-order quantity",
			fixture: "search_C2040.json",
			code:    "C5200613",
			want: productFields{
				ComponentLibraryType: "expand",
				CanPresaleNumber:     -1,
				EncapsulationNumber:  3000,
				PreMinPurchaseNum:    7,
				MinPurchaseNum:       7,
				IsBuyComponent:       "1",
				LibraryType:          LibraryTypeExtended,
				Buyable:              true,
			},
		},
		{
			name:    "part that JLCPCB does not sell",
			fixture: "search_C2040.json",
			code:    "C19400368",
			want: productFields{
				ComponentLibraryType: "expand",
				NoBuyReason:          "This product is no longer manufactured.",
				EncapsulationNumber:  15,
				PreMinPurchaseNum:    1,
				MinPurchaseNum:       1,
				IsBuyComponent:       "0",
				LibraryType:          LibraryTypeExtended,
				Buyable:              false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			products := decodeFixture(t, tt.fixture)
			product, ok := products[tt.code]
			if !ok {
				t.Fatalf("fixture %s has no product %s", tt.fixture, tt.code)
			}
			if got := fieldsOf(&product); got != tt.want {
				t.Errorf("decoded fields mismatch\n got: %+v\nwant: %+v", got, tt.want)
			}
		})
	}
}

// TestProductDecodeNonZeroFields checks the JSON tag of each new field.
// The live fixtures have no true assemblyComponentFlag and no non-empty
// componentAlternativesCode, so this test uses a non-zero value for each field.
func TestProductDecodeNonZeroFields(t *testing.T) {
	data := []byte(`{
		"componentCode": "C1",
		"componentLibraryType": "expand",
		"preferredComponentFlag": true,
		"lossNumber": 3,
		"leastPatchNumber": 5,
		"canPresaleNumber": -10107,
		"noBuyReason": "This product is no longer manufactured.",
		"encapsulationNumber": 4000,
		"preMinPurchaseNum": 42,
		"minPurchaseNum": 2,
		"componentAlternativesCode": "C1525",
		"assemblyComponentFlag": true,
		"isBuyComponent": "0"
	}`)
	want := productFields{
		ComponentLibraryType:      "expand",
		PreferredComponentFlag:    true,
		LossNumber:                3,
		LeastPatchNumber:          5,
		CanPresaleNumber:          -10107,
		NoBuyReason:               "This product is no longer manufactured.",
		EncapsulationNumber:       4000,
		PreMinPurchaseNum:         42,
		MinPurchaseNum:            2,
		ComponentAlternativesCode: "C1525",
		AssemblyComponentFlag:     true,
		IsBuyComponent:            "0",
		LibraryType:               LibraryTypePreferred,
		Buyable:                   false,
	}

	var product Product
	if err := json.Unmarshal(data, &product); err != nil {
		t.Fatalf("decode non-zero fields: %v", err)
	}
	if got := fieldsOf(&product); got != want {
		t.Errorf("decoded fields mismatch\n got: %+v\nwant: %+v", got, want)
	}

	// The cache stores a product as JSON. Encode and decode it again.
	encoded, err := json.Marshal(product)
	if err != nil {
		t.Fatalf("encode product: %v", err)
	}
	var roundTrip Product
	if err := json.Unmarshal(encoded, &roundTrip); err != nil {
		t.Fatalf("decode encoded product: %v", err)
	}
	if got := fieldsOf(&roundTrip); got != want {
		t.Errorf("round-trip fields mismatch\n got: %+v\nwant: %+v", got, want)
	}
}

// TestProductDecodeNullFields checks that JSON null decodes to zero values.
func TestProductDecodeNullFields(t *testing.T) {
	data := []byte(`{
		"componentCode": "C1",
		"componentLibraryType": null,
		"preferredComponentFlag": null,
		"lossNumber": null,
		"leastPatchNumber": null,
		"canPresaleNumber": null,
		"noBuyReason": null,
		"encapsulationNumber": null,
		"preMinPurchaseNum": null,
		"minPurchaseNum": null,
		"componentAlternativesCode": null,
		"assemblyComponentFlag": null,
		"isBuyComponent": null,
		"componentPrices": null,
		"buyComponentPrices": null
	}`)

	var product Product
	if err := json.Unmarshal(data, &product); err != nil {
		t.Fatalf("decode null fields: %v", err)
	}
	want := productFields{Buyable: true}
	if got := fieldsOf(&product); got != want {
		t.Errorf("null fields mismatch\n got: %+v\nwant: %+v", got, want)
	}
	if product.SortedComponentPrices() != nil || product.SortedBuyComponentPrices() != nil {
		t.Error("expected nil sorted prices for null price ladders")
	}
}

// TestProductLibraryType tests the library class mapping.
func TestProductLibraryType(t *testing.T) {
	tests := []struct {
		raw       string
		preferred bool
		want      LibraryType
	}{
		{"base", false, LibraryTypeBasic},
		{"base", true, LibraryTypeBasic},
		{"expand", true, LibraryTypePreferred},
		{"expand", false, LibraryTypeExtended},
		{" Expand ", false, LibraryTypeExtended},
		{"BASE", false, LibraryTypeBasic},
		{"", false, ""},
		{"", true, ""},
		{"unknown", true, ""},
	}

	for _, tt := range tests {
		product := &Product{ComponentLibraryType: tt.raw, PreferredComponentFlag: tt.preferred}
		if got := product.LibraryType(); got != tt.want {
			t.Errorf("LibraryType(%q, preferred=%v) = %q, want %q", tt.raw, tt.preferred, got, tt.want)
		}
	}
}

// TestProductBuyable tests the isBuyComponent mapping.
func TestProductBuyable(t *testing.T) {
	tests := []struct {
		raw  string
		want bool
	}{
		{"1", true},
		{"0", false},
		{" 0 ", false},
		{"", true},
	}

	for _, tt := range tests {
		product := &Product{IsBuyComponent: tt.raw}
		if got := product.Buyable(); got != tt.want {
			t.Errorf("Buyable(%q) = %v, want %v", tt.raw, got, tt.want)
		}
		if product.IsBuyComponent != tt.raw {
			t.Errorf("Buyable changed IsBuyComponent to %q", product.IsBuyComponent)
		}
	}
}

func startNumbers(breaks []PriceBreak) []int {
	out := make([]int, 0, len(breaks))
	for _, b := range breaks {
		out = append(out, b.StartNumber)
	}
	return out
}

// TestSortPriceBreaks tests sort order, stability, and input safety.
func TestSortPriceBreaks(t *testing.T) {
	input := []PriceBreak{
		{StartNumber: 50000, EndNumber: -1, ProductPrice: 0.0017},
		{StartNumber: 1000, EndNumber: 49999, ProductPrice: 0.0026},
		{StartNumber: 1, EndNumber: 999, ProductPrice: 0.0033},
		{StartNumber: 1000, EndNumber: 49999, ProductPrice: 0.0025},
	}
	original := slices.Clone(input)

	sorted := SortPriceBreaks(input)

	want := []PriceBreak{
		{StartNumber: 1, EndNumber: 999, ProductPrice: 0.0033},
		{StartNumber: 1000, EndNumber: 49999, ProductPrice: 0.0026},
		{StartNumber: 1000, EndNumber: 49999, ProductPrice: 0.0025},
		{StartNumber: 50000, EndNumber: -1, ProductPrice: 0.0017},
	}
	if !reflect.DeepEqual(sorted, want) {
		t.Errorf("sorted = %+v, want %+v", sorted, want)
	}
	if !reflect.DeepEqual(input, original) {
		t.Errorf("input changed to %+v", input)
	}

	sorted[0].StartNumber = 99
	if input[2].StartNumber != 1 {
		t.Error("result shares memory with input")
	}

	if SortPriceBreaks(nil) != nil {
		t.Error("expected nil for nil input")
	}
	if got := SortPriceBreaks([]PriceBreak{}); got == nil || len(got) != 0 {
		t.Errorf("expected empty non-nil slice, got %#v", got)
	}
}

// TestProductSortedPricesFromFixture uses the unsorted buy prices of a live response.
func TestProductSortedPricesFromFixture(t *testing.T) {
	product := decodeFixture(t, "search_C7593.json")["C7593"]

	rawBuy := startNumbers(product.BuyComponentPrices)
	if slices.IsSorted(rawBuy) {
		t.Fatalf("fixture buy prices are already sorted: %v", rawBuy)
	}

	want := []int{1, 50, 150, 500, 2500, 5000}
	if got := startNumbers(product.SortedBuyComponentPrices()); !reflect.DeepEqual(got, want) {
		t.Errorf("SortedBuyComponentPrices start numbers = %v, want %v", got, want)
	}
	if got := startNumbers(product.SortedComponentPrices()); !reflect.DeepEqual(got, want) {
		t.Errorf("SortedComponentPrices start numbers = %v, want %v", got, want)
	}
	if got := startNumbers(product.BuyComponentPrices); !reflect.DeepEqual(got, rawBuy) {
		t.Errorf("raw BuyComponentPrices changed to %v, want %v", got, rawBuy)
	}

	sortedBuy := product.SortedBuyComponentPrices()
	first, last := float64(sortedBuy[0].ProductPrice), float64(sortedBuy[len(sortedBuy)-1].ProductPrice)
	if first != 0.1158 || last != 0.0626 {
		t.Errorf("sorted buy prices lost their price values: %+v", sortedBuy)
	}
}
