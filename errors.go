package jlcpcb

import "fmt"

// ErrProductNotFound indicates the product was not found.
type ErrProductNotFound struct {
	ProductCode string
}

func (e ErrProductNotFound) Error() string {
	return fmt.Sprintf("product not found: %s", e.ProductCode)
}

// ErrInvalidInput indicates invalid input parameters.
type ErrInvalidInput struct {
	Message string
}

func (e ErrInvalidInput) Error() string {
	return fmt.Sprintf("invalid input: %s", e.Message)
}

// ErrRateLimited indicates the client was rate limited.
type ErrRateLimited struct{}

func (e ErrRateLimited) Error() string {
	return "rate limited by API"
}

// errorFromCode converts an API error code to an error.
func errorFromCode(code int, message string) error {
	switch code {
	case 200:
		return nil
	case 404:
		return ErrProductNotFound{ProductCode: message}
	case 429:
		return ErrRateLimited{}
	case 400:
		return ErrInvalidInput{Message: message}
	default:
		return fmt.Errorf("API error (code %d): %s", code, message)
	}
}

// shouldRetry determines if a request should be retried.
func shouldRetry(err error, statusCode int) bool {
	if err == nil {
		return false
	}

	// Retry on specific status codes
	switch statusCode {
	case 429: // Too Many Requests
		return true
	case 503: // Service Unavailable
		return true
	case 504: // Gateway Timeout
		return true
	default:
		return false
	}
}
