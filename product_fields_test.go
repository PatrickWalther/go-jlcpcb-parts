package jlcpcb

import (
	"encoding/json"
	"reflect"
	"testing"
)

// The rows of search_v2_rows.json come from live v2 search responses of
// 2026-10-02. The signed URL parameters are replaced with fixed strings.

// rowFields holds the v2 row fields that the refreshed fixture test checks.
type rowFields struct {
	ComponentProductType    PCBAEligibility
	EstimateDate            FlexString
	InitialPrice            FlexFloat64
	AllowPostFlag           bool
	MergedComponentCode     string
	ReplaceUrlSuffix        string
	CanPresaleNumber        int
	MinPurchaseNum          int
	PreMinPurchaseNum       int
	IsBuyComponent          string
	ProductBigImageAccessId string
	MinImageAccessId        string
	DataManualFileAccessId  string
}

func rowFieldsOf(p *Product) rowFields {
	return rowFields{
		ComponentProductType:    p.ComponentProductType,
		EstimateDate:            p.EstimateDate,
		InitialPrice:            p.InitialPrice,
		AllowPostFlag:           p.AllowPostFlag,
		MergedComponentCode:     p.MergedComponentCode,
		ReplaceUrlSuffix:        p.ReplaceUrlSuffix,
		CanPresaleNumber:        p.CanPresaleNumber,
		MinPurchaseNum:          p.MinPurchaseNum,
		PreMinPurchaseNum:       p.PreMinPurchaseNum,
		IsBuyComponent:          p.IsBuyComponent,
		ProductBigImageAccessId: p.ProductBigImageAccessId,
		MinImageAccessId:        p.MinImageAccessId,
		DataManualFileAccessId:  p.DataManualFileAccessId,
	}
}

func TestProductDecodeV2RowFields(t *testing.T) {
	tests := []struct {
		name   string
		code   string
		want   rowFields
		parent string
		leaf   string
	}{
		{
			name: "basic part with a lead time",
			code: "C1525",
			want: rowFields{
				EstimateDate:      "3",
				InitialPrice:      0.0045,
				AllowPostFlag:     true,
				CanPresaleNumber:  15731372,
				MinPurchaseNum:    1,
				PreMinPurchaseNum: 2654,
				IsBuyComponent:    "1",
			},
			parent: "Capacitors",
			leaf:   mlccLeaf,
		},
		{
			name: "standard PCBA only",
			code: "C89140",
			want: rowFields{
				ComponentProductType: PCBAEligibilityStandardOnly,
				EstimateDate:         "33",
				InitialPrice:         0.0012,
				AllowPostFlag:        true,
				CanPresaleNumber:     8,
				MinPurchaseNum:       1,
				PreMinPurchaseNum:    8203,
				IsBuyComponent:       "1",
			},
			parent: "Capacitors",
			leaf:   mlccLeaf,
		},
		{
			name: "part that JLCPCB does not sell, with a merge code",
			code: "C2961140",
			want: rowFields{
				InitialPrice:        1.5217,
				AllowPostFlag:       true,
				MergedComponentCode: "C2040",
				ReplaceUrlSuffix:    "RaspberryPi-RP2040/C2040",
				MinPurchaseNum:      6,
				PreMinPurchaseNum:   6,
				IsBuyComponent:      "0",
			},
			parent: "Embedded Processors & Controllers",
			leaf:   "Microcontrollers (MCU/MPU/SOC)",
		},
		{
			name: "negative stock limit",
			code: "C17354991",
			want: rowFields{
				InitialPrice:      0.0041,
				AllowPostFlag:     true,
				CanPresaleNumber:  -3654,
				MinPurchaseNum:    2201,
				PreMinPurchaseNum: 2201,
				IsBuyComponent:    "1",
			},
			parent: "Capacitors",
			leaf:   mlccLeaf,
		},
		{
			name: "active part with a merge code",
			code: "C8692",
			want: rowFields{
				InitialPrice:        0.1494,
				AllowPostFlag:       true,
				MergedComponentCode: "C26615",
				ReplaceUrlSuffix:    "27360-10_K_5/C26615",
				CanPresaleNumber:    128639,
				MinPurchaseNum:      1,
				PreMinPurchaseNum:   75,
				IsBuyComponent:      "1",
			},
			parent: "Resistors",
			leaf:   "Resistor Networks, Arrays",
		},
	}

	products := decodeFixture(t, "search_v2_rows.json")
	if len(products) != len(tests) {
		t.Fatalf("fixture has %d products, want %d", len(products), len(tests))
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			product, ok := products[tt.code]
			if !ok {
				t.Fatalf("fixture has no product %s", tt.code)
			}
			if got := rowFieldsOf(&product); got != tt.want {
				t.Errorf("decoded fields mismatch\n got: %+v\nwant: %+v", got, tt.want)
			}
			if parent, leaf := product.Category(); parent != tt.parent || leaf != tt.leaf {
				t.Errorf("Category() = (%q, %q), want (%q, %q)", parent, leaf, tt.parent, tt.leaf)
			}
		})
	}
}

func TestSearchResponseDecodePages(t *testing.T) {
	var wrapper searchResponseWrapper
	if err := json.Unmarshal(loadFixture(t, "search_v2_rows.json"), &wrapper); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	resp := wrapper.Data.ComponentPageInfo
	if resp.TotalCount != 15991 || resp.Pages != 160 || resp.PageSize != 100 || resp.Page != 1 {
		t.Errorf("page info = total %d, pages %d, size %d, page %d; want 15991, 160, 100, 1",
			resp.TotalCount, resp.Pages, resp.PageSize, resp.Page)
	}
	if wrapper.Data.SortAndCountVoList != nil {
		t.Errorf("searchType 2 response has categories: %+v", wrapper.Data.SortAndCountVoList)
	}
}

// TestProductDecodeNewFieldsNonZero checks the JSON tag of each new field
// with values that the fixture rows do not have, and a cache round trip.
func TestProductDecodeNewFieldsNonZero(t *testing.T) {
	data := []byte(`{
		"componentCode": "C1",
		"componentProductType": 1,
		"estimateDate": 12,
		"initialPrice": "0.25",
		"allowPostFlag": true,
		"mergedComponentCode": "C2",
		"replaceUrlSuffix": "Brand-Part/C2",
		"productBigImageAccessId": "8552476004850417664",
		"minImageAccessId": "8552476005450338304",
		"dataManualFileAccessId": "8579707269996871680"
	}`)
	want := rowFields{
		ComponentProductType:    PCBAEligibilityEconomicOnly,
		EstimateDate:            "12",
		InitialPrice:            0.25,
		AllowPostFlag:           true,
		MergedComponentCode:     "C2",
		ReplaceUrlSuffix:        "Brand-Part/C2",
		ProductBigImageAccessId: "8552476004850417664",
		MinImageAccessId:        "8552476005450338304",
		DataManualFileAccessId:  "8579707269996871680",
	}

	var product Product
	if err := json.Unmarshal(data, &product); err != nil {
		t.Fatalf("decode new fields: %v", err)
	}
	if got := rowFieldsOf(&product); got != want {
		t.Errorf("decoded fields mismatch\n got: %+v\nwant: %+v", got, want)
	}

	encoded, err := json.Marshal(product)
	if err != nil {
		t.Fatalf("encode product: %v", err)
	}
	var roundTrip Product
	if err := json.Unmarshal(encoded, &roundTrip); err != nil {
		t.Fatalf("decode encoded product: %v", err)
	}
	if !reflect.DeepEqual(roundTrip, product) {
		t.Errorf("round trip changed the product\n got: %+v\nwant: %+v", roundTrip, product)
	}
}

func TestProductDecodeNewFieldsNull(t *testing.T) {
	data := []byte(`{
		"componentCode": "C1",
		"componentProductType": null,
		"estimateDate": null,
		"initialPrice": null,
		"allowPostFlag": null,
		"mergedComponentCode": null,
		"replaceUrlSuffix": null,
		"productBigImageAccessId": null,
		"minImageAccessId": null,
		"dataManualFileAccessId": null
	}`)
	var product Product
	if err := json.Unmarshal(data, &product); err != nil {
		t.Fatalf("decode null fields: %v", err)
	}
	if got := rowFieldsOf(&product); got != (rowFields{}) {
		t.Errorf("null fields mismatch: %+v", got)
	}
	if days, ok := product.LeadTimeDays(); ok {
		t.Errorf("LeadTimeDays() = %d, true; want false for null", days)
	}
}

func TestFlexStringUnmarshal(t *testing.T) {
	tests := []struct {
		data    string
		want    FlexString
		wantErr bool
	}{
		{`"10"`, "10", false},
		{`""`, "", false},
		{`10`, "10", false},
		{`2.5`, "2.5", false},
		{`-3`, "-3", false},
		{`null`, "", false},
		{`true`, "", true},
		{`[]`, "", true},
		{`{}`, "", true},
	}
	for _, tt := range tests {
		got := FlexString("old")
		err := json.Unmarshal([]byte(tt.data), &got)
		if tt.wantErr {
			if err == nil {
				t.Errorf("Unmarshal(%s) = %q, want an error", tt.data, got)
			}
			continue
		}
		if err != nil || got != tt.want {
			t.Errorf("Unmarshal(%s) = %q, %v; want %q", tt.data, got, err, tt.want)
		}
	}
}

func TestProductLeadTimeDays(t *testing.T) {
	tests := []struct {
		estimate FlexString
		days     int
		ok       bool
	}{
		{"3", 3, true},
		{" 33 ", 33, true},
		{"0", 0, true},
		{"", 0, false},
		{"2.5", 0, false},
		{"-1", 0, false},
		{"soon", 0, false},
	}
	for _, tt := range tests {
		product := &Product{EstimateDate: tt.estimate}
		days, ok := product.LeadTimeDays()
		if days != tt.days || ok != tt.ok {
			t.Errorf("LeadTimeDays(%q) = %d, %v; want %d, %v", tt.estimate, days, ok, tt.days, tt.ok)
		}
	}

	products := decodeFixture(t, "search_v2_rows.json")
	c1525 := products["C1525"]
	if days, ok := c1525.LeadTimeDays(); days != 3 || !ok {
		t.Errorf("C1525 LeadTimeDays() = %d, %v; want 3, true", days, ok)
	}
	pure := products["C17354991"]
	if _, ok := pure.LeadTimeDays(); ok {
		t.Error("C17354991 has no estimateDate, so LeadTimeDays() must return false")
	}
}

func TestPCBAEligibility(t *testing.T) {
	tests := []struct {
		value    PCBAEligibility
		economic bool
		standard bool
	}{
		{PCBAEligibilityBoth, true, true},
		{PCBAEligibilityEconomicOnly, true, false},
		{PCBAEligibilityStandardOnly, false, true},
		{PCBAEligibility(3), false, false},
		{PCBAEligibility(-1), false, false},
	}
	for _, tt := range tests {
		if got := tt.value.AllowsEconomic(); got != tt.economic {
			t.Errorf("PCBAEligibility(%d).AllowsEconomic() = %v, want %v", tt.value, got, tt.economic)
		}
		if got := tt.value.AllowsStandard(); got != tt.standard {
			t.Errorf("PCBAEligibility(%d).AllowsStandard() = %v, want %v", tt.value, got, tt.standard)
		}
	}
}

func TestProductCategoryOrder(t *testing.T) {
	product := &Product{FirstSortName: mlccLeaf, SecondSortName: "Capacitors"}
	parent, leaf := product.Category()
	if parent != "Capacitors" || leaf != mlccLeaf {
		t.Errorf("Category() = (%q, %q), want (Capacitors, %q)", parent, leaf, mlccLeaf)
	}
	if product.FirstSortName != mlccLeaf || product.SecondSortName != "Capacitors" {
		t.Error("Category() changed the raw fields")
	}
}
