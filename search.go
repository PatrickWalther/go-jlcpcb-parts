package jlcpcb

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
)

// PresaleType controls search inventory scope.
type PresaleType string

const (
	// PresaleTypeAny searches across all part availability classes.
	PresaleTypeAny PresaleType = ""
	// PresaleTypeStock limits results to the stock availability class.
	// This class includes some rows with stock 0. Use SearchRequest.StockOnly
	// or SearchRequest.MinStock to get only rows with stock.
	PresaleTypeStock PresaleType = "stock"
	// PresaleTypeBuy limits results to pre-order items.
	PresaleTypeBuy PresaleType = "buy"
	// PresaleTypePost limits results to post-supply items.
	PresaleTypePost PresaleType = "post"
)

// ComponentType controls the component library type filter.
type ComponentType string

const (
	ComponentTypeAny    ComponentType = ""
	ComponentTypeBase   ComponentType = "base"
	ComponentTypeExpand ComponentType = "expand"
)

// SortMode selects the sort key of search results.
type SortMode string

const (
	// SortDefault keeps the order of the server (relevance).
	SortDefault SortMode = ""
	// SortByModel sorts by manufacturer part number.
	SortByModel SortMode = "MODEL_SORT"
	// SortByStock sorts by stock quantity.
	SortByStock SortMode = "STOCK_SORT"
	// SortByPrice sorts by unit price. The key is probably the first tier
	// of ComponentPrices.
	SortByPrice SortMode = "PRICE_SORT"
)

// PCBAFilter limits search results to the parts that a PCBA type accepts
// (search field pcbAType).
type PCBAFilter int

const (
	// PCBAFilterNone does not filter by PCBA type.
	PCBAFilterNone PCBAFilter = 0
	// PCBAFilterEconomic removes the parts that Economic PCBA does not accept.
	PCBAFilterEconomic PCBAFilter = 1
	// PCBAFilterStandard keeps the parts that Standard PCBA accepts.
	PCBAFilterStandard PCBAFilter = 2
)

// Category names a part category for a search. The search matches category
// names. The server ignores numeric category ids in a search.
type Category struct {
	// Parent is the first-level category, for example "Capacitors".
	// The search sends it as firstSortName.
	Parent string
	// Leaf is the second-level category, for example
	// "Multilayer Ceramic Capacitors MLCC - SMD/SMT".
	// The search sends it as secondSortName.
	Leaf string
}

// AttributeFilter keeps only parts with one of Values for the attribute Name.
type AttributeFilter struct {
	// Name is the attribute name (attribute_name_en), for example
	// "Voltage Rating".
	Name string
	// Values are exact attribute values, for example "25V". The server
	// matches exact strings: "100nF" matches, but "0.1uF" does not. A part
	// matches when it has one of the values.
	Values []string
}

// SearchService handles component search operations.
type SearchService service

// SearchRequest contains parameters for product search.
//
// The server applies all filters together: a part must match each filter.
type SearchRequest struct {
	Keyword  string
	Page     int
	PageSize int
	// PresaleType limits results to one availability class (presaleType).
	PresaleType PresaleType
	// StockOnly keeps only parts with stock (stockFlag, stockCount > 0).
	// When PresaleType and PresaleTypes are empty, StockOnly also sends
	// presaleType "stock".
	StockOnly     bool
	ComponentType ComponentType
	// Attributes filters by attribute values.
	//
	// Deprecated: Use AttributeFilters, which accepts more than one value
	// for an attribute. Attributes still works. The search groups the
	// entries of Attributes and AttributeFilters by name.
	Attributes []FilterAttribute
	Brands     []string
	// Packages keeps only parts whose package (componentSpecificationEn) is
	// one of these values, for example "0603". The endpoint applies the
	// filter, so every page holds only matching parts.
	Packages []string
	// SortPrimary is the parent category name.
	//
	// Deprecated: Use Category.Parent. SortPrimary is a category filter, not
	// a sort field. Category.Parent overrides it when both are set.
	SortPrimary string
	// SortSecondary is the leaf category name.
	//
	// Deprecated: Use Category.Leaf. SortSecondary is a category filter, not
	// a sort field. Category.Leaf overrides it when both are set.
	SortSecondary string

	// IncludePreferred adds preferred extended parts (preferredComponentFlag).
	// With ComponentType or LibraryTypes set to ComponentTypeBase, the
	// server returns parts that are basic or preferred. Without a library
	// type, the server returns only preferred extended parts.
	IncludePreferred bool
	// Category limits results to one category, by name.
	Category Category
	// AttributeFilters filters by attribute values (componentAttributeList).
	// A part must match each filter.
	AttributeFilters []AttributeFilter
	// LibraryTypes limits results to these library types (componentLibTypes).
	LibraryTypes []ComponentType
	// PresaleTypes limits results to these availability classes
	// (presaleTypes). The class "stock" includes some rows with stock 0.
	PresaleTypes []PresaleType
	// MinStock keeps only parts with at least this stock (startStockNumber).
	// 0 does not filter.
	MinStock int
	// PCBA keeps only parts that a PCBA type accepts (pcbAType).
	PCBA PCBAFilter
	// Sort selects the sort key (sortMode). SortDefault keeps the order of
	// the server.
	Sort SortMode
	// SortDescending sorts from high to low (sortASC "DESC"). It has no
	// effect when Sort is SortDefault.
	SortDescending bool
	// HasDatasheet keeps only parts with a datasheet (dateSheet).
	HasDatasheet bool
	// CategoryCounts asks for the category tree with part counts
	// (searchType 3). SearchResponse.Categories holds the tree.
	CategoryCounts bool
}

// SearchResponse contains search results.
type SearchResponse struct {
	Products   []Product `json:"list"`
	TotalCount int       `json:"total"`
	PageSize   int       `json:"pageSize"`
	Page       int       `json:"pageNum"`
	// Pages is the number of pages for this page size. A page past the last
	// page has no products, and the server then sends 0 for TotalCount and
	// Pages.
	Pages int `json:"pages"`
	// Categories is the category tree of the matching parts, with part
	// counts. The server sends it only when SearchRequest.CategoryCounts is
	// set.
	Categories []CategoryCount `json:"categories,omitempty"`
}

// CategoryCount is one node of a category tree with part counts. A search
// response (data.sortAndCountVoList) and Facets.Categories hold this tree.
// The JSON tags are the wire names of the search response.
type CategoryCount struct {
	ID       int             `json:"componentSortKeyId"` // Numeric category id, for example 2929
	ParentID int             `json:"parentId"`           // Numeric id of the parent category (0 for a first-level category)
	Name     string          `json:"sortName"`           // Category name, for use in Category
	Level    int             `json:"grade"`              // 1 for a first-level category, 2 for a leaf
	Count    int             `json:"componentCount"`     // Number of matching parts in the category
	Children []CategoryCount `json:"childSortList"`      // Leaf categories (nil for a leaf)
}

type searchResponseWrapper struct {
	Data struct {
		ComponentPageInfo  SearchResponse  `json:"componentPageInfo"`
		SortAndCountVoList []CategoryCount `json:"sortAndCountVoList"`
	} `json:"data"`
}

const (
	// defaultSearchPageSize is the page size when SearchRequest.PageSize is 0.
	defaultSearchPageSize = 50
	// maxSearchPageSize is the largest page size that a search sends. The
	// server accepts at least 1200, but a page of 1000 rows is already
	// about 7 MB.
	maxSearchPageSize = 1000

	// searchTypeRows asks for result rows.
	searchTypeRows = 2
	// searchTypeRowsAndCategories asks for result rows and the category tree.
	searchTypeRowsAndCategories = 3
)

// searchRequestBody matches the JSON payload expected by JLCPCB search endpoint.
//
// The fields with omitempty are optional. The server accepts a body without
// them, so the body of a request without these options does not change.
type searchRequestBody struct {
	Keyword                    interface{}           `json:"keyword"`
	CurrentPage                int                   `json:"currentPage"`
	PageSize                   int                   `json:"pageSize"`
	PresaleType                string                `json:"presaleType"`
	SearchType                 int                   `json:"searchType"`
	ComponentLibraryType       interface{}           `json:"componentLibraryType"`
	ComponentAttributeList     []map[string][]string `json:"componentAttributeList"`
	ComponentBrandList         []interface{}         `json:"componentBrandList"`
	ComponentSpecificationList []interface{}         `json:"componentSpecificationList"`
	ParamList                  []interface{}         `json:"paramList"`
	FirstSortName              interface{}           `json:"firstSortName"`
	SecondSortName             interface{}           `json:"secondSortName"`
	SearchSource               string                `json:"searchSource"`
	StockFlag                  bool                  `json:"stockFlag"`
	ComponentLibTypes          []string              `json:"componentLibTypes,omitempty"`
	PreferredComponentFlag     bool                  `json:"preferredComponentFlag,omitempty"`
	PresaleTypes               []string              `json:"presaleTypes,omitempty"`
	StartStockNumber           int                   `json:"startStockNumber,omitempty"`
	PCBAType                   int                   `json:"pcbAType,omitempty"`
	SortMode                   string                `json:"sortMode,omitempty"`
	SortASC                    string                `json:"sortASC,omitempty"`
	DateSheet                  bool                  `json:"dateSheet,omitempty"`
}

// Keyword searches products by keyword. The keyword is required.
//
// Keyword accepts all filters of SearchRequest. Use Query to search without
// a keyword.
func (s *SearchService) Keyword(ctx context.Context, req *SearchRequest) (*SearchResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("%w: request is nil", ErrInvalidRequest)
	}
	if strings.TrimSpace(req.Keyword) == "" {
		return nil, fmt.Errorf("%w: keyword is required", ErrInvalidRequest)
	}
	return s.search(ctx, req)
}

// Query searches products. The keyword is optional, so Query can list the
// parts of a category or the parts with given attribute values. Query sends
// a JSON null keyword when the keyword is empty.
func (s *SearchService) Query(ctx context.Context, req *SearchRequest) (*SearchResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("%w: request is nil", ErrInvalidRequest)
	}
	return s.search(ctx, req)
}

// Pages calls Query for each page of req and gives each page to fn.
//
// Pages starts at req.Page (page 1 when req.Page is 0 or less). It stops
// after the last page or at the first page without products. It does not
// call fn for a page without products. When fn or a request returns an
// error, Pages stops and returns that error. Pages does not change req.
//
// A page past the last page returns TotalCount 0 and Pages 0. Thus Pages
// copies TotalCount and Pages of the first page into each later page.
func (s *SearchService) Pages(ctx context.Context, req *SearchRequest, fn func(*SearchResponse) error) error {
	if req == nil {
		return fmt.Errorf("%w: request is nil", ErrInvalidRequest)
	}
	if fn == nil {
		return fmt.Errorf("%w: page function is nil", ErrInvalidRequest)
	}

	pageReq := *req
	if pageReq.Page <= 0 {
		pageReq.Page = 1
	}

	var total, pages, seen int
	for first := true; ; first = false {
		resp, err := s.Query(ctx, &pageReq)
		if err != nil {
			return err
		}
		if first {
			total, pages = resp.TotalCount, resp.Pages
		} else {
			resp.TotalCount, resp.Pages = total, pages
		}

		if len(resp.Products) == 0 {
			return nil
		}
		seen += len(resp.Products)
		if err := fn(resp); err != nil {
			return err
		}

		switch {
		case pages > 0:
			if pageReq.Page >= pages {
				return nil
			}
		case total > 0:
			// The server sent no page count. Stop when the walk has all
			// rows. A walk that starts after page 1 stops at an empty page.
			if seen >= total {
				return nil
			}
		default:
			// The server sent no page count and no total. Stop, because
			// the next page can be the same page again.
			return nil
		}
		pageReq.Page++
	}
}

// search sends one search request. The caller validates req.
func (s *SearchService) search(ctx context.Context, req *SearchRequest) (*SearchResponse, error) {
	c := s.client
	payload := newSearchRequestBody(req)
	cacheKey := cacheKeyForSearch(payload)

	if c.cacheConfig.Enabled && c.cache != nil {
		if cached, ok := c.cache.Get(cacheKey); ok {
			var cachedResp SearchResponse
			if err := json.Unmarshal(cached, &cachedResp); err == nil {
				return &cachedResp, nil
			}
		}
	}

	var wrapper searchResponseWrapper
	if err := c.do(ctx, http.MethodPost, "/selectSmtComponentList/v2", nil, payload, &wrapper); err != nil {
		return nil, err
	}

	resp := wrapper.Data.ComponentPageInfo
	resp.Categories = wrapper.Data.SortAndCountVoList
	if c.cacheConfig.Enabled && c.cache != nil {
		if data, err := json.Marshal(resp); err == nil {
			c.cache.Set(cacheKey, data, c.cacheConfig.SearchTTL)
		}
	}

	return &resp, nil
}

// newSearchRequestBody normalizes req and returns the request body.
func newSearchRequestBody(req *SearchRequest) searchRequestBody {
	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = defaultSearchPageSize
	}
	if pageSize > maxSearchPageSize {
		pageSize = maxSearchPageSize
	}

	presaleTypes := normalizedPresaleTypes(req.PresaleTypes)
	presaleType := strings.ToLower(strings.TrimSpace(string(req.PresaleType)))
	if presaleType == "" && len(presaleTypes) == 0 && req.StockOnly {
		presaleType = string(PresaleTypeStock)
	}

	var componentLibraryType interface{}
	if req.ComponentType != ComponentTypeAny {
		componentLibraryType = string(req.ComponentType)
	}

	brandList := make([]interface{}, 0, len(req.Brands))
	for _, brand := range req.Brands {
		brandList = append(brandList, brand)
	}

	packageList := make([]interface{}, 0, len(req.Packages))
	for _, pkg := range req.Packages {
		if pkg = strings.TrimSpace(pkg); pkg != "" {
			packageList = append(packageList, pkg)
		}
	}

	parent := strings.TrimSpace(req.Category.Parent)
	if parent == "" {
		parent = req.SortPrimary
	}
	leaf := strings.TrimSpace(req.Category.Leaf)
	if leaf == "" {
		leaf = req.SortSecondary
	}

	searchType := searchTypeRows
	if req.CategoryCounts {
		searchType = searchTypeRowsAndCategories
	}

	body := searchRequestBody{
		Keyword:                    nullableString(req.Keyword),
		CurrentPage:                page,
		PageSize:                   pageSize,
		PresaleType:                presaleType,
		SearchType:                 searchType,
		ComponentLibraryType:       componentLibraryType,
		ComponentAttributeList:     attributeList(req.Attributes, req.AttributeFilters),
		ComponentBrandList:         brandList,
		ComponentSpecificationList: packageList,
		ParamList:                  []interface{}{},
		FirstSortName:              nullableString(parent),
		SecondSortName:             nullableString(leaf),
		SearchSource:               "search",
		StockFlag:                  req.StockOnly,
		ComponentLibTypes:          normalizedLibraryTypes(req.LibraryTypes),
		PreferredComponentFlag:     req.IncludePreferred,
		PresaleTypes:               presaleTypes,
		DateSheet:                  req.HasDatasheet,
	}
	if req.MinStock > 0 {
		body.StartStockNumber = req.MinStock
	}
	if req.PCBA > 0 {
		body.PCBAType = int(req.PCBA)
	}
	if sortMode := strings.ToUpper(strings.TrimSpace(string(req.Sort))); sortMode != "" {
		body.SortMode = sortMode
		body.SortASC = "ASC"
		if req.SortDescending {
			body.SortASC = "DESC"
		}
	}
	return body
}

// attributeList returns the componentAttributeList of a search. It groups
// the values by attribute name, in the order of the first use of each name:
// [{"Capacitance":["100nF","1uF"]},{"Voltage Rating":["25V"]}].
//
// The server rejects the older shape [{attributeName, attributeValue}] with
// envelope code 101. attributeList drops empty names, empty values and
// duplicate values. It returns an empty, non-nil list when no filter is set.
func attributeList(legacy []FilterAttribute, filters []AttributeFilter) []map[string][]string {
	list := make([]map[string][]string, 0, len(legacy)+len(filters))
	index := make(map[string]int)
	add := func(name string, values ...string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		for _, value := range values {
			value = strings.TrimSpace(value)
			if value == "" {
				continue
			}
			i, ok := index[name]
			if !ok {
				i = len(list)
				index[name] = i
				list = append(list, map[string][]string{name: nil})
			}
			if !slices.Contains(list[i][name], value) {
				list[i][name] = append(list[i][name], value)
			}
		}
	}
	for _, attr := range legacy {
		add(attr.Name, attr.Value)
	}
	for _, filter := range filters {
		add(filter.Name, filter.Values...)
	}
	return list
}

// normalizedLibraryTypes returns the lower-case library types without empty
// or duplicate values. It returns nil when no type is set.
func normalizedLibraryTypes(types []ComponentType) []string {
	values := make([]string, 0, len(types))
	for _, t := range types {
		values = append(values, string(t))
	}
	return normalizedValues(values)
}

// normalizedPresaleTypes returns the lower-case presale types without empty
// or duplicate values. It returns nil when no type is set.
func normalizedPresaleTypes(types []PresaleType) []string {
	values := make([]string, 0, len(types))
	for _, t := range types {
		values = append(values, string(t))
	}
	return normalizedValues(values)
}

// normalizedValues trims values and changes them to lower case. It drops
// empty and duplicate values. It returns nil when no value is left.
func normalizedValues(values []string) []string {
	var out []string
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value != "" && !slices.Contains(out, value) {
			out = append(out, value)
		}
	}
	return out
}

// cacheKeyForSearch returns the cache key of a search request body. The key
// covers every field that the request sends.
func cacheKeyForSearch(body searchRequestBody) string {
	serialized, err := json.Marshal(body)
	if err != nil {
		return fmt.Sprintf("search:%v:%d:%d:%s", body.Keyword, body.CurrentPage, body.PageSize, body.PresaleType)
	}
	hash := sha256.Sum256(serialized)
	return fmt.Sprintf("search:%x", hash)
}

func nullableString(value string) interface{} {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return trimmed
}
