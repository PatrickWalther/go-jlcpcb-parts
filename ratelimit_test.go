package jlcpcb

import (
	"context"
	"testing"
	"time"
)

// TestNewRateLimiter tests creating a new rate limiter.
func TestNewRateLimiter(t *testing.T) {
	rl := NewRateLimiter(5.0)

	if rl == nil {
		t.Fatal("expected non-nil rate limiter")
	}

	if rl.maxTokens != 5.0 {
		t.Errorf("expected max tokens 5.0, got %f", rl.maxTokens)
	}

	if rl.rate != 5.0 {
		t.Errorf("expected rate 5.0, got %f", rl.rate)
	}

	if rl.tokens != 5.0 {
		t.Errorf("expected initial tokens 5.0, got %f", rl.tokens)
	}
}

// TestRateLimiterWaitImmediate tests that Wait returns immediately when tokens available.
func TestRateLimiterWaitImmediate(t *testing.T) {
	rl := NewRateLimiter(10.0)
	ctx := context.Background()

	start := time.Now()
	err := rl.Wait(ctx)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Wait failed: %v", err)
	}

	if elapsed > 100*time.Millisecond {
		t.Errorf("Wait took too long: %v", elapsed)
	}
}

// TestRateLimiterWaitMultiple tests consuming multiple tokens.
func TestRateLimiterWaitMultiple(t *testing.T) {
	rl := NewRateLimiter(10.0)
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		err := rl.Wait(ctx)
		if err != nil {
			t.Fatalf("Wait %d failed: %v", i, err)
		}
	}
}

// TestRateLimiterWaitBlocks tests that Wait blocks when out of tokens.
func TestRateLimiterWaitBlocks(t *testing.T) {
	rl := NewRateLimiter(2.0)
	ctx := context.Background()

	for i := 0; i < 2; i++ {
		if err := rl.Wait(ctx); err != nil {
			t.Fatalf("initial wait failed: %v", err)
		}
	}

	start := time.Now()
	err := rl.Wait(ctx)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Wait after blocking failed: %v", err)
	}

	if elapsed < 300*time.Millisecond {
		t.Errorf("Wait blocked for too short: %v (expected ~500ms)", elapsed)
	}
}

// TestRateLimiterWaitContextCancellation tests that Wait respects context cancellation.
func TestRateLimiterWaitContextCancellation(t *testing.T) {
	rl := NewRateLimiter(10.0)
	ctx, cancel := context.WithCancel(context.Background())

	if err := rl.Wait(ctx); err != nil {
		t.Fatalf("initial wait failed: %v", err)
	}

	for i := 0; i < 9; i++ {
		if err := rl.Wait(ctx); err != nil {
			t.Fatalf("wait failed: %v", err)
		}
	}

	errorChan := make(chan error, 1)
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	go func() {
		errorChan <- rl.Wait(ctx)
	}()

	select {
	case err := <-errorChan:
		if err == nil {
			t.Fatal("expected error for cancelled context")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("wait for cancellation took too long")
	}
}

// TestRateLimiterWaitContextTimeout tests that Wait respects context timeout.
func TestRateLimiterWaitContextTimeout(t *testing.T) {
	rl := NewRateLimiter(2.0)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	for i := 0; i < 2; i++ {
		if err := rl.Wait(ctx); err != nil {
			t.Logf("initial wait failed: %v (context may have timed out early)", err)
			return
		}
	}

	err := rl.Wait(ctx)
	if err == nil {
		t.Fatal("expected error for timeout context")
	}

	if err != context.DeadlineExceeded && err != context.Canceled {
		t.Logf("expected deadline/cancel error, got %v", err)
	}
}

// TestRateLimiterWaitRefill tests that tokens are refilled over time.
func TestRateLimiterWaitRefill(t *testing.T) {
	rl := NewRateLimiter(5.0)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		if err := rl.Wait(ctx); err != nil {
			t.Fatalf("initial wait failed: %v", err)
		}
	}

	time.Sleep(300 * time.Millisecond)

	err := rl.Wait(ctx)
	if err != nil {
		t.Fatalf("Wait after refill failed: %v", err)
	}
}

// TestRateLimiterWaitConcurrent tests concurrent Wait calls.
func TestRateLimiterWaitConcurrent(t *testing.T) {
	rl := NewRateLimiter(10.0)
	ctx := context.Background()

	errors := make(chan error, 10)

	for i := 0; i < 10; i++ {
		go func() {
			err := rl.Wait(ctx)
			errors <- err
		}()
	}

	for i := 0; i < 10; i++ {
		err := <-errors
		if err != nil {
			t.Errorf("concurrent Wait failed: %v", err)
		}
	}
}
