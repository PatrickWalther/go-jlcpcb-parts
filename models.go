package jlcpcb

import (
	"bytes"
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

// FlexString handles JSON values that may be either a string or a number.
// A number keeps its JSON text, for example 10 decodes to "10". A JSON null
// decodes to "".
type FlexString string

// UnmarshalJSON implements json.Unmarshaler for FlexString.
func (s *FlexString) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if string(trimmed) == "null" {
		*s = ""
		return nil
	}
	var str string
	if err := json.Unmarshal(trimmed, &str); err == nil {
		*s = FlexString(str)
		return nil
	}
	var num json.Number
	if err := json.Unmarshal(trimmed, &num); err == nil {
		*s = FlexString(num.String())
		return nil
	}
	return fmt.Errorf("cannot unmarshal %s into FlexString", string(data))
}

// PCBAEligibility tells which PCBA types accept a part
// (componentProductType).
type PCBAEligibility int

const (
	// PCBAEligibilityBoth means that Economic and Standard PCBA accept the part.
	PCBAEligibilityBoth PCBAEligibility = 0
	// PCBAEligibilityEconomicOnly means that only Economic PCBA accepts the part.
	PCBAEligibilityEconomicOnly PCBAEligibility = 1
	// PCBAEligibilityStandardOnly means that only Standard PCBA accepts the part.
	PCBAEligibilityStandardOnly PCBAEligibility = 2
)

// AllowsEconomic reports whether Economic PCBA accepts the part. It returns
// false for an unknown value.
func (e PCBAEligibility) AllowsEconomic() bool {
	return e == PCBAEligibilityBoth || e == PCBAEligibilityEconomicOnly
}

// AllowsStandard reports whether Standard PCBA accepts the part. It returns
// false for an unknown value.
func (e PCBAEligibility) AllowsStandard() bool {
	return e == PCBAEligibilityBoth || e == PCBAEligibilityStandardOnly
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
	FirstSortName            string       `json:"firstSortName"`            // Leaf category in a search row (see Category)
	SecondSortName           string       `json:"secondSortName"`           // Parent category in a search row (see Category)
	IsBuyComponent           string       `json:"isBuyComponent"`           // Can be purchased
	UrlSuffix                string       `json:"urlSuffix"`                // URL suffix for webpage
	LcscGoodsUrl             string       `json:"lcscGoodsUrl"`             // LCSC product URL

	// Assembly library and ordering fields. A JSON null decodes to the zero value.
	ComponentLibraryType      string `json:"componentLibraryType"`      // Raw library type: "base" or "expand"
	PreferredComponentFlag    bool   `json:"preferredComponentFlag"`    // True for a preferred extended part
	LossNumber                int    `json:"lossNumber"`                // Base term of the attrition (extra parts) of an assembly order
	LeastPatchNumber          int    `json:"leastPatchNumber"`          // Minimum placement quantity
	CanPresaleNumber          int    `json:"canPresaleNumber"`          // Largest parts order from stock; a larger order is a pre-order (can be negative)
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

	// PCBA, pricing and replacement fields of a v2 search row. A JSON null
	// decodes to the zero value.
	ComponentProductType PCBAEligibility `json:"componentProductType"` // PCBA types that accept the part
	EstimateDate         FlexString      `json:"estimateDate"`         // Estimated lead time in days (often empty, see LeadTimeDays)
	InitialPrice         FlexFloat64     `json:"initialPrice"`         // ComponentPrices tier price at MinPurchaseNum
	AllowPostFlag        bool            `json:"allowPostFlag"`        // True when the customer can consign the part to JLCPCB
	MergedComponentCode  string          `json:"mergedComponentCode"`  // Merge or alternative part code; active parts have it too, so it is not an EOL marker
	ReplaceUrlSuffix     string          `json:"replaceUrlSuffix"`     // Part page URL suffix of MergedComponentCode, for example "RaspberryPi-RP2040/C2040"

	// Stable file access ids. A file access id has no expiry time, unlike a
	// signed URL. The v2 search sends null for these ids today. The classic
	// search and the detail records send them. A JSON null decodes to "".
	// FileURL gives the download URL of an id, and FileService.Open
	// downloads the file.
	ProductBigImageAccessId string `json:"productBigImageAccessId"` // File access id of the large image
	MinImageAccessId        string `json:"minImageAccessId"`        // File access id of the small image
	DataManualFileAccessId  string `json:"dataManualFileAccessId"`  // File access id of the datasheet copy that JLCPCB hosts
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
// unsigned URL, because the live API sends the signed URLs. It skips a URL
// without a file name, for example the LCSC folder URL
// "https://assets.lcsc.com/images/lcsc/900x900/" that some records send.
//
// A signed URL expires 30 minutes after the response. Download the image soon,
// and do not store the URL. StableImageURL and StableThumbnailURL prefer the
// URLs that have no expiry time.
func (p *Product) ImageURL() string {
	for _, candidate := range []string{
		p.ProductBigImageAccessIdUrl,
		p.MinImageAccessIdUrl,
		p.ComponentImageUrl,
		p.MinImage,
	} {
		if candidate = strings.TrimSpace(candidate); candidate != "" && hasFileName(candidate) {
			return candidate
		}
	}
	return ""
}

// DatasheetURLs returns the datasheet URLs of the part, best first, without
// empty or duplicate URLs. The order is:
//
//  1. FileURL(DataManualFileAccessId): the stable URL of the PDF copy that
//     JLCPCB hosts.
//  2. DataManualFileAccessIdUrl: a signed URL of the same copy. It expires 30
//     or 60 minutes after the response.
//  3. DataManualUrl: usually an LCSC URL. A "www.lcsc.com/datasheet/" URL
//     gives an HTML viewer page, not a PDF.
//  4. DataManualOfficialLink: the manufacturer page.
//
// It returns nil when the part has no datasheet URL. Download a signed URL
// soon, and do not store it.
func (p *Product) DatasheetURLs() []string {
	var urls []string
	for _, candidate := range []string{
		FileURL(p.DataManualFileAccessId),
		p.DataManualFileAccessIdUrl,
		p.DataManualUrl,
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

// Category returns the parent and leaf category names of a search row, for
// example "Capacitors" and "Multilayer Ceramic Capacitors MLCC - SMD/SMT".
//
// A search row sends the leaf in firstSortName and the parent in
// secondSortName. A search request and a detail record use the reverse
// order. Category returns the names in the true order.
func (p *Product) Category() (parent, leaf string) {
	return p.SecondSortName, p.FirstSortName
}

// LeadTimeDays returns the estimated lead time in days from EstimateDate.
// It returns false when EstimateDate is empty or is not a whole number of
// days that is 0 or more.
//
// Only v2 search rows send EstimateDate, and only some rows with stock or
// with a negative CanPresaleNumber. The value is null on every pre-order
// row with stock 0, so it is not a lead time for such a part. The JLCPCB
// part page shows it only for an order larger than CanPresaleNumber.
func (p *Product) LeadTimeDays() (int, bool) {
	days, err := strconv.Atoi(strings.TrimSpace(string(p.EstimateDate)))
	if err != nil || days < 0 {
		return 0, false
	}
	return days, true
}

// GetProductURL returns the JLCPCB product page URL.
func (p *Product) GetProductURL() string {
	if p.UrlSuffix != "" {
		return fmt.Sprintf("https://jlcpcb.com/partdetail/%s", p.UrlSuffix)
	}
	return fmt.Sprintf("https://jlcpcb.com/partdetail/%s", p.ComponentCode)
}

// FilterAttribute represents a component attribute filter with one value.
//
// Deprecated: Use AttributeFilter, which accepts more than one value. The
// JSON tags of FilterAttribute are not the wire shape of a search.
type FilterAttribute struct {
	Name  string `json:"attributeName"`
	Value string `json:"attributeValue"`
}
