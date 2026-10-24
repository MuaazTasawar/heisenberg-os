package middleware

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/heisenberg-os/backend/pkg/response"
)

const maxTrackedClients = 100_000

type window struct {
	count int
	reset time.Time
}

// RateLimiter is a fixed-window limiter keyed by client IP. It lives in memory, so each
// server process counts separately.
type RateLimiter struct {
	mu        sync.Mutex
	limit     int
	window    time.Duration
	now       func() time.Time
	hits      map[string]*window
	lastSweep time.Time
}

func NewRateLimiter(limit int, per time.Duration) *RateLimiter {
	return &RateLimiter{limit: limit, window: per, now: time.Now, hits: make(map[string]*window)}
}

// Allow records one request for key. It returns whether the request may proceed, how many
// requests are left in the window and, when blocked, how long until the window resets.
func (r *RateLimiter) Allow(key string) (allowed bool, remaining int, retryAfter time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := r.now()
	r.sweep(now)

	w, ok := r.hits[key]
	if !ok || !now.Before(w.reset) {
		w = &window{reset: now.Add(r.window)}
		r.hits[key] = w
	}
	if w.count >= r.limit {
		return false, 0, w.reset.Sub(now)
	}
	w.count++
	return true, r.limit - w.count, 0
}

// sweep drops expired windows, at most once per window, so the map cannot grow forever.
func (r *RateLimiter) sweep(now time.Time) {
	if now.Sub(r.lastSweep) < r.window && len(r.hits) < maxTrackedClients {
		return
	}
	r.lastSweep = now
	for key, w := range r.hits {
		if !now.Before(w.reset) {
			delete(r.hits, key)
		}
	}
	if len(r.hits) >= maxTrackedClients {
		r.hits = make(map[string]*window)
	}
}

// Middleware answers 429 once a client exceeds the limit. A limit of 0 or less disables it.
func (r *RateLimiter) Middleware() gin.HandlerFunc {
	if r.limit <= 0 {
		return func(c *gin.Context) { c.Next() }
	}
	return func(c *gin.Context) {
		allowed, remaining, retryAfter := r.Allow(c.ClientIP())
		c.Header("X-RateLimit-Limit", strconv.Itoa(r.limit))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(remaining))
		if !allowed {
			seconds := int(math.Ceil(retryAfter.Seconds()))
			if seconds < 1 {
				seconds = 1
			}
			c.Header("Retry-After", strconv.Itoa(seconds))
			response.Fail(c, http.StatusTooManyRequests,
				fmt.Sprintf("too many requests - try again in %d seconds", seconds))
			c.Abort()
			return
		}
		c.Next()
	}
}

// RateLimit returns middleware allowing limit requests per client IP per window.
func RateLimit(limit int, per time.Duration) gin.HandlerFunc {
	return NewRateLimiter(limit, per).Middleware()
}
