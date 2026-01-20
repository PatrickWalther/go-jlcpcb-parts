package jlcpcb

import (
	"testing"
)

// TestErrorFromCodeNotFound tests errorFromCode for 404.
func TestErrorFromCodeNotFound(t *testing.T) {
	err := errorFromCode(404, "product not found")

	if _, ok := err.(ErrProductNotFound); !ok {
		t.Fatalf("expected ErrProductNotFound, got %v", err)
	}
}

// TestErrorFromCodeRateLimit tests errorFromCode for 429.
func TestErrorFromCodeRateLimit(t *testing.T) {
	err := errorFromCode(429, "rate limited")

	if _, ok := err.(ErrRateLimited); !ok {
		t.Fatalf("expected ErrRateLimited, got %v", err)
	}
}

// TestErrorFromCodeInvalidInput tests errorFromCode for 400.
func TestErrorFromCodeInvalidInput(t *testing.T) {
	err := errorFromCode(400, "invalid input")

	if _, ok := err.(ErrInvalidInput); !ok {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

// TestErrProductNotFoundString tests the error message format.
func TestErrProductNotFoundString(t *testing.T) {
	err := ErrProductNotFound{ProductCode: "C12345"}
	errStr := err.Error()

	if !contains(errStr, "product") || !contains(errStr, "not") || !contains(errStr, "found") {
		t.Errorf("unexpected error string: %s", errStr)
	}
}

// TestErrRateLimitedString tests the rate limit error message.
func TestErrRateLimitedString(t *testing.T) {
	err := ErrRateLimited{}
	errStr := err.Error()

	if !contains(errStr, "rate") {
		t.Errorf("unexpected error string: %s", errStr)
	}
}

// TestErrInvalidInputString tests the invalid input error message.
func TestErrInvalidInputString(t *testing.T) {
	err := ErrInvalidInput{Message: "test error"}
	errStr := err.Error()

	if !contains(errStr, "invalid") {
		t.Errorf("unexpected error string: %s", errStr)
	}
}

// TestErrorsAreDistinct tests that different error types are distinct.
func TestErrorsAreDistinct(t *testing.T) {
	err1 := ErrProductNotFound{ProductCode: "C1"}
	err2 := ErrRateLimited{}
	err3 := ErrInvalidInput{Message: "test"}

	if err1.Error() == err2.Error() {
		t.Error("different error types should have different messages")
	}
	if err1.Error() == err3.Error() {
		t.Error("different error types should have different messages")
	}
	if err2.Error() == err3.Error() {
		t.Error("different error types should have different messages")
	}
}

// TestShouldRetry tests the retry decision logic.
func TestShouldRetry(t *testing.T) {
	tests := []struct {
		statusCode  int
		shouldRetry bool
	}{
		{429, true},  // Rate limited
		{503, true},  // Service unavailable
		{504, true},  // Gateway timeout
		{200, false}, // OK
		{404, false}, // Not found
		{500, false}, // Internal server error
	}

	// Create a dummy error for testing
	dummyErr := ErrInvalidInput{Message: "test"}
	for _, test := range tests {
		result := shouldRetry(dummyErr, test.statusCode)
		if result != test.shouldRetry {
			t.Errorf("shouldRetry(%d) = %v, expected %v", test.statusCode, result, test.shouldRetry)
		}
	}
}

// TestShouldRetryWithNilError tests that shouldRetry handles nil errors.
func TestShouldRetryWithNilError(t *testing.T) {
	result := shouldRetry(nil, 200)
	if result {
		t.Error("shouldRetry(nil, 200) should return false")
	}
}
