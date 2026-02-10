package jlcpcb

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
)

var (
	// ErrInvalidRequest indicates invalid input or malformed requests.
	ErrInvalidRequest = errors.New("jlcpcb: invalid request")

	// ErrNotFound indicates a requested resource was not found.
	ErrNotFound = errors.New("jlcpcb: not found")

	// ErrRateLimited indicates API rate limits were exceeded.
	ErrRateLimited = errors.New("jlcpcb: rate limited")

	// ErrServer indicates server-side failures.
	ErrServer = errors.New("jlcpcb: server error")
)

// APIError represents an error returned by the JLCPCB API.
type APIError struct {
	StatusCode int
	Code       int
	Message    string
	Details    string
}

func (e *APIError) Error() string {
	msg := strings.TrimSpace(e.Message)
	if msg == "" {
		msg = "API error"
	}
	if details := strings.TrimSpace(e.Details); details != "" {
		return fmt.Sprintf("jlcpcb: %s (status %d, code %d): %s", msg, e.StatusCode, e.Code, details)
	}
	return fmt.Sprintf("jlcpcb: %s (status %d, code %d)", msg, e.StatusCode, e.Code)
}

// Unwrap returns a sentinel error category for errors.Is matching.
func (e *APIError) Unwrap() error {
	code := e.Code
	if code == 0 {
		code = e.StatusCode
	}

	switch code {
	case http.StatusBadRequest:
		return ErrInvalidRequest
	case http.StatusNotFound:
		return ErrNotFound
	case http.StatusTooManyRequests:
		return ErrRateLimited
	default:
		if code >= 500 && code <= 599 {
			return ErrServer
		}
	}
	return nil
}

// shouldRetry determines if a request should be retried.
func shouldRetry(err error, statusCode int) bool {
	if err == nil {
		return false
	}

	switch statusCode {
	case http.StatusTooManyRequests, http.StatusInternalServerError,
		http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	}

	if errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout() || netErr.Temporary()
	}

	var apiErr *APIError
	if errors.As(err, &apiErr) {
		switch apiErr.Code {
		case 429, 500, 502, 503, 504:
			return true
		}
	}

	return false
}
