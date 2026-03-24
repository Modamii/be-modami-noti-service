package gateway

import (
	"sync"
	"time"
)

// RateLimiter implements a per-key token bucket rate limiter.
type RateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	rate    int           // tokens added per interval
	burst   int           // max tokens (bucket capacity)
	interval time.Duration // refill interval
}

type bucket struct {
	tokens   int
	lastFill time.Time
}

func NewRateLimiter(rate, burst int, interval time.Duration) *RateLimiter {
	return &RateLimiter{
		buckets:  make(map[string]*bucket),
		rate:     rate,
		burst:    burst,
		interval: interval,
	}
}

// Allow checks whether the given key is within rate limits.
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	b, ok := rl.buckets[key]
	if !ok {
		b = &bucket{tokens: rl.burst, lastFill: now}
		rl.buckets[key] = b
	}

	// Refill tokens based on elapsed time
	elapsed := now.Sub(b.lastFill)
	refill := int(elapsed/rl.interval) * rl.rate
	if refill > 0 {
		b.tokens += refill
		if b.tokens > rl.burst {
			b.tokens = rl.burst
		}
		b.lastFill = now
	}

	if b.tokens <= 0 {
		return false
	}
	b.tokens--
	return true
}

// Cleanup removes stale entries older than maxAge. Call periodically.
func (rl *RateLimiter) Cleanup(maxAge time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	cutoff := time.Now().Add(-maxAge)
	for key, b := range rl.buckets {
		if b.lastFill.Before(cutoff) {
			delete(rl.buckets, key)
		}
	}
}
