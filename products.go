package jlcpcb

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// searchRequestBody is the JSON body for the search endpoint.
type searchRequestBody struct {
	Keyword                    string        `json:"keyword"`
	CurrentPage                int           `json:"currentPage"`
	PageSize                   int           `json:"pageSize"`
	PresaleType                string        `json:"presaleType"` // "stock", "buy", "post"
	SearchType                 int           `json:"searchType"`  // 1 = suggest, 2 = full search
	ComponentLibraryType       interface{}   `json:"componentLibraryType"`
	ComponentAttributeList     []interface{} `json:"componentAttributeList"`
	ComponentBrandList         []interface{} `json:"componentBrandList"`
	ComponentSpecificationList []interface{} `json:"componentSpecificationList"`
	ParamList                  []interface{} `json:"paramList"`
	FirstSortName              interface{}   `json:"firstSortName"`
	SecondSortName             interface{}   `json:"secondSortName"`
	SearchSource               string        `json:"searchSource"` // "search"
	StockFlag                  bool          `json:"stockFlag"`
	PreferredComponentFlag     bool          `json:"preferredComponentFlag"`
}

// KeywordSearch searches for products by keyword with optional filters.
// Uses POST /selectSmtComponentList/v2 endpoint.
func (c *Client) KeywordSearch(ctx context.Context, req SearchRequest) (*SearchResponse, error) {
	keyword := strings.TrimSpace(req.Keyword)
	if keyword == "" {
		return nil, fmt.Errorf("keyword is required")
	}

	if req.CurrentPage <= 0 {
		req.CurrentPage = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 50
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}

	cacheKey := c.getCacheKeySearch(keyword, req.CurrentPage, req.PageSize)
	if c.cache != nil {
		if cached, ok := c.cache.Get(cacheKey); ok {
			var resp SearchResponse
			if err := json.Unmarshal(cached, &resp); err == nil {
				return &resp, nil
			}
		}
	}

	// Build attribute filters
	attrList := []interface{}{}
	for _, attr := range req.Attributes {
		attrList = append(attrList, map[string]string{
			"attributeName":  attr.Name,
			"attributeValue": attr.Value,
		})
	}

	// Build brand filters
	brandList := []interface{}{}
	for _, brand := range req.Brands {
		brandList = append(brandList, brand)
	}

	// Determine presale type
	presaleType := "stock"
	if req.PresaleType != "" {
		presaleType = req.PresaleType
	}

	// Determine component library type
	var componentLibType interface{}
	if req.ComponentType != "" {
		componentLibType = req.ComponentType
	}

	body, err := c.doRequest(ctx, "POST", "/selectSmtComponentList/v2", nil, searchRequestBody{
		Keyword:                    keyword,
		CurrentPage:                req.CurrentPage,
		PageSize:                   req.PageSize,
		PresaleType:                presaleType,
		SearchType:                 2,
		ComponentLibraryType:       componentLibType,
		ComponentAttributeList:     attrList,
		ComponentBrandList:         brandList,
		ComponentSpecificationList: []interface{}{},
		ParamList:                  []interface{}{},
		FirstSortName:              req.SortBy,
		SecondSortName:             req.SortBySecondary,
		SearchSource:               "search",
		StockFlag:                  req.StockOnly,
		PreferredComponentFlag:     req.PreferredOnly,
	})
	if err != nil {
		return nil, err
	}

	var wrapper productSearchWrapper
	if err := c.parseResponse(body, &wrapper); err != nil {
		return nil, err
	}

	resp := &SearchResponse{
		Products:   wrapper.Data.ComponentPageInfo.Products,
		TotalCount: wrapper.Data.ComponentPageInfo.TotalCount,
		PageSize:   wrapper.Data.ComponentPageInfo.PageSize,
		PageNumber: wrapper.Data.ComponentPageInfo.PageNumber,
	}

	if c.cache != nil {
		if cacheData, err := json.Marshal(resp); err == nil {
			c.cache.Set(cacheKey, cacheData, 5*time.Minute)
		}
	}

	return resp, nil
}

// GetProductDetails retrieves detailed information for a specific product.
// Uses search endpoint to find product by part code
func (c *Client) GetProductDetails(ctx context.Context, partCode string) (*Product, error) {
	partCode = strings.TrimSpace(partCode)
	if partCode == "" {
		return nil, fmt.Errorf("part code is required")
	}

	cacheKey := c.getCacheKeyProduct(partCode)
	if c.cache != nil {
		if cached, ok := c.cache.Get(cacheKey); ok {
			var product Product
			if err := json.Unmarshal(cached, &product); err == nil {
				return &product, nil
			}
		}
	}

	// Search for the product by part code
	resp, err := c.KeywordSearch(ctx, SearchRequest{
		Keyword:     partCode,
		CurrentPage: 1,
		PageSize:    1,
	})
	if err != nil {
		return nil, err
	}

	if resp == nil || len(resp.Products) == 0 {
		return nil, fmt.Errorf("product not found: %s", partCode)
	}

	product := &resp.Products[0]

	if c.cache != nil {
		if cacheData, err := json.Marshal(product); err == nil {
			c.cache.Set(cacheKey, cacheData, 5*time.Minute)
		}
	}

	return product, nil
}

// getCacheKeySearch generates a cache key for search requests.
func (c *Client) getCacheKeySearch(keyword string, page, pageSize int) string {
	return fmt.Sprintf("search:%s:%s:%d:%d", c.currency, keyword, page, pageSize)
}

// getCacheKeyProduct generates a cache key for product detail requests.
func (c *Client) getCacheKeyProduct(sku string) string {
	return fmt.Sprintf("product:%s:%s", c.currency, sku)
}
