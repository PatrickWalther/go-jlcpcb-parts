# go-jlcpcb-parts

[![Go Reference](https://pkg.go.dev/badge/github.com/PatrickWalther/go-jlcpcb-parts.svg)](https://pkg.go.dev/github.com/PatrickWalther/go-jlcpcb-parts)
[![Go Report Card](https://goreportcard.com/badge/github.com/PatrickWalther/go-jlcpcb-parts)](https://goreportcard.com/report/github.com/PatrickWalther/go-jlcpcb-parts)
[![Tests](https://github.com/PatrickWalther/go-jlcpcb-parts/actions/workflows/test.yml/badge.svg)](https://github.com/PatrickWalther/go-jlcpcb-parts/actions)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A Go client for the JLCPCB parts endpoints that https://jlcpcb.com/parts uses: part search, part detail, facet counts, category names, assembly calculators and file downloads.

> Note: JLCPCB does not publish an official public parts API. This library uses publicly accessible endpoints and can break if JLCPCB changes their internal contracts. See [Risks and limits](#risks-and-limits).

## Requirements

- Go 1.23+
- No external dependencies (stdlib only)

## Installation

```bash
go get github.com/PatrickWalther/go-jlcpcb-parts
```

## Features

- Service-based API (`client.Search`, `client.Product`, `client.Assembly`, `client.Category`, `client.File`)
- Keyword part search with filters and pagination
- Search without a keyword by category and attribute values
- Server-side filters for library type, stock, PCBA type and datasheets, and sort options
- Page walk that keeps the totals of the first page
- Category tree with part counts
- Facet counts by library type, availability, category, package, brand and attribute value
- Attribute value lookup that maps an input such as `0.1uF` to the exact facet string `100nF`
- Category names by numeric category id
- Product details lookup by JLC code or MPN
- Exact part detail by JLC code, with assembly, MSL, ECCN and category id fields
- Batch part detail by part id (250 ids for each request), with the pre-order ladder and stable file access ids
- Assembly library class (basic, preferred, extended) and ordering fields
- Price ladders sorted by quantity
- Parts-order quote (stock or pre-order, minimum quantity, price ladder)
- PCBA type eligibility and lead time fields
- PCBA attrition and order quantity calculators, and a local estimate with the same rule
- Image and datasheet URLs that do not expire (file access ids), and a check for signed URLs
- File download by access id, with the content type found from the bytes
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

- `client.Search.Keyword(ctx, req)`: search with a keyword (the keyword is required)
- `client.Search.Query(ctx, req)`: search with an optional keyword
- `client.Search.Pages(ctx, req, fn)`: call `fn` for each page of a search
- `client.Product.Details(ctx, identifier)`: keyword search for one part
- `client.Product.Detail(ctx, code)`: exact detail record of one JLC part code
- `client.Product.DetailsByIDs(ctx, ids)`: detail records of many parts by part id
- `client.Search.Facets(ctx, req)`: part counts by library type, availability, category, package, brand and attribute value
- `client.Assembly.Attrition(ctx, rows)`: attrition (extra parts for wastage) of each placement row
- `client.Assembly.OrderQuantities(ctx, rows)`: part quantity that JLCPCB charges for each placement row
- `client.Category.Info(ctx, id)`: category names of a numeric category id
- `client.File.Open(ctx, accessID)`: download a part image or a datasheet copy by file access id

### Search request

```go
resp, err := client.Search.Keyword(ctx, &jlcpcb.SearchRequest{
	Keyword:       "capacitor",
	Page:          1,   // default: 1
	PageSize:      20,  // default: 50, max: 1000
	PresaleType:   jlcpcb.PresaleTypeAny,    // Any, Stock, Buy, Post
	StockOnly:     false,                    // stockFlag: only parts with stock > 0
	ComponentType: jlcpcb.ComponentTypeBase, // Any, Base, Expand
	Brands:        []string{"TDK", "Murata"},
	Packages:      []string{"0402"}, // the endpoint filters by package
})
```

The server applies all filters together: a part must match each filter.

### Basic and preferred parts in one query

```go
resp, err := client.Search.Keyword(ctx, &jlcpcb.SearchRequest{
	Keyword:          "SOIC-8",
	ComponentType:    jlcpcb.ComponentTypeBase,
	IncludePreferred: true, // preferredComponentFlag
})
for _, p := range resp.Products {
	fmt.Println(p.ComponentCode, p.LibraryType()) // "basic" or "preferred"
}
```

With `ComponentTypeBase`, `IncludePreferred` adds the preferred extended parts to the basic parts.
The total of this query is the basic total plus the preferred total, so one query replaces two.
`IncludePreferred` without a library type returns only the preferred extended parts.

### Category and parametric search

`Query` accepts the same request as `Keyword`, but the keyword is optional.
This example returns the basic and preferred 0402 ceramic capacitors of 100nF or 1uF that have stock and that Economic PCBA accepts:

```go
resp, err := client.Search.Query(ctx, &jlcpcb.SearchRequest{
	Category: jlcpcb.Category{
		Parent: "Capacitors",
		Leaf:   "Multilayer Ceramic Capacitors MLCC - SMD/SMT",
	},
	Packages: []string{"0402"},
	AttributeFilters: []jlcpcb.AttributeFilter{
		{Name: "Capacitance", Values: []string{"100nF", "1uF"}},
		{Name: "Voltage Rating", Values: []string{"25V", "50V"}},
	},
	LibraryTypes:     []jlcpcb.ComponentType{jlcpcb.ComponentTypeBase},
	IncludePreferred: true,
	StockOnly:        true,
	PCBA:             jlcpcb.PCBAFilterEconomic,
	Sort:             jlcpcb.SortByStock,
	SortDescending:   true,
	CategoryCounts:   true,
})
```

| Field | Wire field | Effect |
|---|---|---|
| `Category` | `firstSortName` (parent), `secondSortName` (leaf) | Category filter by name. The server ignores numeric category ids. |
| `AttributeFilters` | `componentAttributeList` | Sent as `[{"<name>":["<value>",...]}]`. The values of one filter are OR. The filters are AND. The server matches exact strings: `"100nF"` matches, `"0.1uF"` does not. |
| `LibraryTypes` | `componentLibTypes` | Library type filter (`base`, `expand`). |
| `IncludePreferred` | `preferredComponentFlag` | With the `base` library type: basic OR preferred parts. Alone: preferred extended parts only. |
| `PresaleTypes` | `presaleTypes` | Availability classes. The class `stock` includes some rows with stock 0. |
| `StockOnly` | `stockFlag` | Only parts with stock > 0. When `PresaleType` and `PresaleTypes` are empty, it also sends `presaleType` `stock`. |
| `MinStock` | `startStockNumber` | Only parts with stock >= `MinStock`. |
| `PCBA` | `pcbAType` | `PCBAFilterEconomic` (1) removes the parts that Economic PCBA does not accept. `PCBAFilterStandard` is 2. |
| `Sort`, `SortDescending` | `sortMode`, `sortASC` | `SortByModel`, `SortByStock` or `SortByPrice`, ascending or descending. |
| `HasDatasheet` | `dateSheet` | Only parts with a datasheet. |
| `CategoryCounts` | `searchType` 3 | `SearchResponse.Categories` holds the category tree with part counts. |

`Attributes` (one value for each entry), `SortPrimary` and `SortSecondary` are deprecated.
They still work. Use `AttributeFilters`, `Category.Parent` and `Category.Leaf`.
`SortPrimary` and `SortSecondary` are category filters, not sort fields.

The server rejects a request body that it cannot read with envelope code 101.
`errors.Is(err, jlcpcb.ErrRejected)` is then true.

### Pages

```go
err := client.Search.Pages(ctx, &jlcpcb.SearchRequest{
	Category: jlcpcb.Category{Parent: "Resistors", Leaf: "Chip Resistor - Surface Mount"},
	PageSize: 500,
}, func(page *jlcpcb.SearchResponse) error {
	fmt.Printf("page %d of %d: %d parts\n", page.Page, page.Pages, len(page.Products))
	return nil
})
```

`Pages` starts at `req.Page` and stops after the last page or at the first page without products.
When `fn` returns an error, `Pages` stops and returns that error.
A page past the last page returns `TotalCount` 0 and `Pages` 0, so `Pages` copies these values from the first page into each later page.

### Facets

```go
facets, err := client.Search.Facets(ctx, &jlcpcb.FacetRequest{
	ParentID: 2,    // Capacitors
	LeafID:   2929, // Multilayer Ceramic Capacitors MLCC - SMD/SMT
	Packages: []string{"0402"},
	Attributes: []jlcpcb.AttributeFilter{
		{Name: "Voltage Rating", Values: []string{"16V"}},
		{Name: "Capacitance", Values: []string{"100nF"}},
		{Name: "Temperature Coefficient", Values: []string{"X7R"}},
	},
})
if err != nil {
	log.Fatal(err)
}
fmt.Println(facets.Total, facets.Counts.Basic, facets.Counts.Extended) // 151 1 150
fmt.Println(facets.Presale[jlcpcb.PresaleTypeStock])                   // 60

capacitance, _ := facets.Param("Capacitance")
fmt.Println(capacitance.Canonical("0.1uF")) // [100nF]
```

`Facets` sends one POST request to `filterComponentAttribute`, the endpoint of the filter panel of the JLCPCB part pages.
The cache keeps the answer for 15 minutes.

The facet endpoint takes numeric category ids, not names.
Get the ids from `ComponentDetail.ParentCategoryID` and `LeafCategoryID`, from `CategoryCount.ID`, or from `client.Category.Info`.

| Field | Wire field | Effect |
|---|---|---|
| `Keyword` | `keyword`, `queryString` | Optional search text. |
| `ParentID`, `LeafID` | `productTypeIdList`, `componentTypeIdList` | Category filter by numeric id. Set `ParentID` with `LeafID`. |
| `Packages`, `Brands` | `componentSpecificationList`, `componentBrandList` | Package and manufacturer filters. |
| `LibraryTypes` | `orderLibraryTypeList` | Library type filter. The facet endpoint ignores `componentLibTypes`. |
| `IncludePreferred` | `preferredComponentFlag` | With the `base` library type: basic OR preferred parts. |
| `PresaleTypes`, `PCBA`, `HasDatasheet` | `presaleTypes`, `pcbAType`, `dateSheet` | The same filters as in a search. |
| `Attributes` | `paramList` | Attribute filters. The server matches exact strings. |
| `FacetFor` | `nowCondition` | Removes one filter from the facet counts. |

The client sets `catalogLevel` from the request: 2 with `LeafID`, 1 with `ParentID` or `Keyword`, and 0 without them.

`Facets` holds these counts:

- `Total`: the parts that match all filters.
- `Counts`: basic, preferred, extended, Economic PCBA, Standard PCBA, datasheet, photo and mechanical assembly parts. `Extended` includes the preferred parts.
- `Presale`: the parts of each availability class (`stock`, `buy`, `post`).
- `Categories`: the category tree with part counts. Skip the tree "Others" (id 35) to classify a part.
- `Packages`, `Brands`: the part count of each package and manufacturer.
- `Params`: the values of each attribute with part counts, the units and the unit factors.

`FacetFor` takes a parameter name or one of the `FacetFor` constants (`FacetForLibraryType`, `FacetForStock`, `FacetForPCBA`, `FacetForInformation`, `FacetForPackage`).
The server then counts every facet without that filter, so one call shows which values exist for the row.
`Total` stays filtered.
The library counts become flags: 1 for a class with parts, 0 for a class without parts.
`Facets.CountsAreFlags` is then true.

The server counts a part twice for a symmetric "±" value, for example "±10%".
`ParamValue.Count` holds the true part count, and `ParamValue.DocCount` holds the raw count.

`ParamFacet.Canonical(input)` returns the facet values with the same meaning as `input`.
Use the result in a filter, because the server matches exact strings: `"100nF"` gives 866 parts, and `"0.1uF"` gives 0.

- An equal string matches.
- A number with a unit compares in the base unit of the attribute: `"0.1uF"`, `"100000pF"` and `"100n"` give `["100nF"]`, and `"10k"` gives `["10kΩ"]`.
- When input or a value has no number, the string match ignores case: `"x7r"` gives `["X7R"]`.
- The unit match ignores case, but `m` (milli) never matches `M` (mega): `"1mΩ"` does not give `"1MΩ"`, and `"1.8mhz"` does not give `"1.8MHz"`.
- A range compares both ends: `"-40°C~+125°C"` gives `"-40℃~+125℃"`, `"-40℃~+125℃@(Ta)"` and `"-40℃~+125℃@(Tj)"`.
- A symmetric range needs the sign: `"±1%"` gives `["±1%"]`, but `"1%"` gives no value.
- A number without a unit uses the base unit (the unit with the factor 1, for example pF for Capacitance).

### Category names

```go
info, err := client.Category.Info(ctx, 2929)
if err != nil {
	log.Fatal(err)
}
fmt.Println(info.ParentName, "/", info.LeafName) // Capacitors / Multilayer Ceramic Capacitors MLCC - SMD/SMT

resp, err := client.Search.Query(ctx, &jlcpcb.SearchRequest{Category: info.Category()})
```

`Info` sends one GET request to `sort/info/{id}`.
The cache keeps the answer for 24 hours.
For a first-level category, `LeafID` is 0 and `LeafName` is empty.
An id of 0 or less gives `ErrInvalidRequest` and sends no request.
The server answers an unknown id with envelope code 500.
`Info` then returns an error that matches `ErrNotFound` and `ErrServer`, and it does not retry.

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
Use `Detail` when you need the record of an exact part code.

### Exact and batch part detail

```go
detail, err := client.Product.Detail(ctx, "C2040")
if errors.Is(err, jlcpcb.ErrNotFound) {
	log.Fatal("JLCPCB does not know this part code")
}
if err != nil {
	log.Fatal(err)
}
fmt.Println(detail.LCSCComponentID, detail.AssemblyProcess, detail.MoistureSensitivityLevelEn)

details, err := client.Product.DetailsByIDs(ctx, []int64{1877, 2392, 3349129})
if err != nil {
	log.Fatal(err)
}
for id, d := range details {
	product := d.Product()
	quote := product.PartsOrderQuote(5000)
	fmt.Println(id, d.ComponentCode, len(d.BuyPrices), quote.MinQty)
}
```

`Detail` sends one GET request to `getComponentDetail?componentCode=<CODE>`.
The code is not case sensitive.
A code that is not `C` followed by digits gives `ErrInvalidRequest` and sends no request.
The server answers an unknown code with a record in which every field is null. `Detail` then returns `ErrNotFound`.

`DetailsByIDs` sends POST requests to `compareComponentDetails`.
The part id is `Product.ComponentID`, `ComponentDetail.LCSCComponentID` or the LCSC `productId`.

- It drops ids of 0 or less and duplicate ids.
- It sends at most 250 ids in each request.
- It returns a map keyed by part id. The server sorts the records by part code, not in request order.
- The server omits unknown ids, unlisted parts and parts with `componentStatus` `"no"`. A missing id is not an error.
- The server keeps parts that JLCPCB does not sell. Check `IsBuyComponent`.
- The cache keeps each record by part id. A call requests only the ids that are not in the cache.

Both methods sort `Prices` and `BuyPrices` by `StartNumber`.

| Field | `Detail` | `DetailsByIDs` |
|---|---|---|
| `BuyPrices` (pre-order ladder) | nil | set |
| File access ids | empty | set on most parts |
| Signed URL lifetime | 30 minutes | 60 minutes |
| `URLSuffix` | empty | set |
| Unlisted parts (`componentStatus` `"no"`) | returned | omitted |

A detail record sends the parent category in `firstSortName` and the leaf in `secondSortName`.
`ComponentDetail` names these fields `ParentCategory` and `LeafCategory`.
`ParentCategoryID` and `LeafCategoryID` hold the numeric category ids.
A detail record has no preferred flag, no lead time and no merge code.
`CanPresaleNumber` is 0 where a search row sends a negative value.

`ComponentDetail.Product()` returns the record as a `Product`:

- `Prices` goes to `ComponentPrices`, and `BuyPrices` goes to `BuyComponentPrices`.
- The category names go to `FirstSortName` and `SecondSortName` in the order of a search row. Thus `Product.Category()` returns the same parent and leaf.
- `LCSCComponentID` goes to `ComponentID`.
- `PreferredComponentFlag` is false. Thus `LibraryType()` returns `LibraryTypeExtended` for a preferred extended part.

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
image := product.StableImageURL()     // large image, 900x900
thumb := product.StableThumbnailURL() // small image, 96x96
sheet := product.StableDatasheetURL() // PDF copy that JLCPCB hosts
if jlcpcb.IsSignedURL(thumb) {
	// The record has no stable URL. Download the file now, and do not store the URL.
}
for _, url := range product.DatasheetURLs() {
	// Try each URL until one download succeeds.
}
```

`FileURL(accessID)` returns `https://jlcpcb.com/api/file/downloadByFileSystemAccessId/<id>`.
This URL needs no headers and has no expiry time.
The same id gave the same bytes 45 minutes later, and an id that was 54 days old still gave a file.
A byte comparison over more days was not done.
`FileURL` returns `""` when the id is not all digits.

The stable methods use these fields, in this order. They return `""` when no field is set.

| Method | 1. File access id | 2. URL without expiry | 3. Signed URL (expires) |
|---|---|---|---|
| `StableImageURL()` | `ProductBigImageAccessId` | `ComponentImageUrl`, when it is an LCSC URL | `ProductBigImageAccessIdUrl` |
| `StableThumbnailURL()` | `MinImageAccessId` | `MinImage`, when it is an LCSC URL | `MinImageAccessIdUrl` |
| `StableDatasheetURL()` | `DataManualFileAccessId` | `DataManualUrl`, when it is a JLCPCB file URL | `DataManualFileAccessIdUrl` |

`ComponentDetail` has the same three methods.

- The batch detail (`DetailsByIDs`) sends the file access ids for most parts. The v2 search and `Detail` send no access ids. Thus for a search row, the stable methods usually return a signed URL. Use `IsSignedURL` to find it.
- Some parts have no JLCPCB image. These records send LCSC image URLs in `componentImageUrl` and `minImage` (for example C6186). LCSC image URLs have no expiry time.
- Some records send an LCSC folder URL without a file name, for example `https://assets.lcsc.com/images/lcsc/96x96/`. The methods skip such a URL.
- `StableDatasheetURL()` does not return an LCSC datasheet URL, because a `www.lcsc.com/datasheet/` URL gives an HTML viewer page, not a PDF.

A signed URL expires 30 minutes (search and `Detail`) or 60 minutes (`DetailsByIDs`) after the response.
Download the file soon, and do not store the URL.
The default cache keeps a response for 5 minutes, so a signed URL from the cache has at least 25 minutes left.

`DatasheetURLs()` returns all datasheet URLs, best first, without empty or duplicate values:

1. `FileURL(DataManualFileAccessId)`: the stable URL of the PDF copy that JLCPCB hosts.
2. `dataManualFileAccessIdUrl`: a signed URL of the same copy.
3. `dataManualUrl`: usually an LCSC URL. A `www.lcsc.com/datasheet/` URL gives an HTML viewer page.
4. `dataManualOfficialLink`: the manufacturer page.

`ImageURL()` returns the first value of `productBigImageAccessIdUrl`, `minImageAccessIdUrl`, `componentImageUrl` and `minImage` that has a file name.
It prefers the signed URLs, so do not store its result.

### File downloads

```go
body, info, err := client.File.Open(ctx, detail.ProductBigImageAccessID)
if errors.Is(err, jlcpcb.ErrNotFound) {
	log.Fatal("JLCPCB has no file with this access id")
}
if err != nil {
	log.Fatal(err)
}
defer body.Close()

fmt.Println(info.ContentType, info.FileName, info.Size) // image/jpeg C1525-正面.jpg 55710
```

`Open` sends one GET request to the file URL below the API root.
It waits for the rate limiter and retries like the other requests.
The caller must close the body.
The cache does not keep files.

- `ContentType`: the server sends `application/x-msdownload` for images, so `Open` does not use the server value. `Open` reads the first 512 bytes and finds the type with `http.DetectContentType`. The body still starts at the first byte. `Open` does not check the type, so check `ContentType` before you use the file.
- `FileName`: the name from `Content-Disposition`, without a folder prefix. `Open` decodes percent-encoded UTF-8, plain ASCII and the RFC 5987 form (`filename*=UTF-8''...`). Bytes that are not valid UTF-8, for example a raw GBK name, become `_`.
- `Size`: the `Content-Length`, or -1 when the server sends no length.

An id that is not all digits gives `ErrInvalidRequest`, and `Open` sends no request.
The server answers an unknown id with HTTP 200 and the body `{"code":500,...}`.
`Open` then returns an error that matches `ErrNotFound` and `ErrServer`, and it does not retry.
An empty body also gives `ErrNotFound`.

### Parts order, PCBA type and lead time

```go
quote := product.PartsOrderQuote(5000)
if quote.PreOrder {
	fmt.Println("JLCPCB fills the whole quantity as a pre-order")
}
fmt.Println("minimum quantity:", quote.MinQty)
for _, pb := range quote.Ladder {
	fmt.Printf("%d+: %.4f USD\n", pb.StartNumber, float64(pb.ProductPrice))
}

if !product.ComponentProductType.AllowsEconomic() {
	fmt.Println("Economic PCBA does not accept this part")
}
if days, ok := product.LeadTimeDays(); ok {
	fmt.Println("estimated lead time:", days, "days")
}
parent, leaf := product.Category()
```

`PartsOrderQuote(qty)` copies the rule of the JLCPCB parts shop:

- An order of `qty <= CanPresaleNumber` ships from stock. The ladder is `ComponentPrices`.
- A larger order is a pre-order for the whole quantity. The ladder is `BuyComponentPrices`.
- The minimum quantity is `max(MinPurchaseNum, PreMinPurchaseNum)` when `qty > CanPresaleNumber`, when `CanPresaleNumber <= 0`, or when `CanPresaleNumber < MinPurchaseNum`. Otherwise it is `MinPurchaseNum`.

This rule is for a parts order.
The limit for a PCBA order can differ.
JLCPCB keeps part of its stock for PCBA orders, so a PCBA order probably draws on `StockCount`.
This PCBA limit is inferred from UI text and is not verified.
`PartsOrderQuote` does not check `Buyable()`.

`ComponentProductType` tells which PCBA types accept the part: `PCBAEligibilityBoth` (0), `PCBAEligibilityEconomicOnly` (1) or `PCBAEligibilityStandardOnly` (2).

`LeadTimeDays()` reads `estimateDate`.
Only v2 search rows send it, and only some rows with stock.
It is null on pre-order rows with stock 0, so it is not a lead time for such a part.

`Category()` returns the parent and the leaf category in the true order.
A search row sends the leaf in `firstSortName` and the parent in `secondSortName`.
A search request and a detail record use the reverse order.

### PCBA attrition and order quantity

```go
rows := []jlcpcb.PlacementRow{{
	Side:                jlcpcb.AssemblySideSingle,
	Boards:              1000, // pasteNumber
	PerBoard:            100,  // componentDesignator
	LossNumber:          product.LossNumber,
	LeastPatchNumber:    product.LeastPatchNumber,
	EncapsulationNumber: product.EncapsulationNumber,
}}

attrition, err := client.Assembly.Attrition(ctx, rows) // [190] for loss 10 and reel 10000
qty, err := client.Assembly.OrderQuantities(ctx, rows) // [100190]

// The same rule without a request:
fmt.Println(jlcpcb.EstimateAttrition(rows[0], jlcpcb.DefaultWastageCoefficient)) // 190
fmt.Println(jlcpcb.EstimateOrderQty(rows[0], jlcpcb.DefaultWastageCoefficient))  // 100190
```

`Attrition` and `OrderQuantities` send one POST request to the calculators of the JLCPCB parts shop (`calculateAttrition` and `calculateComponentOrderQty`).
They return one value for each row, in the order of the rows.
An empty row list returns an empty list and sends no request.

The calculators answer a bad row with 0 and no error.
Thus both methods validate each row first with `PlacementRow.Validate()`:

- `Boards` and `PerBoard` must be more than 0.
- `Side` must be `AssemblySideSingle`, `AssemblySideBoth` or empty. An empty side means single.
- `LossNumber`, `LeastPatchNumber` and `EncapsulationNumber` must not be less than 0.

A row that is not valid gives `ErrInvalidRequest`, and the method sends no request.

`EstimateAttrition` and `EstimateOrderQty` compute the rule of the calculators:

```text
need      = Boards × PerBoard
attrition = k × (LossNumber + floor(coef × max(0, need − EncapsulationNumber)))   k = 2 for "both", else 1
qty       = max(need + attrition, LeastPatchNumber)
```

The rule gave the answer of the live calculators for all 68 recorded rows.
`DefaultWastageCoefficient` is 0.002, the value of the JLCPCB config key `SYSTEM.smt_config.smt_wastage_coefficient`.
The link between this key and the calculators is inferred.
A `coef` that is not a finite number above 0 uses the default.
The rule applies no minimum order quantity and no reel rounding.
The parts shop uses the same calculators. The use of this rule for a PCBA order is inferred.

## Public Types

### `SearchResponse`

- `Products []Product`
- `TotalCount int`
- `PageSize int`
- `Page int`
- `Pages int`: number of pages for this page size
- `Categories []CategoryCount`: category tree with part counts (only with `CategoryCounts`)

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
- `FirstSortName string`: leaf category in a search row (see `Category()`)
- `SecondSortName string`: parent category in a search row (see `Category()`)
- `IsBuyComponent string`
- `UrlSuffix string`
- `LcscGoodsUrl string`
- `ComponentLibraryType string`: raw library type, `"base"` or `"expand"`
- `PreferredComponentFlag bool`: true for a preferred extended part
- `LossNumber int`: base term of the attrition (extra parts) of an assembly order. JLCPCB adds more parts for large quantities and for double-sided assembly.
- `LeastPatchNumber int`: minimum placement quantity
- `CanPresaleNumber int`: largest parts order from stock. A larger order is a pre-order. Search rows can send a negative value.
- `NoBuyReason string`: reason that JLCPCB does not sell the part
- `EncapsulationNumber int`: parts per reel or package
- `PreMinPurchaseNum int`: minimum pre-order purchase quantity
- `ComponentAlternativesCode string`: alternative part code
- `AssemblyComponentFlag bool`: raw `assemblyComponentFlag` value
- `ComponentImageUrl string`, `MinImage string`: unsigned image URLs (often empty)
- `ProductBigImageAccessIdUrl string`, `MinImageAccessIdUrl string`: signed image URLs
- `DataManualFileAccessIdUrl string`: signed URL of the datasheet copy that JLCPCB hosts
- `DataManualOfficialLink string`: datasheet URL at the manufacturer (often empty)
- `ComponentProductType PCBAEligibility`: PCBA types that accept the part
- `EstimateDate FlexString`: estimated lead time in days (often empty, see `LeadTimeDays()`)
- `InitialPrice FlexFloat64`: `ComponentPrices` tier price at `MinPurchaseNum`
- `AllowPostFlag bool`: true when the customer can consign the part to JLCPCB
- `MergedComponentCode string`: merge or alternative part code. Active parts have it too, so it is not an EOL marker.
- `ReplaceUrlSuffix string`: part page URL suffix of `MergedComponentCode`
- `ProductBigImageAccessId string`, `MinImageAccessId string`, `DataManualFileAccessId string`: stable file access ids. The v2 search sends null for them today.

A JSON `null` decodes to the zero value: `""`, `0`, or `false`.

Methods:

- `GetProductURL() string`
- `LibraryType() LibraryType`
- `Buyable() bool`
- `SortedComponentPrices() []PriceBreak`
- `SortedBuyComponentPrices() []PriceBreak`
- `Category() (parent, leaf string)`
- `PartsOrderQuote(qty int) PartsOrderQuote`
- `LeadTimeDays() (int, bool)`
- `ImageURL() string`, `DatasheetURLs() []string`
- `StableImageURL() string`, `StableThumbnailURL() string`, `StableDatasheetURL() string`

### `ComponentDetail`

- `LCSCComponentID int64`: numeric part id (`lcscComponentId`)
- `ComponentCode`, `ComponentModelEn`, `ComponentBrandEn`, `ComponentName`, `ComponentSpecificationEn`, `Describe string`
- `Attributes []Attribute`
- `ComponentStatus string`: `"yes"` for a listed part
- `ParentCategory string`, `LeafCategory string`: category names (`firstSortName`, `secondSortName`)
- `ParentCategoryID int`, `LeafCategoryID int`: numeric category ids (`firstTypeNameId`, `secondTypeNameId`)
- `ComponentLibraryType string`, `AssemblyComponentFlag bool`
- `AssemblyProcess string`: `"SMT"` or `"THT"`
- `AssemblyMode string`: for example `"smtWeld"` or `"manualWeld"` (hand soldering)
- `ComponentProductType PCBAEligibility`
- `XrayFlag bool`: true when the part needs an X-ray inspection
- `SpecialComponentFee FlexFloat64`: extra assembly fee in USD (0 for most parts)
- `NeedAuditFlag bool`, `OrderInstructionEnglish string`
- `ComponentDesignator string`: designator prefix. It is not reliable.
- `MoistureSensitivityLevelEn string`, `EccnCode string`
- `StockCount`, `CanPresaleNumber`, `MinPurchaseNum`, `PreMinPurchaseNum int`
- `InitialPrice FlexFloat64`
- `Prices []PriceBreak`, `BuyPrices []PriceBreak`: sorted by `StartNumber`
- `LossNumber`, `LeastPatchNumber int`
- `EncapsulationNumber int`, `EncapsulationUnit string`: parts per reel, tube or package
- `WarehouseCode string`
- `IsBuyComponent string`, `NoBuyReason string`, `AllowPostFlag bool`
- `ComponentAlternativesCode string`, `AlternativesLCSCComponentID int64`, `ReplaceURLSuffix string`: replacement part
- `ProductBigImageAccessID`, `MinImageAccessID`, `DataManualFileAccessID string`: stable file access ids
- `ProductBigImageSignedURL`, `MinImageSignedURL`, `DataManualFileSignedURL string`: signed URLs
- `ComponentImageURL`, `MinImageURL`, `DataManualURL`, `DataManualOfficialLink`, `LCSCGoodsURL string`
- `URLSuffix string`: part page URL suffix (only from `DetailsByIDs`)

Methods:

- `Category() (parent, leaf string)`
- `Product() Product`
- `StableImageURL() string`, `StableThumbnailURL() string`, `StableDatasheetURL() string`

### `Facets`

- `Total int`
- `Counts LibraryCounts`: `Basic`, `Preferred`, `Extended`, `Economic`, `Standard`, `Datasheet`, `Photo`, `MechanicalAssembly`
- `CountsAreFlags bool`: true when `FacetFor` is set
- `Presale map[PresaleType]int`
- `Categories []CategoryCount`
- `Packages []Bucket`, `Brands []Bucket`: `Value`, `Name`, `Count`
- `Params []ParamFacet`

Methods:

- `Param(name string) (ParamFacet, bool)`

### `ParamFacet`

- `Name string`
- `Range bool`: true for a numeric attribute
- `Units []string`, `UnitScale map[string]float64`: units and their factors to the base unit
- `Values []ParamValue`: `Value`, `Count`, `DocCount`, `Norm`, `IntervalStart`, `IntervalEnd`

Methods:

- `Canonical(input string) []string`

### `PlacementRow`

- `Side AssemblySide` (`assemblySide`): `AssemblySideSingle` or `AssemblySideBoth`
- `Boards int` (`pasteNumber`)
- `PerBoard int` (`componentDesignator`)
- `LossNumber int`, `LeastPatchNumber int`, `EncapsulationNumber int`

Methods:

- `Validate() error`

### `CategoryInfo`

- `ParentID int`, `ParentName string`
- `LeafID int`, `LeafName string`

Methods:

- `Category() Category`

### `FileInfo`

- `ContentType string`: media type from the first 512 bytes, for example `image/jpeg` or `application/pdf`
- `FileName string`: file name from `Content-Disposition`, without a folder prefix
- `Size int64`: `Content-Length`, or -1 when it is not known

### `LibraryType`

- `LibraryTypeBasic` (`"basic"`)
- `LibraryTypePreferred` (`"preferred"`)
- `LibraryTypeExtended` (`"extended"`)

### Functions

- `SortPriceBreaks([]PriceBreak) []PriceBreak`
- `EstimateAttrition(PlacementRow, float64) int`
- `EstimateOrderQty(PlacementRow, float64) int`
- `FileURL(accessID string) string`: stable download URL of a file access id
- `IsSignedURL(url string) bool`: true for a signed URL that expires

## Client Options

- `WithHTTPClient(*http.Client)`
- `WithBaseURL(string)`: base URL of the search endpoint (useful for tests)
- `WithAPIRoot(string)`: root of the JLCPCB web API, default `https://jlcpcb.com/api`. The detail, facet, calculator, category and file endpoints use it. When `WithBaseURL` is not set, the search endpoint also uses it.
- `WithRateLimit(float64)` requests/second
- `WithRetryConfig(RetryConfig)`
- `WithCache(Cache)`
- `WithCacheConfig(CacheConfig)`
- `WithoutCache()`

When the `WithBaseURL` value ends with `/overseas-pcb-order/v1/shoppingCart/smtGood` and `WithAPIRoot` is not set, the client uses the part before this path as the API root.
`FileURL` and the stable URL methods always use the default root, because a stored URL must work without the client.

## Caching

Caching is enabled by default with in-memory cache:

- Search TTL: `5m`
- Details TTL: `5m`. It applies to `Details`, `Detail` and the records of `DetailsByIDs`.
- Facets TTL: `15m` (`CacheConfig.FacetsTTL`). 0 uses the default.
- Category TTL: `24h` (`CacheConfig.CategoryTTL`). 0 uses the default.

The calculators and the file downloads do not use the cache.

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
- `jlcpcb.ErrRejected`: the server rejected the request with envelope code 101. The message of this code is generic, so the cause is not known.

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
- not retried: an envelope error in an HTTP 200 answer of `Category.Info` and `File.Open`, because the answer for an unknown id does not change
- a `MaxRetries` value less than 0 counts as 0: the client sends one request

Default rate limit:

- `5` requests/second token bucket

## Risks and limits

- **Undocumented endpoints.** JLCPCB does not document these endpoints, and it can change or remove them at any time. The weekly integration workflow runs live contract tests to find a change early. Keep a fallback for each feature that you build on this library.
- **Errors in HTTP 200 answers.** The server sends many errors with HTTP status 200 and an envelope code, for example code 101 for a request body that it cannot read, and code 500 for an unknown category id or file access id. The client maps the envelope codes to the sentinel errors. Some endpoints give no error for bad input: the calculators answer a bad row with 0, an unknown attribute name in a filter gives 0 parts, and the batch detail omits unknown ids. The client validates the input where it can. When a filter gives 0 parts, check the filter names.
- **Signed URLs expire.** The signed image and datasheet URLs expire 30 or 60 minutes after the response. Do not store them. Store a `FileURL` URL or the downloaded bytes.
- **Inferred rules.** Some rules come from the code or the text of the JLCPCB web pages, not from a contract: the PCBA stock limit of `PartsOrderQuote`, the link between `DefaultWastageCoefficient` and the calculators, and the use of the calculator rule for a PCBA order. The doc comment of each function tells which part is inferred.
- **Rate limits.** JLCPCB publishes no rate limit. No live request during the development of this library got an HTTP 429, but third parties report HTTP 403 for request bursts. The default limit is 5 requests per second. Use a lower limit for bulk jobs, and use `DetailsByIDs` to send fewer requests.
- **Site terms.** This library does not check the terms of use of jlcpcb.com. Read them before a bulk download of data, images or datasheets.

## Testing

Unit tests:

```bash
go test ./...
```

Integration tests (live API, opt-in, read-only, at most 1 request per second for the contract tests):

```bash
go test -tags=integration -run Integration ./...
```

## Changes in v1.4.0

- Fix: `Attributes` used a body shape that the server rejects with envelope code 101. The search now sends `componentAttributeList` as `[{"<name>":["<value>",...]}]`.
- Fix: envelope code 101 maps to the new `ErrRejected`.
- Fix: the `Version` constant and the comments of `FirstSortName`, `SecondSortName`, `CanPresaleNumber` and `LossNumber`.
- The `PageSize` cap is 1000 (before: 100). The default stays 50.
- New `Search.Query` and `Search.Pages`.
- New request fields: `IncludePreferred`, `Category`, `AttributeFilters`, `LibraryTypes`, `PresaleTypes`, `MinStock`, `PCBA`, `Sort`, `SortDescending`, `HasDatasheet` and `CategoryCounts`.
- New response fields: `SearchResponse.Pages` and `SearchResponse.Categories`.
- New `Product` fields: `ComponentProductType`, `EstimateDate`, `InitialPrice`, `AllowPostFlag`, `MergedComponentCode`, `ReplaceUrlSuffix` and the file access ids.
- New `Product.Category()`, `Product.PartsOrderQuote()` and `Product.LeadTimeDays()`.
- `Attributes`, `FilterAttribute`, `SortPrimary` and `SortSecondary` are deprecated. They still work.
- New `client.Product.Detail()`: the exact detail record of one JLC part code.
- New `client.Product.DetailsByIDs()`: the detail records of many parts by part id, 250 ids for each request.
- New `ComponentDetail` type with `Category()` and `Product()`.
- New `WithAPIRoot()` client option.
- New `client.Search.Facets()` with `FacetRequest`, `Facets`, `ParamFacet` and `ParamFacet.Canonical()`.
- New `client.Assembly.Attrition()` and `client.Assembly.OrderQuantities()` with `PlacementRow`.
- New `EstimateAttrition()`, `EstimateOrderQty()` and `DefaultWastageCoefficient`.
- New `client.Category.Info()` with `CategoryInfo`.
- New `CacheConfig.FacetsTTL` and `CacheConfig.CategoryTTL`.
- New `FileURL()` and `IsSignedURL()`.
- New `StableImageURL()`, `StableThumbnailURL()` and `StableDatasheetURL()` on `Product` and `ComponentDetail`.
- New `client.File.Open()` with `FileInfo`.
- Changed: `DatasheetURLs()` returns the stable access id URL first, then the signed URL, then `dataManualUrl`, then `dataManualOfficialLink`. Before, `dataManualUrl` came first, but a `www.lcsc.com/datasheet/` URL gives an HTML viewer page.
- Fix: `ImageURL()` skips a URL without a file name, for example the LCSC folder URL that some records send.
- Fix: a `RetryConfig.MaxRetries` value less than 0 sent no request and returned no error. The client now sends one request.

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
