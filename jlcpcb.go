// Package jlcpcb provides a Go client for JLCPCB parts search endpoints.
//
// JLCPCB does not provide an official public API for parts search. This package
// uses publicly accessible endpoints from https://jlcpcb.com/parts.
//
// Endpoints are organized into services:
//
//   - client.Search   — keyword, category and parametric search, and facet
//     counts
//   - client.Product  — product details lookup by keyword, exact part detail
//     by component code, and batch part detail by part id
//   - client.Assembly — PCBA attrition and order quantity calculators
//   - client.Category — category names by numeric category id
package jlcpcb

// Version is the current package version.
const Version = "1.4.0"
