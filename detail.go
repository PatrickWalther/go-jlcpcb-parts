package jlcpcb

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
)

const (
	// componentDetailPath is the path of the exact part detail below the API
	// root. The add-to-cart dialog of the JLCPCB part pages calls it.
	componentDetailPath = "/overseas-pcb-order/v1/smtComponentShopCart/getComponentDetail"
	// compareDetailsPath is the path of the batch part detail below the API
	// root.
	compareDetailsPath = smtGoodPath + "/compareComponentDetails"
	// compareChunkSize is the largest number of ids in one batch detail
	// request. A live request with 500 ids returned 500 records, and no
	// server limit was found. A response of 250 records is about 2.6 MB
	// before gzip.
	compareChunkSize = 250
)

// ComponentDetail is the detail record of one JLCPCB part.
// ProductService.Detail and ProductService.DetailsByIDs return it.
//
// A detail record sends the category names in the true order: the parent in
// firstSortName and the leaf in secondSortName. A search row uses the
// reverse order. A detail record has no preferred flag, no lead time and no
// merge code. Use Product to get the record as a Product.
//
// A JSON null decodes to the zero value.
type ComponentDetail struct {
	LCSCComponentID          int64       `json:"lcscComponentId"`          // Numeric part id. It equals Product.ComponentID and the LCSC productId
	ComponentCode            string      `json:"componentCode"`            // JLCPCB part code (e.g., C1525)
	ComponentModelEn         string      `json:"componentModelEn"`         // Manufacturer part number
	ComponentBrandEn         string      `json:"componentBrandEn"`         // Manufacturer name
	ComponentName            string      `json:"componentName"`            // Component name
	ComponentSpecificationEn string      `json:"componentSpecificationEn"` // Package/footprint
	Describe                 string      `json:"describe"`                 // Description
	Attributes               []Attribute `json:"attributes"`               // Specifications
	ComponentStatus          string      `json:"componentStatus"`          // "yes" for a listed part. Detail also returns unlisted parts ("no")

	// Category names and numeric category ids. The search ignores the
	// numeric ids. Use the names in a Category filter, and the ids in a
	// FacetRequest.
	ParentCategory   string `json:"firstSortName"`    // Parent category (e.g., "Capacitors")
	LeafCategory     string `json:"secondSortName"`   // Leaf category (e.g., "Multilayer Ceramic Capacitors MLCC - SMD/SMT")
	ParentCategoryID int    `json:"firstTypeNameId"`  // Numeric id of the parent category (e.g., 2)
	LeafCategoryID   int    `json:"secondTypeNameId"` // Numeric id of the leaf category (e.g., 2929)

	// Assembly fields.
	ComponentLibraryType       string          `json:"componentLibraryType"`       // Raw library type: "base" or "expand". A detail record has no preferred flag
	AssemblyComponentFlag      bool            `json:"assemblyComponentFlag"`      // Raw assemblyComponentFlag value
	AssemblyProcess            string          `json:"assemblyProcess"`            // Assembly process: "SMT" or "THT"
	AssemblyMode               string          `json:"assemblyMode"`               // Assembly method (e.g., "smtWeld" or "manualWeld" for hand soldering)
	ComponentProductType       PCBAEligibility `json:"componentProductType"`       // PCBA types that accept the part
	XrayFlag                   bool            `json:"xrayFlag"`                   // True when the part needs an X-ray inspection (e.g., a BGA)
	SpecialComponentFee        FlexFloat64     `json:"specialComponentFee"`        // Extra assembly fee of the part in USD (0 for most parts, the fee basis is not known)
	NeedAuditFlag              bool            `json:"needAuditFlag"`              // Raw needAuditFlag value (false for most parts)
	OrderInstructionEnglish    string          `json:"orderInstructionEnglish"`    // Order instruction text (empty for most parts)
	ComponentDesignator        string          `json:"componentDesignator"`        // Designator prefix (e.g., "C" or "U"). It is not reliable
	MoistureSensitivityLevelEn string          `json:"moistureSensitivityLevelEn"` // Moisture sensitivity level (e.g., "MSL 1")
	EccnCode                   string          `json:"eccnCode"`                   // Export control class (e.g., "EAR99")

	// Stock and ordering fields.
	StockCount          int          `json:"stockCount"`          // Stock quantity
	CanPresaleNumber    int          `json:"canPresaleNumber"`    // Largest parts order from stock. A detail record sends 0 where a search row sends a negative value
	MinPurchaseNum      int          `json:"minPurchaseNum"`      // Minimum order quantity
	PreMinPurchaseNum   int          `json:"preMinPurchaseNum"`   // Minimum pre-order purchase quantity
	InitialPrice        FlexFloat64  `json:"initialPrice"`        // Prices tier price at MinPurchaseNum
	Prices              []PriceBreak `json:"prices"`              // Stock price ladder, sorted by StartNumber
	BuyPrices           []PriceBreak `json:"buyPrices"`           // Pre-order price ladder, sorted by StartNumber (nil from Detail)
	LossNumber          int          `json:"lossNumber"`          // Base term of the attrition (extra parts) of an assembly order
	LeastPatchNumber    int          `json:"leastPatchNumber"`    // Minimum placement quantity
	EncapsulationNumber int          `json:"encapsulationNumber"` // Parts per reel, tube or package
	EncapsulationUnit   string       `json:"encapsulationUnit"`   // Unit of EncapsulationNumber (e.g., "PCS")
	WarehouseCode       string       `json:"warehouseCode"`       // Warehouse code (e.g., "sz")
	IsBuyComponent      string       `json:"isBuyComponent"`      // "0" when JLCPCB does not sell the part
	NoBuyReason         string       `json:"noBuyReason"`         // Reason that JLCPCB does not sell the part (empty for null)
	AllowPostFlag       bool         `json:"allowPostFlag"`       // True when the customer can consign the part to JLCPCB

	// Replacement fields. JLCPCB sets them on some parts that it no longer
	// sells.
	ComponentAlternativesCode   string `json:"componentAlternativesCode"`   // Part code of the replacement (e.g., "C2040")
	AlternativesLCSCComponentID int64  `json:"alternativesLcscComponentId"` // Numeric part id of the replacement
	ReplaceURLSuffix            string `json:"replaceUrlSuffix"`            // Part page URL suffix of the replacement

	// File access ids and file URLs. A file access id has no expiry time.
	// A signed URL expires after 30 minutes (Detail) or 60 minutes
	// (DetailsByIDs). Download the file soon, and do not store a signed URL.
	// FileURL gives the download URL of an id. StableImageURL,
	// StableThumbnailURL and StableDatasheetURL choose the best URL.
	ProductBigImageAccessID  string `json:"productBigImageAccessId"`    // File access id of the large image (empty from Detail)
	MinImageAccessID         string `json:"minImageAccessId"`           // File access id of the small image (empty from Detail)
	DataManualFileAccessID   string `json:"dataManualFileAccessId"`     // File access id of the datasheet copy that JLCPCB hosts (empty from Detail)
	ProductBigImageSignedURL string `json:"productBigImageAccessIdUrl"` // Signed URL of the large image
	MinImageSignedURL        string `json:"minImageAccessIdUrl"`        // Signed URL of the small image
	DataManualFileSignedURL  string `json:"dataManualFileAccessIdUrl"`  // Signed URL of the datasheet copy that JLCPCB hosts
	ComponentImageURL        string `json:"componentImageUrl"`          // Image URL (often empty)
	MinImageURL              string `json:"minImage"`                   // Small image URL (often empty)
	DataManualURL            string `json:"dataManualUrl"`              // Datasheet URL (usually an LCSC PDF)
	DataManualOfficialLink   string `json:"dataManualOfficialLink"`     // Datasheet URL at the manufacturer (often empty)
	LCSCGoodsURL             string `json:"lcscGoodsUrl"`               // LCSC product URL

	// URLSuffix is the part page URL suffix. Only DetailsByIDs sets it. The
	// batch response sends it next to the record. Do not parse the part id
	// from it: most suffixes do not start with the id.
	URLSuffix string `json:"urlSuffix"`
}

// Category returns the parent and leaf category names. It returns the same
// names as Product.Category of the same part.
func (d *ComponentDetail) Category() (parent, leaf string) {
	return d.ParentCategory, d.LeafCategory
}

// Product returns the detail record as a Product, so that the Product
// methods (for example PartsOrderQuote and LibraryType) work on it.
//
// Product copies each field that both types have. It maps these fields:
//
//   - Prices to ComponentPrices, and BuyPrices to BuyComponentPrices.
//   - LeafCategory to FirstSortName, and ParentCategory to SecondSortName.
//     This is the order of a search row, so Product.Category returns the
//     same names as ComponentDetail.Category.
//   - LCSCComponentID to ComponentID.
//
// A detail record has no preferred flag, no lead time and no merge code.
// Thus PreferredComponentFlag is false, and LibraryType returns
// LibraryTypeExtended for a preferred extended part. EstimateDate and
// MergedComponentCode are empty. Use a search row to get these fields.
//
// Product copies the slices, so a change to the Product does not change d.
func (d *ComponentDetail) Product() Product {
	return Product{
		ComponentID:              int(d.LCSCComponentID),
		ComponentCode:            d.ComponentCode,
		ComponentModelEn:         d.ComponentModelEn,
		ComponentBrandEn:         d.ComponentBrandEn,
		ComponentName:            d.ComponentName,
		ComponentSpecificationEn: d.ComponentSpecificationEn,
		StockCount:               d.StockCount,
		MinPurchaseNum:           d.MinPurchaseNum,
		ComponentPrices:          slices.Clone(d.Prices),
		BuyComponentPrices:       slices.Clone(d.BuyPrices),
		Attributes:               slices.Clone(d.Attributes),
		DataManualUrl:            d.DataManualURL,
		Describe:                 d.Describe,
		FirstSortName:            d.LeafCategory,
		SecondSortName:           d.ParentCategory,
		IsBuyComponent:           d.IsBuyComponent,
		UrlSuffix:                d.URLSuffix,
		LcscGoodsUrl:             d.LCSCGoodsURL,

		ComponentLibraryType:      d.ComponentLibraryType,
		LossNumber:                d.LossNumber,
		LeastPatchNumber:          d.LeastPatchNumber,
		CanPresaleNumber:          d.CanPresaleNumber,
		NoBuyReason:               d.NoBuyReason,
		EncapsulationNumber:       d.EncapsulationNumber,
		PreMinPurchaseNum:         d.PreMinPurchaseNum,
		ComponentAlternativesCode: d.ComponentAlternativesCode,
		AssemblyComponentFlag:     d.AssemblyComponentFlag,

		ComponentImageUrl:          d.ComponentImageURL,
		MinImage:                   d.MinImageURL,
		ProductBigImageAccessIdUrl: d.ProductBigImageSignedURL,
		MinImageAccessIdUrl:        d.MinImageSignedURL,
		DataManualFileAccessIdUrl:  d.DataManualFileSignedURL,
		DataManualOfficialLink:     d.DataManualOfficialLink,

		ComponentProductType: d.ComponentProductType,
		InitialPrice:         d.InitialPrice,
		AllowPostFlag:        d.AllowPostFlag,
		ReplaceUrlSuffix:     d.ReplaceURLSuffix,

		ProductBigImageAccessId: d.ProductBigImageAccessID,
		MinImageAccessId:        d.MinImageAccessID,
		DataManualFileAccessId:  d.DataManualFileAccessID,
	}
}

// sortLadders sorts Prices and BuyPrices by StartNumber. The live API sends
// BuyPrices in a random order in most records.
func (d *ComponentDetail) sortLadders() {
	d.Prices = SortPriceBreaks(d.Prices)
	d.BuyPrices = SortPriceBreaks(d.BuyPrices)
}

// componentDetailResponse is the response of the exact part detail.
type componentDetailResponse struct {
	Data *ComponentDetail `json:"data"`
}

// compareDetailsResponse is the response of the batch part detail.
type compareDetailsResponse struct {
	Data []struct {
		URLSuffix string           `json:"urlSuffix"`
		Detail    *ComponentDetail `json:"componentDetailVo"`
	} `json:"data"`
}

// Detail returns the detail record of one JLCPCB part code, for example
// "C1525". The code is not case sensitive. Detail sends one GET request on a
// cache miss.
//
// Detail returns ErrInvalidRequest when code is not a part code (C followed
// by digits). It returns ErrNotFound when JLCPCB does not know the code.
//
// Detail uses the endpoint of the add-to-cart dialog of the JLCPCB part
// pages. Its record has no pre-order ladder (BuyPrices is nil) and no file
// access ids, and its signed URLs expire after 30 minutes. DetailsByIDs
// returns these fields. Detail also returns parts that JLCPCB no longer
// lists (ComponentStatus "no"). DetailsByIDs does not return them.
//
// Detail sorts Prices by StartNumber.
func (s *ProductService) Detail(ctx context.Context, code string) (*ComponentDetail, error) {
	normalized := strings.ToUpper(strings.TrimSpace(code))
	if !isComponentCode(normalized) {
		return nil, fmt.Errorf("%w: %q is not a JLCPCB part code such as C1525", ErrInvalidRequest, code)
	}

	c := s.client
	cacheKey := cacheKeyForDetailCode(normalized)
	if detail, ok := c.cachedDetail(cacheKey); ok {
		return detail, nil
	}

	var resp componentDetailResponse
	params := url.Values{"componentCode": {normalized}}
	if err := c.doAPI(ctx, http.MethodGet, componentDetailPath, params, nil, &resp); err != nil {
		return nil, err
	}

	// The server answers an unknown code with code 200 and a record in
	// which every field is null.
	detail := resp.Data
	if detail == nil || strings.TrimSpace(detail.ComponentCode) == "" {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, normalized)
	}
	detail.sortLadders()
	c.cacheDetail(cacheKey, detail)
	return detail, nil
}

// DetailsByIDs returns the detail records of parts by numeric part id. The
// part id is Product.ComponentID, ComponentDetail.LCSCComponentID or the
// LCSC productId. The map key is the part id.
//
// DetailsByIDs drops ids of 0 or less and duplicate ids. It sends the other
// ids in POST requests of at most 250 ids. An empty id list returns an empty
// map and sends no request.
//
// The server omits unknown ids, parts that JLCPCB does not list, and
// parts with ComponentStatus "no". These ids are not in the map. A missing
// id is not an error. The server keeps the parts that JLCPCB does not sell,
// so check IsBuyComponent.
//
// The records have the pre-order ladder (BuyPrices) and the file access
// ids. Their signed URLs expire after 60 minutes. DetailsByIDs sorts Prices
// and BuyPrices by StartNumber. CanPresaleNumber is 0 where a search row
// sends a negative value.
//
// The cache keeps each record by part id, and DetailsByIDs requests only
// the ids that are not in the cache. When a request fails, DetailsByIDs
// returns the error and no map. The cache keeps the records of the earlier
// requests.
func (s *ProductService) DetailsByIDs(ctx context.Context, ids []int64) (map[int64]*ComponentDetail, error) {
	c := s.client
	unique := uniquePartIDs(ids)
	details := make(map[int64]*ComponentDetail, len(unique))

	var missing []int64
	for _, id := range unique {
		if detail, ok := c.cachedDetail(cacheKeyForDetailID(id)); ok {
			details[id] = detail
			continue
		}
		missing = append(missing, id)
	}

	for chunk := range slices.Chunk(missing, compareChunkSize) {
		requested := make(map[int64]bool, len(chunk))
		body := make([]string, 0, len(chunk))
		for _, id := range chunk {
			requested[id] = true
			body = append(body, strconv.FormatInt(id, 10))
		}

		var resp compareDetailsResponse
		if err := c.doAPI(ctx, http.MethodPost, compareDetailsPath, nil, body, &resp); err != nil {
			return nil, err
		}

		// The server sorts the records by part code, not in request order.
		// Keep only the records of the requested ids, so that each map key
		// is a requested id.
		for _, row := range resp.Data {
			detail := row.Detail
			if detail == nil || !requested[detail.LCSCComponentID] {
				continue
			}
			detail.URLSuffix = row.URLSuffix
			detail.sortLadders()
			details[detail.LCSCComponentID] = detail
			c.cacheDetail(cacheKeyForDetailID(detail.LCSCComponentID), detail)
		}
	}

	return details, nil
}

// uniquePartIDs returns the ids that are more than 0, without duplicates,
// in the order of the first use.
func uniquePartIDs(ids []int64) []int64 {
	seen := make(map[int64]bool, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

// cachedDetail returns the cached detail record of key. Each call returns a
// new record, so the caller can change it.
func (c *Client) cachedDetail(key string) (*ComponentDetail, bool) {
	if !c.cacheConfig.Enabled || c.cache == nil {
		return nil, false
	}
	cached, ok := c.cache.Get(key)
	if !ok {
		return nil, false
	}
	var detail ComponentDetail
	if err := json.Unmarshal(cached, &detail); err != nil {
		return nil, false
	}
	return &detail, true
}

// cacheDetail stores a detail record under key for DetailsTTL.
func (c *Client) cacheDetail(key string, detail *ComponentDetail) {
	if !c.cacheConfig.Enabled || c.cache == nil {
		return
	}
	if data, err := json.Marshal(detail); err == nil {
		c.cache.Set(key, data, c.cacheConfig.DetailsTTL)
	}
}

// cacheKeyForDetailCode returns the cache key of a Detail record. The code
// is upper case.
func cacheKeyForDetailCode(code string) string {
	return "detail:code:" + code
}

// cacheKeyForDetailID returns the cache key of a DetailsByIDs record.
func cacheKeyForDetailID(id int64) string {
	return "detail:id:" + strconv.FormatInt(id, 10)
}
