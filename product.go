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
		PageSize:    50,
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

func cacheKeyForProduct(identifier string) string {
	normalized := strings.ToUpper(strings.TrimSpace(identifier))
	hash := sha256.Sum256([]byte(normalized))
	return fmt.Sprintf("product:%x", hash)
}
