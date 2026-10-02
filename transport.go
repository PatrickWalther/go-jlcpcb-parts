package jlcpcb

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// apiEnvelope is the top-level JLCPCB API wrapper.
type apiEnvelope struct {
	Code    int             `json:"code"`
	Message json.RawMessage `json:"message"`
}

// do performs an HTTP request to the search base URL plus path, with retry
// support, and parses API-level errors.
func (c *Client) do(ctx context.Context, method, path string, params url.Values, reqBody interface{}, result interface{}) error {
	return c.doURL(ctx, method, c.baseURL+path, params, reqBody, result)
}

// doAPI performs an HTTP request to the API root plus path, with retry
// support, and parses API-level errors.
func (c *Client) doAPI(ctx context.Context, method, path string, params url.Values, reqBody interface{}, result interface{}) error {
	return c.doURL(ctx, method, c.apiRoot+path, params, reqBody, result)
}

// doURL performs an HTTP request to endpoint with retry support and parses
// API-level errors.
func (c *Client) doURL(ctx context.Context, method, endpoint string, params url.Values, reqBody interface{}, result interface{}) error {
	return c.doURLWithRetry(ctx, shouldRetry, method, endpoint, params, reqBody, result)
}

// doURLWithRetry performs an HTTP request to endpoint and parses API-level
// errors. It retries a failed attempt when retry returns true.
func (c *Client) doURLWithRetry(ctx context.Context, retry func(error, int) bool, method, endpoint string, params url.Values, reqBody interface{}, result interface{}) error {
	return c.withRetry(ctx, retry, func() (int, error) {
		return c.doOnce(ctx, method, endpoint, params, reqBody, result)
	})
}

// withRetry calls attempt until it succeeds, until retry returns false for
// its error, or until the attempts of the retry configuration are used. It
// waits for the rate limiter before each attempt and for the backoff before
// each retry. attempt returns the HTTP status code (0 when no response
// arrived) and the error of the attempt.
//
// withRetry always makes at least one attempt, also when MaxRetries is less
// than 0.
func (c *Client) withRetry(ctx context.Context, retry func(error, int) bool, attempt func() (int, error)) error {
	var lastErr error
	maxAttempts := max(c.retryConfig.MaxRetries+1, 1)

	for i := 0; i < maxAttempts; i++ {
		if i > 0 {
			backoff := c.retryConfig.calculateBackoff(i - 1)
			if err := sleep(ctx, backoff); err != nil {
				return err
			}
		}

		if err := c.rateLimiter.Wait(ctx); err != nil {
			return fmt.Errorf("jlcpcb: rate limiter wait failed: %w", err)
		}

		statusCode, err := attempt()
		if err == nil {
			return nil
		}

		lastErr = err
		if !retry(err, statusCode) || i >= maxAttempts-1 {
			return err
		}
	}

	return lastErr
}

func (c *Client) doOnce(ctx context.Context, method, endpoint string, params url.Values, reqBody interface{}, result interface{}) (int, error) {
	reqURL := endpoint
	if len(params) > 0 {
		reqURL += "?" + params.Encode()
	}

	var bodyReader io.Reader
	if reqBody != nil {
		payload, err := json.Marshal(reqBody)
		if err != nil {
			return 0, fmt.Errorf("%w: failed to marshal request body: %v", ErrInvalidRequest, err)
		}
		bodyReader = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, reqURL, bodyReader)
	if err != nil {
		return 0, fmt.Errorf("jlcpcb: failed to create request: %w", err)
	}

	c.setHeaders(req, reqBody != nil)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("jlcpcb: request failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	respBody, err := readResponseBody(resp)
	if err != nil {
		return resp.StatusCode, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, &APIError{
			StatusCode: resp.StatusCode,
			Code:       resp.StatusCode,
			Message:    http.StatusText(resp.StatusCode),
			Details:    string(respBody),
		}
	}

	var envelope apiEnvelope
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return resp.StatusCode, fmt.Errorf("jlcpcb: failed to parse response envelope: %w", err)
	}

	if envelope.Code != 200 {
		return resp.StatusCode, &APIError{
			StatusCode: resp.StatusCode,
			Code:       envelope.Code,
			Message:    envelopeMessage(envelope.Message),
			Details:    string(respBody),
		}
	}

	if result != nil {
		if err := json.Unmarshal(respBody, result); err != nil {
			return resp.StatusCode, fmt.Errorf("jlcpcb: failed to parse response body: %w", err)
		}
	}

	return resp.StatusCode, nil
}

func (c *Client) setHeaders(req *http.Request, hasBody bool) {
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Origin", "https://jlcpcb.com")
	req.Header.Set("Referer", "https://jlcpcb.com/parts")
	if hasBody {
		req.Header.Set("Content-Type", "application/json")
	}
}

func readResponseBody(resp *http.Response) ([]byte, error) {
	reader := resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gzReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("jlcpcb: failed to create gzip reader: %w", err)
		}
		defer func() {
			_ = gzReader.Close()
		}()
		reader = gzReader
	}

	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("jlcpcb: failed to read response body: %w", err)
	}
	return body, nil
}

func envelopeMessage(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}

	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		return asString
	}

	var asAny interface{}
	if err := json.Unmarshal(raw, &asAny); err == nil {
		serialized, marshalErr := json.Marshal(asAny)
		if marshalErr == nil {
			return string(serialized)
		}
	}

	return string(raw)
}
