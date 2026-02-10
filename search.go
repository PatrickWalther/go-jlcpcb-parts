package jlcpcb

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// PresaleType controls search inventory scope.
type PresaleType string

const (
	// PresaleTypeAny searches across all part availability classes.
	PresaleTypeAny PresaleType = ""
	// PresaleTypeStock limits results to in-stock items.
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

// SearchService handles component search operations.
type SearchService service

// SearchRequest contains parameters for product search.
type SearchRequest struct {
	Keyword       string
	Page          int
	PageSize      int
	PresaleType   PresaleType
	StockOnly     bool
	ComponentType ComponentType
	Attributes    []FilterAttribute
	Brands        []string
	SortPrimary   string
	SortSecondary string
}

// SearchResponse contains search results.
type SearchResponse struct {
	Products   []Product `json:"list"`
	TotalCount int       `json:"total"`
	PageSize   int       `json:"pageSize"`
	Page       int       `json:"pageNum"`
}

type searchResponseWrapper struct {
	Data struct {
		ComponentPageInfo SearchResponse `json:"componentPageInfo"`
	} `json:"data"`
}

// searchRequestBody matches the JSON payload expected by JLCPCB search endpoint.
type searchRequestBody struct {
	Keyword                    string        `json:"keyword"`
	CurrentPage                int           `json:"currentPage"`
	PageSize                   int           `json:"pageSize"`
	PresaleType                string        `json:"presaleType"`
	SearchType                 int           `json:"searchType"`
	ComponentLibraryType       interface{}   `json:"componentLibraryType"`
	ComponentAttributeList     []interface{} `json:"componentAttributeList"`
	ComponentBrandList         []interface{} `json:"componentBrandList"`
	ComponentSpecificationList []interface{} `json:"componentSpecificationList"`
	ParamList                  []interface{} `json:"paramList"`
	FirstSortName              interface{}   `json:"firstSortName"`
	SecondSortName             interface{}   `json:"secondSortName"`
	SearchSource               string        `json:"searchSource"`
	StockFlag                  bool          `json:"stockFlag"`
}

// Keyword searches products by keyword.
func (s *SearchService) Keyword(ctx context.Context, req *SearchRequest) (*SearchResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("%w: request is nil", ErrInvalidRequest)
	}

	keyword := strings.TrimSpace(req.Keyword)
	if keyword == "" {
		return nil, fmt.Errorf("%w: keyword is required", ErrInvalidRequest)
	}

	searchReq := *req
	searchReq.Keyword = keyword
	if searchReq.Page <= 0 {
		searchReq.Page = 1
	}
	if searchReq.PageSize <= 0 {
		searchReq.PageSize = 50
	}
	if searchReq.PageSize > 100 {
		searchReq.PageSize = 100
	}

	presaleType := strings.ToLower(strings.TrimSpace(string(searchReq.PresaleType)))
	if presaleType == "" && searchReq.StockOnly {
		presaleType = string(PresaleTypeStock)
	}

	c := s.client
	if c.cacheConfig.Enabled && c.cache != nil {
		cacheKey := cacheKeyForSearch(&searchReq, presaleType)
		if cached, ok := c.cache.Get(cacheKey); ok {
			var cachedResp SearchResponse
			if err := json.Unmarshal(cached, &cachedResp); err == nil {
				return &cachedResp, nil
			}
		}
	}

	attrList := make([]interface{}, 0, len(searchReq.Attributes))
	for _, attr := range searchReq.Attributes {
		attrList = append(attrList, map[string]string{
			"attributeName":  attr.Name,
			"attributeValue": attr.Value,
		})
	}

	brandList := make([]interface{}, 0, len(searchReq.Brands))
	for _, brand := range searchReq.Brands {
		brandList = append(brandList, brand)
	}

	var componentLibraryType interface{}
	if searchReq.ComponentType != ComponentTypeAny {
		componentLibraryType = string(searchReq.ComponentType)
	}

	payload := searchRequestBody{
		Keyword:                    searchReq.Keyword,
		CurrentPage:                searchReq.Page,
		PageSize:                   searchReq.PageSize,
		PresaleType:                presaleType,
		SearchType:                 2,
		ComponentLibraryType:       componentLibraryType,
		ComponentAttributeList:     attrList,
		ComponentBrandList:         brandList,
		ComponentSpecificationList: []interface{}{},
		ParamList:                  []interface{}{},
		FirstSortName:              nullableString(searchReq.SortPrimary),
		SecondSortName:             nullableString(searchReq.SortSecondary),
		SearchSource:               "search",
		StockFlag:                  searchReq.StockOnly,
	}

	var wrapper searchResponseWrapper
	if err := c.do(ctx, http.MethodPost, "/selectSmtComponentList/v2", nil, payload, &wrapper); err != nil {
		return nil, err
	}

	resp := wrapper.Data.ComponentPageInfo
	if c.cacheConfig.Enabled && c.cache != nil {
		if data, err := json.Marshal(resp); err == nil {
			cacheKey := cacheKeyForSearch(&searchReq, presaleType)
			c.cache.Set(cacheKey, data, c.cacheConfig.SearchTTL)
		}
	}

	return &resp, nil
}

func cacheKeyForSearch(req *SearchRequest, presaleType string) string {
	keyPayload := struct {
		Keyword       string            `json:"keyword"`
		Page          int               `json:"page"`
		PageSize      int               `json:"pageSize"`
		PresaleType   string            `json:"presaleType"`
		StockOnly     bool              `json:"stockOnly"`
		ComponentType ComponentType     `json:"componentType"`
		Attributes    []FilterAttribute `json:"attributes"`
		Brands        []string          `json:"brands"`
		SortPrimary   string            `json:"sortPrimary"`
		SortSecondary string            `json:"sortSecondary"`
	}{
		Keyword:       req.Keyword,
		Page:          req.Page,
		PageSize:      req.PageSize,
		PresaleType:   presaleType,
		StockOnly:     req.StockOnly,
		ComponentType: req.ComponentType,
		Attributes:    req.Attributes,
		Brands:        req.Brands,
		SortPrimary:   req.SortPrimary,
		SortSecondary: req.SortSecondary,
	}

	serialized, err := json.Marshal(keyPayload)
	if err != nil {
		return fmt.Sprintf("search:%s:%d:%d:%s", req.Keyword, req.Page, req.PageSize, presaleType)
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
