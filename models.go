package jlcpcb

import (
	"cmp"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
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

	// Assembly library and ordering fields. A JSON null decodes to the zero value.
	ComponentLibraryType      string `json:"componentLibraryType"`      // Raw library type: "base" or "expand"
	PreferredComponentFlag    bool   `json:"preferredComponentFlag"`    // True for a preferred extended part
	LossNumber                int    `json:"lossNumber"`                // Attrition: extra parts that JLCPCB adds to an order
	LeastPatchNumber          int    `json:"leastPatchNumber"`          // Minimum placement quantity
	CanPresaleNumber          int    `json:"canPresaleNumber"`          // Quantity open for pre-order (can be negative)
	NoBuyReason               string `json:"noBuyReason"`               // Reason that JLCPCB does not sell the part (empty for null)
	EncapsulationNumber       int    `json:"encapsulationNumber"`       // Parts per reel or package
	PreMinPurchaseNum         int    `json:"preMinPurchaseNum"`         // Minimum pre-order purchase quantity
	ComponentAlternativesCode string `json:"componentAlternativesCode"` // Alternative part code (empty for null)
	AssemblyComponentFlag     bool   `json:"assemblyComponentFlag"`     // Raw assemblyComponentFlag value

	// Media URLs. An "AccessIdUrl" field is a signed URL that expires 30
	// minutes after the response. Download the file soon, and do not store
	// the URL. A JSON null decodes to the zero value.
	ComponentImageUrl          string `json:"componentImageUrl"`          // Image URL (often empty)
	MinImage                   string `json:"minImage"`                   // Small image URL (often empty)
	ProductBigImageAccessIdUrl string `json:"productBigImageAccessIdUrl"` // Signed URL of the large image
	MinImageAccessIdUrl        string `json:"minImageAccessIdUrl"`        // Signed URL of the small image
	DataManualFileAccessIdUrl  string `json:"dataManualFileAccessIdUrl"`  // Signed URL of the datasheet copy that JLCPCB hosts
	DataManualOfficialLink     string `json:"dataManualOfficialLink"`     // Datasheet URL at the manufacturer (often empty)
}

// LibraryType is the JLCPCB assembly library class of a part.
type LibraryType string

const (
	// LibraryTypeBasic is a basic part.
	LibraryTypeBasic LibraryType = "basic"
	// LibraryTypePreferred is a preferred extended part.
	LibraryTypePreferred LibraryType = "preferred"
	// LibraryTypeExtended is an extended part that is not preferred.
	LibraryTypeExtended LibraryType = "extended"
)

// LibraryType returns the assembly library class of the part.
//
// It maps ComponentLibraryType and PreferredComponentFlag as follows:
//
//   - "base" returns LibraryTypeBasic.
//   - "expand" with PreferredComponentFlag true returns LibraryTypePreferred.
//   - "expand" with PreferredComponentFlag false returns LibraryTypeExtended.
//   - An empty or unknown value returns "".
//
// The match ignores case and surrounding white space.
func (p *Product) LibraryType() LibraryType {
	raw := strings.TrimSpace(p.ComponentLibraryType)
	switch {
	case strings.EqualFold(raw, "base"):
		return LibraryTypeBasic
	case strings.EqualFold(raw, "expand"):
		if p.PreferredComponentFlag {
			return LibraryTypePreferred
		}
		return LibraryTypeExtended
	default:
		return ""
	}
}

// Buyable reports whether JLCPCB sells the part.
//
// It returns false only when IsBuyComponent is "0". It returns true for "1".
// It also returns true for an empty value, because an empty value is unknown.
// NoBuyReason often gives the reason when Buyable returns false.
func (p *Product) Buyable() bool {
	return strings.TrimSpace(p.IsBuyComponent) != "0"
}

// ImageURL returns the best image URL of the part, or "" when the part has no
// image. It prefers the large image to the small image, and a signed URL to an
// unsigned URL, because the live API sends the signed URLs.
//
// A signed URL expires 30 minutes after the response. Download the image soon,
// and do not store the URL.
func (p *Product) ImageURL() string {
	for _, candidate := range []string{
		p.ProductBigImageAccessIdUrl,
		p.MinImageAccessIdUrl,
		p.ComponentImageUrl,
		p.MinImage,
	} {
		if candidate = strings.TrimSpace(candidate); candidate != "" {
			return candidate
		}
	}
	return ""
}

// DatasheetURLs returns the datasheet URLs of the part, best first, without
// empty or duplicate URLs. The order is DataManualUrl (usually an LCSC PDF),
// DataManualFileAccessIdUrl (a signed URL of the copy that JLCPCB hosts) and
// DataManualOfficialLink (the manufacturer page). It returns nil when the part
// has no datasheet URL.
//
// The signed URL expires 30 minutes after the response. Download the file
// soon, and do not store the URL.
func (p *Product) DatasheetURLs() []string {
	var urls []string
	for _, candidate := range []string{
		p.DataManualUrl,
		p.DataManualFileAccessIdUrl,
		p.DataManualOfficialLink,
	} {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" || slices.Contains(urls, candidate) {
			continue
		}
		urls = append(urls, candidate)
	}
	return urls
}

// SortedComponentPrices returns a copy of ComponentPrices sorted by StartNumber.
// It does not change ComponentPrices.
func (p *Product) SortedComponentPrices() []PriceBreak {
	return SortPriceBreaks(p.ComponentPrices)
}

// SortedBuyComponentPrices returns a copy of BuyComponentPrices sorted by StartNumber.
// It does not change BuyComponentPrices.
//
// The live API can send BuyComponentPrices in a random order.
func (p *Product) SortedBuyComponentPrices() []PriceBreak {
	return SortPriceBreaks(p.BuyComponentPrices)
}

// SortPriceBreaks returns a copy of breaks sorted by StartNumber in ascending order.
//
// The sort is stable: breaks with the same StartNumber keep their order.
// It does not change breaks. It returns nil when breaks is nil.
func SortPriceBreaks(breaks []PriceBreak) []PriceBreak {
	sorted := slices.Clone(breaks)
	slices.SortStableFunc(sorted, func(a, b PriceBreak) int {
		return cmp.Compare(a.StartNumber, b.StartNumber)
	})
	return sorted
}

// GetProductURL returns the JLCPCB product page URL.
func (p *Product) GetProductURL() string {
	if p.UrlSuffix != "" {
		return fmt.Sprintf("https://jlcpcb.com/partdetail/%s", p.UrlSuffix)
	}
	return fmt.Sprintf("https://jlcpcb.com/partdetail/%s", p.ComponentCode)
}

// FilterAttribute represents a component attribute filter.
type FilterAttribute struct {
	Name  string `json:"attributeName"`
	Value string `json:"attributeValue"`
}
