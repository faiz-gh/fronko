package ratelimit

import (
	"context"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/faiz-gh/fronko/backend/internal/platform/httpx"
)

// visitorIdleTTL is how long an IP's bucket is kept after its last request.
const visitorIdleTTL = 10 * time.Minute

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimiter is an in-memory, per-client-IP token bucket. State lives in this
// process only, so each backend instance enforces its own budget.
type RateLimiter struct {
	limit      rate.Limit
	burst      int
	trustProxy bool

	mu       sync.Mutex
	visitors map[string]*visitor
}

// NewRateLimiter allows burst requests at once, refilling at limit per second.
// It evicts idle clients in the background until ctx is cancelled.
func NewRateLimiter(ctx context.Context, limit rate.Limit, burst int, trustProxy bool) *RateLimiter {
	rl := &RateLimiter{
		limit:      limit,
		burst:      burst,
		trustProxy: trustProxy,
		visitors:   make(map[string]*visitor),
	}
	go rl.evictIdle(ctx)
	return rl
}

func (rl *RateLimiter) evictIdle(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			rl.mu.Lock()
			for ip, v := range rl.visitors {
				if now.Sub(v.lastSeen) > visitorIdleTTL {
					delete(rl.visitors, ip)
				}
			}
			rl.mu.Unlock()
		}
	}
}

// reserve reports whether the request may proceed and, if not, how long until it could.
func (rl *RateLimiter) reserve(ip string) (bool, time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	v, ok := rl.visitors[ip]
	if !ok {
		v = &visitor{limiter: rate.NewLimiter(rl.limit, rl.burst)}
		rl.visitors[ip] = v
	}
	v.lastSeen = time.Now()

	res := v.limiter.Reserve()
	if delay := res.Delay(); delay > 0 {
		res.Cancel() // don't consume a token for a rejected request
		return false, delay
	}
	return true, 0
}

// Limit wraps a handler, answering 429 with Retry-After once a client's budget is spent.
func (rl *RateLimiter) Limit(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ok, wait := rl.reserve(ClientIP(r, rl.trustProxy))
		if !ok {
			w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(wait.Seconds()))))
			httpx.WriteError(w, http.StatusTooManyRequests, "too many requests, please try again shortly")
			return
		}
		next(w, r)
	}
}

// ClientIP returns the caller's IP. X-Real-IP is only honoured when the backend
// sits behind a proxy that sets it (trustProxy); otherwise anyone could spoof it.
func ClientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if ip := strings.TrimSpace(r.Header.Get("X-Real-IP")); ip != "" {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
