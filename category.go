package jlcpcb

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// categoryInfoPath is the path of the category name lookup below the API
// root. The client adds the numeric category id.
const categoryInfoPath = "/overseas-smt/web/component/sort/info/"

// CategoryService looks up JLCPCB part categories.
type CategoryService service

// CategoryInfo holds the names and numeric ids of a category and its parent.
// The JSON tags are the wire names.
type CategoryInfo struct {
	ParentID   int    `json:"firstSortId"`    // Numeric id of the first-level category, for example 2
	ParentName string `json:"firstSortName"`  // Name of the first-level category, for example "Capacitors"
	LeafID     int    `json:"secondSortId"`   // Numeric id of the leaf category (0 for a first-level category)
	LeafName   string `json:"secondSortName"` // Name of the leaf category ("" for a first-level category)
}

// Category returns the names as a search Category filter.
func (i *CategoryInfo) Category() Category {
	return Category{Parent: i.ParentName, Leaf: i.LeafName}
}

// categoryInfoResponse is the response of the category name lookup.
type categoryInfoResponse struct {
	Data *CategoryInfo `json:"data"`
}

// Info returns the names of the category with the numeric id sortKeyID
// (componentSortKeyId), for example 2929. For a leaf category, Info also
// returns the parent. For a first-level category, LeafID is 0 and LeafName
// is "". Info sends one GET request on a cache miss. The cache keeps the
// answer for CacheConfig.CategoryTTL (24 hours by default).
//
// Use Info to get the category names for a search Category filter from the
// numeric ids of ComponentDetail or Facets.
//
// Info returns ErrInvalidRequest for an id of 0 or less and sends no
// request. The server answers an unknown id with envelope code 500. Info
// then returns an error that matches ErrNotFound and ErrServer, because the
// answer does not show which one applies. Info does not retry this answer.
func (s *CategoryService) Info(ctx context.Context, sortKeyID int) (*CategoryInfo, error) {
	if sortKeyID <= 0 {
		return nil, fmt.Errorf("%w: category id must be more than 0, got %d", ErrInvalidRequest, sortKeyID)
	}

	c := s.client
	cacheKey := "category:" + strconv.Itoa(sortKeyID)
	var cached CategoryInfo
	if c.cachedValue(cacheKey, &cached) {
		return &cached, nil
	}

	var resp categoryInfoResponse
	endpoint := c.apiRoot + categoryInfoPath + strconv.Itoa(sortKeyID)
	err := c.doURLWithRetry(ctx, retryUnlessEnvelopeError, http.MethodGet, endpoint, nil, nil, &resp)
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusOK && errors.Is(err, ErrServer) {
		return nil, fmt.Errorf("%w: category %d: %w", ErrNotFound, sortKeyID, err)
	}
	if err != nil {
		return nil, err
	}

	info := resp.Data
	if info == nil || strings.TrimSpace(info.ParentName) == "" {
		return nil, fmt.Errorf("%w: category %d", ErrNotFound, sortKeyID)
	}
	c.cacheValue(cacheKey, info, c.cacheConfig.categoryTTL())
	return info, nil
}
