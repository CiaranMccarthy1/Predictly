// internal/market/rate_limiter.go
package market

import (
	"context"
	"sync"
	"time"
)

// TokenBucket implements a classic token-bucket rate limiter.
// Tokens are replenished at a fixed rate up to the bucket's capacity.
// Each call to Wait() blocks until a token is available or the context
// is cancelled.
type TokenBucket struct {
	mu       sync.Mutex
	tokens   float64
	capacity float64
	rate     float64 // tokens per second
	lastFill time.Time
}

// NewTokenBucket creates a rate limiter that permits `rate` operations
// per second with a burst capacity of `capacity`.
func NewTokenBucket(rate, capacity float64) *TokenBucket {
	return &TokenBucket{
		tokens:   capacity,
		capacity: capacity,
		rate:     rate,
		lastFill: time.Now(),
	}
}

// Wait blocks until a token is available. Returns an error only if the
// context is cancelled while waiting.
func (tb *TokenBucket) Wait(ctx context.Context) error {
	for {
		tb.mu.Lock()
		tb.refill()
		if tb.tokens >= 1 {
			tb.tokens--
			tb.mu.Unlock()
			return nil
		}
		// Calculate how long until the next token arrives
		deficit := 1.0 - tb.tokens
		waitDur := time.Duration(deficit / tb.rate * float64(time.Second))
		tb.mu.Unlock()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitDur):
			// loop back and try again
		}
	}
}

// refill adds tokens based on elapsed time. Must be called with mu held.
func (tb *TokenBucket) refill() {
	now := time.Now()
	elapsed := now.Sub(tb.lastFill).Seconds()
	tb.tokens += elapsed * tb.rate
	if tb.tokens > tb.capacity {
		tb.tokens = tb.capacity
	}
	tb.lastFill = now
}
