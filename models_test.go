package jlcpcb

import (
	"encoding/json"
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

	expectedURL := "https://jlcpcb.com/parts/details/6597989-MPM3506AGQVZ/C5676715"
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

	expected := "https://jlcpcb.com/parts/details/C123456"
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
		PageNumber: 1,
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

	if resp.PageNumber != 1 {
		t.Errorf("expected page number 1, got %d", resp.PageNumber)
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
		Keyword:     "MPM3506",
		CurrentPage: 1,
		PageSize:    20,
		IsAvailable: true,
	}

	if req.Keyword != "MPM3506" {
		t.Errorf("expected keyword MPM3506, got %s", req.Keyword)
	}

	if req.PageSize != 20 {
		t.Errorf("expected page size 20, got %d", req.PageSize)
	}

	if !req.IsAvailable {
		t.Error("expected IsAvailable to be true")
	}
}
