package jlcpcb

import (
	"context"
	"testing"
	"time"
)

// TestDefaultRetryConfig tests default retry configuration.
func TestDefaultRetryConfig(t *testing.T) {
	config := DefaultRetryConfig()

	if config.MaxRetries != 3 {
		t.Errorf("expected MaxRetries 3, got %d", config.MaxRetries)
	}

	if config.InitialBackoff == 0 {
		t.Error("expected non-zero InitialBackoff")
	}

	if config.MaxBackoff == 0 {
		t.Error("expected non-zero MaxBackoff")
	}

	if config.BackoffMultiplier == 0 {
		t.Error("expected non-zero BackoffMultiplier")
	}
}

// TestCalculateBackoff tests backoff calculation.
func TestCalculateBackoff(t *testing.T) {
	config := RetryConfig{
		InitialBackoff:    100 * time.Millisecond,
		MaxBackoff:        1 * time.Second,
		BackoffMultiplier: 2.0,
	}

	backoff := config.calculateBackoff(0)
	if backoff != 100*time.Millisecond {
		t.Errorf("expected 100ms for attempt 0, got %v", backoff)
	}

	backoff = config.calculateBackoff(1)
	if backoff != 200*time.Millisecond {
		t.Errorf("expected 200ms for attempt 1, got %v", backoff)
	}

	backoff = config.calculateBackoff(2)
	if backoff != 400*time.Millisecond {
		t.Errorf("expected 400ms for attempt 2, got %v", backoff)
	}
}

// TestCalculateBackoffCap tests that backoff respects max cap.
func TestCalculateBackoffCap(t *testing.T) {
	config := RetryConfig{
		InitialBackoff:    100 * time.Millisecond,
		MaxBackoff:        500 * time.Millisecond,
		BackoffMultiplier: 2.0,
	}

	// After 3 attempts: 100ms, 200ms, 400ms, 800ms (capped at 500ms)
	backoff := config.calculateBackoff(3)
	if backoff > 500*time.Millisecond {
		t.Errorf("expected max backoff 500ms, got %v", backoff)
	}
}

// TestSleep tests the sleep function respects context.
func TestSleep(t *testing.T) {
	ctx := context.Background()

	start := time.Now()
	err := sleep(ctx, 100*time.Millisecond)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("sleep failed: %v", err)
	}

	if elapsed < 100*time.Millisecond {
		t.Errorf("sleep finished too early: %v", elapsed)
	}
}

// TestSleepCancellation tests that sleep respects context cancellation.
func TestSleepCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	err := sleep(ctx, 5*time.Second)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error for cancelled context")
	}

	if elapsed > 200*time.Millisecond {
		t.Errorf("sleep took too long after cancellation: %v", elapsed)
	}
}

// TestSleepTimeout tests that sleep respects context timeout.
func TestSleepTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := sleep(ctx, 5*time.Second)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error for timeout context")
	}

	if elapsed > 200*time.Millisecond {
		t.Errorf("sleep took too long before timeout: %v", elapsed)
	}
}
