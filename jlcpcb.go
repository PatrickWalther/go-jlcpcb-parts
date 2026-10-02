// Package jlcpcb provides a Go client for JLCPCB parts search endpoints.
//
// JLCPCB does not provide an official public API for parts search. This package
// uses publicly accessible endpoints from https://jlcpcb.com/parts.
//
// Endpoints are organized into services:
//
//   - client.Search  — keyword, category and parametric search
//   - client.Product — product details lookup
package jlcpcb

// Version is the current package version.
const Version = "1.4.0"
