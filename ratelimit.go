package jlcpcb

import (
	"context"
	"sync"
	"time"
)

// RateLimiter implements token bucket rate limiting.
type RateLimiter struct {
	mu         sync.Mutex
	rate       float64   // requests per second
	maxTokens  float64   // maximum tokens in bucket
	tokens     float64   // current tokens
	lastUpdate time.Time // last time tokens were updated
}

// NewRateLimiter creates a new rate limiter.
func NewRateLimiter(rps float64) *RateLimiter {
	if rps <= 0 {
		rps = 1.0
	}
	return &RateLimiter{
		rate:       rps,
		maxTokens:  rps,
		tokens:     rps,
		lastUpdate: time.Now(),
	}
}

// Wait blocks until a token is available.
func (rl *RateLimiter) Wait(ctx context.Context) error {
	for {
		rl.mu.Lock()
		rl.refill()

		if rl.tokens >= 1.0 {
			rl.tokens -= 1.0
			rl.mu.Unlock()
			return nil
		}
		rl.mu.Unlock()

		// Calculate wait time
		waitTime := time.Duration(float64(time.Second) / rl.rate)
		select {
		case <-time.After(waitTime):
			continue
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// refill adds tokens based on time elapsed.
// Must be called with mu held.
func (rl *RateLimiter) refill() {
	now := time.Now()
	elapsed := now.Sub(rl.lastUpdate).Seconds()
	rl.tokens += elapsed * rl.rate

	if rl.tokens > rl.maxTokens {
		rl.tokens = rl.maxTokens
	}

	rl.lastUpdate = now
}
