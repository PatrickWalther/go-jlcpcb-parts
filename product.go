package jlcpcb

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
)

// ProductService handles product-related operations.
type ProductService service

// Details returns a detailed product record for the given identifier.
//
// The identifier can be a JLC component code (for example, C3900982)
// or a manufacturer part number (for example, CGJ2B2C0G1H390J050BA).
//
// Details searches 10 results for a component code and 50 results for any
// other identifier. It returns the exact componentCode or componentModelEn
// match. If no result matches exactly, it returns the first result.
//
// Details is a keyword search, so it can return a part with a different
// code. Use Detail for the exact record of a component code, and
// DetailsByIDs for the records of many parts in few requests. A search row
// has fields that a detail record does not have: PreferredComponentFlag,
// EstimateDate and MergedComponentCode.
func (s *ProductService) Details(ctx context.Context, identifier string) (*Product, error) {
	normalized := strings.TrimSpace(identifier)
	if normalized == "" {
		return nil, fmt.Errorf("%w: identifier is required", ErrInvalidRequest)
	}

	c := s.client
	cacheKey := cacheKeyForProduct(normalized)
	if c.cacheConfig.Enabled && c.cache != nil {
		if cached, ok := c.cache.Get(cacheKey); ok {
			var cachedProduct Product
			if err := json.Unmarshal(cached, &cachedProduct); err == nil {
				return &cachedProduct, nil
			}
		}
	}

	searchResp, err := c.Search.Keyword(ctx, &SearchRequest{
		Keyword:     normalized,
		Page:        1,
		PageSize:    detailsPageSize(normalized),
		PresaleType: PresaleTypeAny,
	})
	if err != nil {
		return nil, err
	}

	if searchResp == nil || len(searchResp.Products) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, normalized)
	}

	product := searchResp.Products[0]
	for _, candidate := range searchResp.Products {
		if strings.EqualFold(candidate.ComponentCode, normalized) ||
			strings.EqualFold(candidate.ComponentModelEn, normalized) {
			product = candidate
			break
		}
	}

	if c.cacheConfig.Enabled && c.cache != nil {
		if data, err := json.Marshal(product); err == nil {
			c.cache.Set(cacheKey, data, c.cacheConfig.DetailsTTL)
		}
	}

	return &product, nil
}

const (
	// detailsCodePageSize is the search page size for a JLC component code.
	// The exact match was the first result in every live probe.
	detailsCodePageSize = 10
	// detailsMPNPageSize is the search page size for a manufacturer part number.
	detailsMPNPageSize = 50
)

// detailsPageSize returns the search page size for a Details lookup.
func detailsPageSize(identifier string) int {
	if isComponentCode(identifier) {
		return detailsCodePageSize
	}
	return detailsMPNPageSize
}

// isComponentCode reports whether identifier matches ^[Cc][0-9]+$.
func isComponentCode(identifier string) bool {
	if len(identifier) < 2 || (identifier[0] != 'C' && identifier[0] != 'c') {
		return false
	}
	for i := 1; i < len(identifier); i++ {
		if identifier[i] < '0' || identifier[i] > '9' {
			return false
		}
	}
	return true
}

func cacheKeyForProduct(identifier string) string {
	normalized := strings.ToUpper(strings.TrimSpace(identifier))
	hash := sha256.Sum256([]byte(normalized))
	return fmt.Sprintf("product:%x", hash)
}
