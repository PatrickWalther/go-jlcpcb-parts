package jlcpcb

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"testing"
)

func TestAPIErrorUnwrap(t *testing.T) {
	tests := []struct {
		name string
		err  *APIError
		want error
	}{
		{
			name: "invalid request",
			err:  &APIError{StatusCode: http.StatusBadRequest, Code: 400},
			want: ErrInvalidRequest,
		},
		{
			name: "not found",
			err:  &APIError{StatusCode: http.StatusNotFound, Code: 404},
			want: ErrNotFound,
		},
		{
			name: "rate limited",
			err:  &APIError{StatusCode: http.StatusTooManyRequests, Code: 429},
			want: ErrRateLimited,
		},
		{
			name: "server error",
			err:  &APIError{StatusCode: http.StatusInternalServerError, Code: 500},
			want: ErrServer,
		},
		{
			name: "rejected request",
			err:  &APIError{StatusCode: http.StatusOK, Code: 101},
			want: ErrRejected,
		},
	}

	for _, tt := range tests {
		if !errors.Is(tt.err, tt.want) {
			t.Fatalf("%s: expected errors.Is(..., %v)", tt.name, tt.want)
		}
	}
}

func TestAPIErrorString(t *testing.T) {
	err := &APIError{
		StatusCode: 503,
		Code:       503,
		Message:    "service unavailable",
		Details:    "upstream timeout",
	}
	msg := err.Error()
	if msg == "" {
		t.Fatal("expected non-empty error message")
	}
	if !strings.Contains(msg, "service unavailable") {
		t.Fatalf("expected message to contain API message, got %q", msg)
	}
}

func TestShouldRetryByStatusCode(t *testing.T) {
	tests := []struct {
		status int
		retry  bool
	}{
		{429, true},
		{500, true},
		{502, true},
		{503, true},
		{504, true},
		{404, false},
		{400, false},
	}

	baseErr := &APIError{Message: "test"}
	for _, tt := range tests {
		got := shouldRetry(baseErr, tt.status)
		if got != tt.retry {
			t.Fatalf("status %d: expected retry=%v, got %v", tt.status, tt.retry, got)
		}
	}
}

func TestShouldRetryByAPICode(t *testing.T) {
	tests := []struct {
		code  int
		retry bool
	}{
		{429, true},
		{500, true},
		{502, true},
		{503, true},
		{504, true},
		{404, false},
		{101, false},
	}

	for _, tt := range tests {
		got := shouldRetry(&APIError{Code: tt.code}, 0)
		if got != tt.retry {
			t.Fatalf("API code %d: expected retry=%v, got %v", tt.code, tt.retry, got)
		}
	}
}

func TestShouldRetryContextErrors(t *testing.T) {
	if shouldRetry(context.Canceled, 0) {
		t.Fatal("expected context.Canceled to not be retryable")
	}
	if !shouldRetry(context.DeadlineExceeded, 0) {
		t.Fatal("expected context.DeadlineExceeded to be retryable")
	}
}

func TestShouldRetryNetworkTimeout(t *testing.T) {
	netErr := &net.DNSError{IsTimeout: true}
	if !shouldRetry(netErr, 0) {
		t.Fatal("expected timeout network error to be retryable")
	}
}
