package jlcpcb

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestTransportDoSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code":    200,
			"message": nil,
			"data": map[string]interface{}{
				"value": "ok",
			},
		})
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL), WithHTTPClient(server.Client()))

	var resp struct {
		Data struct {
			Value string `json:"value"`
		} `json:"data"`
	}
	err := client.do(context.Background(), http.MethodPost, "/", nil, map[string]string{"x": "y"}, &resp)
	if err != nil {
		t.Fatalf("expected successful request, got %v", err)
	}
	if resp.Data.Value != "ok" {
		t.Fatalf("expected parsed value 'ok', got %q", resp.Data.Value)
	}
}

func TestTransportDoAPICodeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code":    429,
			"message": "too many requests",
			"data":    nil,
		})
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL), WithHTTPClient(server.Client()), WithRetryConfig(RetryConfig{
		MaxRetries:        0,
		InitialBackoff:    1,
		MaxBackoff:        1,
		BackoffMultiplier: 1,
	}))

	var resp interface{}
	err := client.do(context.Background(), http.MethodGet, "/", nil, nil, &resp)
	if err == nil {
		t.Fatal("expected API error")
	}
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("expected ErrRateLimited, got %v", err)
	}
}

func TestTransportRetriesOn503(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := calls.Add(1)
		if current == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("temporary failure"))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code":    200,
			"message": nil,
			"data": map[string]interface{}{
				"value": "recovered",
			},
		})
	}))
	defer server.Close()

	client := NewClient(
		WithBaseURL(server.URL),
		WithHTTPClient(server.Client()),
		WithRetryConfig(RetryConfig{
			MaxRetries:        1,
			InitialBackoff:    1,
			MaxBackoff:        1,
			BackoffMultiplier: 1,
		}),
	)

	var resp struct {
		Data struct {
			Value string `json:"value"`
		} `json:"data"`
	}
	err := client.do(context.Background(), http.MethodGet, "/", nil, nil, &resp)
	if err != nil {
		t.Fatalf("expected successful retry, got %v", err)
	}
	if calls.Load() != 2 {
		t.Fatalf("expected 2 calls (1 retry), got %d", calls.Load())
	}
	if resp.Data.Value != "recovered" {
		t.Fatalf("expected recovered value, got %q", resp.Data.Value)
	}
}

// TestTransportNegativeMaxRetries checks that a negative MaxRetries still
// sends one request. Before, the client sent no request and returned no
// error.
func TestTransportNegativeMaxRetries(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		_, _ = w.Write([]byte(`{"code":500,"message":"server error","data":null}`))
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL), WithHTTPClient(server.Client()), WithRetryConfig(RetryConfig{
		MaxRetries:        -1,
		InitialBackoff:    1,
		MaxBackoff:        1,
		BackoffMultiplier: 1,
	}))

	var resp interface{}
	if err := client.do(context.Background(), http.MethodGet, "/", nil, nil, &resp); !errors.Is(err, ErrServer) {
		t.Fatalf("error = %v, want ErrServer", err)
	}
	if n := requests.Load(); n != 1 {
		t.Errorf("server got %d requests, want 1", n)
	}
}
