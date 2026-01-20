package jlcpcb

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// Attribute represents a product specification/parameter.
type Attribute struct {
	Name  string `json:"attribute_name_en"`    // Attribute name
	Value string `json:"attribute_value_name"` // Attribute value
}

// FlexFloat64 handles JSON values that may be either a number or a string.
type FlexFloat64 float64

// UnmarshalJSON implements json.Unmarshaler for FlexFloat64.
func (f *FlexFloat64) UnmarshalJSON(data []byte) error {
	// Try as number first
	var num float64
	if err := json.Unmarshal(data, &num); err == nil {
		*f = FlexFloat64(num)
		return nil
	}
	// Try as string
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		num, err := strconv.ParseFloat(str, 64)
		if err != nil {
			return fmt.Errorf("cannot parse %q as float64: %w", str, err)
		}
		*f = FlexFloat64(num)
		return nil
	}
	return fmt.Errorf("cannot unmarshal %s into FlexFloat64", string(data))
}

// PriceBreak represents a quantity-based price tier.
type PriceBreak struct {
	StartNumber  int         `json:"startNumber"`  // Minimum quantity for this tier
	EndNumber    int         `json:"endNumber"`    // Maximum quantity for this tier (-1 means unlimited)
	ProductPrice FlexFloat64 `json:"productPrice"` // Price per unit in USD
}

// Product represents a JLCPCB electronic component.
type Product struct {
	ComponentID              int          `json:"componentId"`              // Component ID
	ComponentCode            string       `json:"componentCode"`            // JLCPCB part code (e.g., C5676715)
	ComponentModelEn         string       `json:"componentModelEn"`         // Manufacturer part number
	ComponentBrandEn         string       `json:"componentBrandEn"`         // Manufacturer name
	ComponentTypeEn          string       `json:"componentTypeEn"`          // Component type
	ComponentName            string       `json:"componentName"`            // Component name
	ComponentSpecificationEn string       `json:"componentSpecificationEn"` // Package/footprint
	StockCount               int          `json:"stockCount"`               // Stock quantity
	MinPurchaseNum           int          `json:"minPurchaseNum"`           // Minimum order quantity
	ComponentPrices          []PriceBreak `json:"componentPrices"`          // Price breaks
	BuyComponentPrices       []PriceBreak `json:"buyComponentPrices"`       // Buy price breaks
	Attributes               []Attribute  `json:"attributes"`               // Specifications
	DataManualUrl            string       `json:"dataManualUrl"`            // Datasheet URL
	Describe                 string       `json:"describe"`                 // Description
	FirstSortName            string       `json:"firstSortName"`            // Primary category
	SecondSortName           string       `json:"secondSortName"`           // Secondary category
	IsBuyComponent           string       `json:"isBuyComponent"`           // Can be purchased
	UrlSuffix                string       `json:"urlSuffix"`                // URL suffix for webpage
	LcscGoodsUrl             string       `json:"lcscGoodsUrl"`             // LCSC product URL
}

// GetProductURL returns the JLCPCB product page URL.
func (p *Product) GetProductURL() string {
	if p.UrlSuffix != "" {
		return fmt.Sprintf("https://jlcpcb.com/parts/details/%s", p.UrlSuffix)
	}
	return fmt.Sprintf("https://jlcpcb.com/parts/details/%s", p.ComponentCode)
}

// FilterAttribute represents a component attribute filter.
type FilterAttribute struct {
	Name  string `json:"attributeName"`
	Value string `json:"attributeValue"`
}

// SearchRequest contains parameters for a product search.
type SearchRequest struct {
	Keyword     string
	CurrentPage int
	PageSize    int
	IsAvailable bool
	// Advanced filters
	PresaleType     string            // "stock", "buy", "post", or empty for all
	ComponentType   string            // "base" or "expand"
	Attributes      []FilterAttribute // Filter by attributes
	Brands          []string          // Filter by brand names
	StockOnly       bool              // Only show in-stock items
	PreferredOnly   bool              // Only show preferred components
	SortBy          string            // Primary sort field
	SortBySecondary string            // Secondary sort field
}

// SearchResponse contains the results of a product search.
type SearchResponse struct {
	Products   []Product `json:"list"`
	TotalCount int       `json:"total"`
	PageSize   int       `json:"pageSize"`
	PageNumber int       `json:"pageNum"`
}

// productSearchWrapper matches the JLCPCB API response structure.
type productSearchWrapper struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		ComponentPageInfo SearchResponse `json:"componentPageInfo"`
	} `json:"data"`
}

// ProductResponse contains a single product response.
type ProductResponse struct {
	Code    int     `json:"code"`
	Msg     string  `json:"msg"`
	Success bool    `json:"success"`
	Result  Product `json:"result"`
}
