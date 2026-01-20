# go-jlcpcb-parts

[![Go Reference](https://pkg.go.dev/badge/github.com/PatrickWalther/go-jlcpcb-parts.svg)](https://pkg.go.dev/github.com/PatrickWalther/go-jlcpcb-parts)
[![Go Report Card](https://goreportcard.com/badge/github.com/PatrickWalther/go-jlcpcb-parts)](https://goreportcard.com/report/github.com/PatrickWalther/go-jlcpcb-parts)
[![Tests](https://github.com/PatrickWalther/go-jlcpcb-parts/actions/workflows/test.yml/badge.svg)](https://github.com/PatrickWalther/go-jlcpcb-parts/actions)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A Go client library for the [JLCPCB](https://jlcpcb.com) parts API. Provides access to the JLCPCB parts catalog with support for searching, retrieving product details, caching, rate limiting, and automatic retries.

> **Note**: JLCPCB does not have an official public API. This library uses the publicly accessible endpoints from https://jlcpcb.com/parts that work without authentication.

## Requirements

- **Go 1.21+** (tested on Go 1.21 and 1.23)
- No external dependencies

## Features

- **Product Search**: Search for parts by keyword with pagination support
- **Product Details**: Retrieve detailed information for specific parts by SKU
- **Caching**: Built-in in-memory caching with TTL support
- **Rate Limiting**: Token bucket rate limiting to respect API quotas
- **Retry Logic**: Automatic exponential backoff retry on failures
- **Flexible Configuration**: Extensive client options for customization

## Installation

```bash
go get github.com/PatrickWalther/go-jlcpcb-parts
```

## Quick Start

```bash
# Initialize a new Go module (if needed)
go mod init example.com/myapp

# Get the library
go get github.com/PatrickWalther/go-jlcpcb-parts

# Run tests to verify installation
go test github.com/PatrickWalther/go-jlcpcb-parts/...
```

## Usage

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/PatrickWalther/go-jlcpcb-parts"
)

func main() {
    // Create a new client
    client := jlcpcb.NewClient()

    ctx := context.Background()

    // Search for products
    results, err := client.KeywordSearch(ctx, jlcpcb.SearchRequest{
        Keyword:  "MPM3506",
        PageSize: 10,
    })
    if err != nil {
        log.Fatal(err)
    }

    for _, product := range results.Products {
        fmt.Printf("%s - %s: %s\n", 
            product.SKU, 
            product.MPN,
            product.Title)
    }

    // Get product details
    product, err := client.GetProductDetails(ctx, "C12345")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Product: %s by %s\n", product.Title, product.Manufacturer)
    fmt.Printf("Stock: %d, Min Order: %d\n", product.Stock, product.MinOrder)
    fmt.Printf("URL: %s\n", product.GetProductURL())

    // Access specifications
    for _, attr := range product.Attributes {
        fmt.Printf("  %s: %s\n", attr.Name, attr.Value)
    }
}
```

## Client Options

```go
// Custom HTTP client
client := jlcpcb.NewClient(
    jlcpcb.WithHTTPClient(&http.Client{Timeout: 60*time.Second}))

// Custom currency (affects pricing)
client := jlcpcb.NewClient(jlcpcb.WithCurrency("EUR"))

// Custom rate limit (requests per second)
client := jlcpcb.NewClient(jlcpcb.WithRateLimit(10.0))

// Enable caching
cache := jlcpcb.NewMemoryCache()
client := jlcpcb.NewClient(jlcpcb.WithCache(cache))

// Custom retry configuration
client := jlcpcb.NewClient(jlcpcb.WithRetryConfig(jlcpcb.RetryConfig{
    MaxRetries:     5,
    InitialBackoff: 1 * time.Second,
    MaxBackoff:     60 * time.Second,
    BackoffMultiplier: 2.0,
}))
```

## API Reference

### Client Methods

#### `KeywordSearch(ctx context.Context, req SearchRequest) (*SearchResponse, error)`

Searches for products by keyword with pagination support.

**Parameters:**
- `ctx`: Context for request cancellation
- `req`: SearchRequest with:
  - `Keyword`: Part name or keyword (required)
  - `CurrentPage`: Page number (default: 1)
  - `PageSize`: Results per page (default: 50)
  - `IsAvailable`: Only available parts (default: false)

**Returns:** SearchResponse with matched products and total count

#### `GetProductDetails(ctx context.Context, sku string) (*Product, error)`

Retrieves detailed information for a specific product.

**Parameters:**
- `ctx`: Context for request cancellation
- `sku`: JLCPCB SKU/part number (required)

**Returns:** Product with full details

### Product Search

```go
results, err := client.KeywordSearch(ctx, jlcpcb.SearchRequest{
    Keyword:  "capacitor 100nF",
    PageSize: 30, // max 100
})

// Access results
for _, p := range results.Products {
    fmt.Println(p.SKU, p.MPN, p.Manufacturer)
}
```

### Product Details

```go
product, err := client.GetProductDetails(ctx, "C12345")

// Product info
fmt.Println(product.SKU)           // "C12345"
fmt.Println(product.MPN)           // Manufacturer part number
fmt.Println(product.Manufacturer)  // Manufacturer name
fmt.Println(product.Title)         // Description
fmt.Println(product.Stock)         // Available stock
fmt.Println(product.DatasheetURL)  // Datasheet URL
fmt.Println(product.Package)       // Package/footprint
fmt.Println(product.GetProductURL()) // JLCPCB product page

// Pricing
for _, pb := range product.PriceList {
    fmt.Printf("Qty %d+: %.4f %s\n", pb.Quantity, pb.Price, pb.Currency)
}

// Specifications
for _, attr := range product.Attributes {
    fmt.Printf("%s: %s\n", attr.Name, attr.Value)
}
```

## Data Types

### Product

| Field | Type | Description |
|-------|------|-------------|
| SKU | string | JLCPCB SKU/part number |
| MPN | string | Manufacturer part number |
| Manufacturer | string | Manufacturer name |
| Title | string | Product title/description |
| DatasheetURL | string | Datasheet URL |
| Image | string | Product image URL |
| Stock | int | Available stock |
| MinOrder | int | Minimum order quantity |
| PriceList | []PriceBreak | Quantity price breaks |
| Attributes | []Attribute | Product specifications |
| Package | string | Package/footprint |
| Category | string | Product category |
| Rating | float64 | Product rating |
| IsAvailable | bool | In-stock status |

### PriceBreak

| Field | Type | Description |
|-------|------|-------------|
| Quantity | int | Minimum quantity for this price |
| Price | float64 | Unit price |
| Currency | string | Currency code (e.g., "USD") |

### Attribute

| Field | Type | Description |
|-------|------|-------------|
| Name | string | Attribute name |
| Value | string | Attribute value |

## Error Handling

```go
if errors.Is(err, jlcpcb.ErrProductNotFound) {
    // Product not found (404)
}
if errors.Is(err, jlcpcb.ErrRateLimited) {
    // Rate limited (429)
}
if errors.Is(err, jlcpcb.ErrInvalidInput) {
    // Invalid input (400)
}
```

## Testing

This library includes comprehensive unit and integration tests:

```bash
# Run all tests (unit tests only, ~1.8s)
go test ./...

# Run with coverage report
go test ./... -cover

# Generate coverage HTML report
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out

# Run integration tests (makes real API calls, ~1.8s)
go test -run Integration ./...

# Run specific test
go test -run TestKeywordSearchBasic ./...
```

## Development

### Code Quality

```bash
# Run linter
golangci-lint run ./...

# Run type checker
go vet ./...

# Format code
go fmt ./...
```

### Project Structure

```
.
├── *.go              # Main library code
├── *_test.go         # Unit tests
├── go.mod            # Module definition
├── README.md         # Documentation
└── .gitignore        # Git ignore file
```

## License

MIT License - see [LICENSE](LICENSE) for details.

## Contributing

Contributions are welcome! Please:
1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Ensure `go test ./...` and `golangci-lint run ./...` pass
5. Submit a pull request

## Acknowledgments

- [JLCPCB](https://jlcpcb.com) for providing the parts API
- [go-lcsc](https://github.com/PatrickWalther/go-lcsc) for the API endpoint discovery approach
