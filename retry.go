package jlcpcb

import (
	"context"
	"math"
	"time"
)

// RetryConfig contains retry configuration.
type RetryConfig struct {
	MaxRetries      int           // Maximum number of retries
	InitialBackoff  time.Duration // Initial backoff duration
	MaxBackoff      time.Duration // Maximum backoff duration
	BackoffMultiplier float64       // Multiplier for exponential backoff
}

// DefaultRetryConfig returns a default retry configuration.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:        3,
		InitialBackoff:    100 * time.Millisecond,
		MaxBackoff:        10 * time.Second,
		BackoffMultiplier: 2.0,
	}
}

// calculateBackoff calculates the backoff duration for a given attempt.
func (rc RetryConfig) calculateBackoff(attempt int) time.Duration {
	backoff := float64(rc.InitialBackoff) * math.Pow(rc.BackoffMultiplier, float64(attempt))
	maxBackoffFloat := float64(rc.MaxBackoff)

	if backoff > maxBackoffFloat {
		backoff = maxBackoffFloat
	}

	return time.Duration(backoff)
}

// sleep sleeps for the specified duration, respecting context cancellation.
func sleep(ctx context.Context, duration time.Duration) error {
	select {
	case <-time.After(duration):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
