# go-jlcpcb-parts

[![Go Reference](https://pkg.go.dev/badge/github.com/PatrickWalther/go-jlcpcb-parts.svg)](https://pkg.go.dev/github.com/PatrickWalther/go-jlcpcb-parts)
[![Go Report Card](https://goreportcard.com/badge/github.com/PatrickWalther/go-jlcpcb-parts)](https://goreportcard.com/report/github.com/PatrickWalther/go-jlcpcb-parts)
[![Tests](https://github.com/PatrickWalther/go-jlcpcb-parts/actions/workflows/test.yml/badge.svg)](https://github.com/PatrickWalther/go-jlcpcb-parts/actions)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A Go client for JLCPCB parts search endpoints used by https://jlcpcb.com/parts.

> Note: JLCPCB does not publish an official public parts API. This library uses publicly accessible endpoints and can break if JLCPCB changes their internal contracts.

## Requirements

- Go 1.22+
- No external dependencies (stdlib only)

## Installation

```bash
go get github.com/PatrickWalther/go-jlcpcb-parts
```

## Features

- Service-based API (`client.Search`, `client.Product`)
- Keyword part search with filters and pagination
- Product details lookup by JLC code or MPN
- Assembly library class (basic, preferred, extended) and ordering fields
- Price ladders sorted by quantity
- Typed error handling (`errors.Is`)
- Optional in-memory response caching
- Rate limiting and retry/backoff
- Thread-safe client for concurrent use

## Quick Start

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/PatrickWalther/go-jlcpcb-parts"
)

func main() {
	client := jlcpcb.NewClient()
	ctx := context.Background()

	resp, err := client.Search.Keyword(ctx, &jlcpcb.SearchRequest{
		Keyword:  "CGJ2B2C0G1H390J050BA",
		PageSize: 5,
	})
	if err != nil {
		log.Fatal(err)
	}

	for _, p := range resp.Products {
		fmt.Printf("%s | %s | %s\n", p.ComponentCode, p.ComponentModelEn, p.ComponentBrandEn)
	}

	product, err := client.Product.Details(ctx, "C3900982")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Details: %s - %s\n", product.ComponentCode, product.ComponentName)
}
```

## API Overview

### Services

- `client.Search.Keyword(ctx, req)`
- `client.Product.Details(ctx, identifier)`

### Search request

```go
resp, err := client.Search.Keyword(ctx, &jlcpcb.SearchRequest{
	Keyword:       "capacitor",
	Page:          1,   // default: 1
	PageSize:      20,  // default: 50, max: 100
	PresaleType:   jlcpcb.PresaleTypeAny,   // Any, Stock, Buy, Post
	StockOnly:     false,                   // if true and PresaleTypeAny => stock search
	ComponentType: jlcpcb.ComponentTypeBase, // Any, Base, Expand
	Brands:        []string{"TDK", "Murata"},
	Packages:      []string{"0402"},           // the endpoint filters by package
	Attributes: []jlcpcb.FilterAttribute{
		{Name: "Package", Value: "0402"},
	},
	SortPrimary:   "",
	SortSecondary: "",
})
```

### Product details

```go
// Accepts JLC part code or MPN.
product, err := client.Product.Details(ctx, "CGJ2B2C0G1H390J050BA")
```

On a cache miss, Details sends one keyword search. The page size depends on the identifier:

- A JLC part code (`^[Cc][0-9]+$`, for example `C7593`) uses page size 10.
- Any other identifier, for example an MPN, uses page size 50.

Details returns the first result whose `componentCode` or `componentModelEn` is equal to the identifier.
The match ignores case.
If no result matches, Details returns the first result.

### Assembly library and ordering

```go
product, err := client.Product.Details(ctx, "C7593")
if err != nil {
	log.Fatal(err)
}

fmt.Println(product.LibraryType()) // "preferred"

if !product.Buyable() {
	fmt.Println("JLCPCB does not sell this part:", product.NoBuyReason)
}

for _, pb := range product.SortedBuyComponentPrices() {
	fmt.Printf("%d+: %.4f USD\n", pb.StartNumber, float64(pb.ProductPrice))
}
```

`LibraryType()` maps the raw fields to one class:

| `componentLibraryType` | `preferredComponentFlag` | `LibraryType()` |
|---|---|---|
| `base` | any | `LibraryTypeBasic` (`"basic"`) |
| `expand` | `true` | `LibraryTypePreferred` (`"preferred"`) |
| `expand` | `false` | `LibraryTypeExtended` (`"extended"`) |
| empty or other | any | `""` |

The match ignores case and surrounding white space.

`Buyable()` reads `isBuyComponent`:

| `isBuyComponent` | `Buyable()` |
|---|---|
| `"1"` | `true` |
| `"0"` | `false` |
| empty (unknown) | `true` |

`Buyable()` returns false only for `"0"`. When it returns false, `NoBuyReason` often gives the reason.
The raw `IsBuyComponent` field does not change.

The live API can send `buyComponentPrices` in a random order.
`SortedComponentPrices()` and `SortedBuyComponentPrices()` return copies sorted by `StartNumber`.
`SortPriceBreaks(breaks)` sorts any `[]PriceBreak` the same way. The sort is stable.
These functions do not change the raw fields.

### Images and datasheets

```go
if url := product.ImageURL(); url != "" {
	// Download the image now. The signed URL expires after 30 minutes.
}
for _, url := range product.DatasheetURLs() {
	// Try each URL until one download succeeds.
}
```

`ImageURL()` returns the first non-empty value of `productBigImageAccessIdUrl`, `minImageAccessIdUrl`, `componentImageUrl` and `minImage`.
The live API sends the image only in the signed `AccessIdUrl` fields.

`DatasheetURLs()` returns `dataManualUrl`, `dataManualFileAccessIdUrl` and `dataManualOfficialLink` in this order, without empty or duplicate values.

A signed URL expires 30 minutes after the response.
Download the file soon, and do not store the URL.
The default cache keeps a response for 5 minutes, so a cached URL is still valid.

## Public Types

### `SearchResponse`

- `Products []Product`
- `TotalCount int`
- `PageSize int`
- `Page int`

### `Product`

- `ComponentCode string`
- `ComponentModelEn string`
- `ComponentBrandEn string`
- `ComponentTypeEn string`
- `ComponentName string`
- `ComponentSpecificationEn string`
- `StockCount int`
- `MinPurchaseNum int`
- `ComponentPrices []PriceBreak`
- `BuyComponentPrices []PriceBreak`
- `Attributes []Attribute`
- `DataManualUrl string`
- `Describe string`
- `FirstSortName string`
- `SecondSortName string`
- `IsBuyComponent string`
- `UrlSuffix string`
- `LcscGoodsUrl string`
- `ComponentLibraryType string`: raw library type, `"base"` or `"expand"`
- `PreferredComponentFlag bool`: true for a preferred extended part
- `LossNumber int`: attrition, the extra parts that JLCPCB adds to an order
- `LeastPatchNumber int`: minimum placement quantity
- `CanPresaleNumber int`: quantity open for pre-order (can be negative)
- `NoBuyReason string`: reason that JLCPCB does not sell the part
- `EncapsulationNumber int`: parts per reel or package
- `PreMinPurchaseNum int`: minimum pre-order purchase quantity
- `ComponentAlternativesCode string`: alternative part code
- `AssemblyComponentFlag bool`: raw `assemblyComponentFlag` value
- `ComponentImageUrl string`, `MinImage string`: unsigned image URLs (often empty)
- `ProductBigImageAccessIdUrl string`, `MinImageAccessIdUrl string`: signed image URLs
- `DataManualFileAccessIdUrl string`: signed URL of the datasheet copy that JLCPCB hosts
- `DataManualOfficialLink string`: datasheet URL at the manufacturer (often empty)

A JSON `null` decodes to the zero value: `""`, `0`, or `false`.

Methods:

- `GetProductURL() string`
- `LibraryType() LibraryType`
- `Buyable() bool`
- `SortedComponentPrices() []PriceBreak`
- `SortedBuyComponentPrices() []PriceBreak`

### `LibraryType`

- `LibraryTypeBasic` (`"basic"`)
- `LibraryTypePreferred` (`"preferred"`)
- `LibraryTypeExtended` (`"extended"`)

### Functions

- `SortPriceBreaks([]PriceBreak) []PriceBreak`

## Client Options

- `WithHTTPClient(*http.Client)`
- `WithBaseURL(string)` (useful for tests)
- `WithRateLimit(float64)` requests/second
- `WithRetryConfig(RetryConfig)`
- `WithCache(Cache)`
- `WithCacheConfig(CacheConfig)`
- `WithoutCache()`

## Caching

Caching is enabled by default with in-memory cache:

- Search TTL: `5m`
- Details TTL: `5m`

Custom cache config:

```go
client := jlcpcb.NewClient(
	jlcpcb.WithCacheConfig(jlcpcb.CacheConfig{
		Enabled:    true,
		SearchTTL:  2 * time.Minute,
		DetailsTTL: 10 * time.Minute,
	}),
)
```

Clear cache:

```go
client.ClearCache()
```

## Errors

Sentinel errors:

- `jlcpcb.ErrInvalidRequest`
- `jlcpcb.ErrNotFound`
- `jlcpcb.ErrRateLimited`
- `jlcpcb.ErrServer`

Use with `errors.Is`:

```go
if errors.Is(err, jlcpcb.ErrNotFound) {
	// handle not found
}
```

Detailed API errors are returned as `*jlcpcb.APIError`.

## Retries and Rate Limits

Default retry behavior:

- retries: `3`
- backoff: exponential (`100ms` base, max `10s`, multiplier `2.0`)
- retried on: network timeout/errors and `429/500/502/503/504`

Default rate limit:

- `5` requests/second token bucket

## Testing

Unit tests:

```bash
go test ./...
```

Integration tests (live API, opt-in):

```bash
go test -tags=integration -run Integration ./...
```

## Changes in v1.1.0

- `Product` decodes the assembly library and ordering fields (see `Product`).
- New `Product.LibraryType()`, `Product.Buyable()`, `Product.SortedComponentPrices()` and `Product.SortedBuyComponentPrices()`.
- New `SortPriceBreaks` function.
- `Product.Details` uses page size 10 for a JLC part code. It still uses page size 50 for other identifiers.

## Breaking Changes in v1.0.0

- API moved to service-based access:
  - `client.KeywordSearch(...)` -> `client.Search.Keyword(...)`
  - `client.GetProductDetails(...)` -> `client.Product.Details(...)`
- Removed legacy/unused request fields:
  - `SearchRequest.IsAvailable`
  - `SearchRequest.PreferredOnly`
- Removed `WithCurrency(...)` (did not affect API responses)
- README/examples now use raw JLC field names (`ComponentCode`, `ComponentModelEn`, etc.)

## License

MIT, see [LICENSE](LICENSE).
