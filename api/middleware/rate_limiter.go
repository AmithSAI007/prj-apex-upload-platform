package middleware

import (
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/AmithSAI007/prj-apex-upload-platform/api/dto"
	"github.com/AmithSAI007/prj-apex-upload-platform/pkg/constants"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

// GlobalRateLimiter returns a Gin middleware that enforces a server-wide
// requests-per-second limit using a token-bucket algorithm. Requests that
// exceed the limit receive HTTP 429 with a Retry-After header.
func GlobalRateLimiter(rps float64, burst int) gin.HandlerFunc {
	limiter := rate.NewLimiter(rate.Limit(rps), burst)
	return func(c *gin.Context) {
		if !limiter.Allow() {
			reservation := limiter.Reserve()
			delay := reservation.Delay()
			reservation.Cancel()

			retryAfter := int(delay.Seconds()) + 1
			c.Header("Retry-After", strconv.Itoa(retryAfter))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, dto.ErrorResponse{
				Error: dto.ErrorPayload{
					Code:      dto.ErrorCodeRateLimited,
					Message:   "Server rate limit exceeded, please retry later",
					RequestID: c.GetString(TraceIDKey),
				},
			})
			return
		}
		c.Next()
	}
}

// clientEntry pairs a rate limiter with its last access time for TTL eviction.
type clientEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// PerClientRateLimiter enforces per-user request rate limits. The limiter key
// is the authenticated user_id from the JWT (set by AuthMiddleware); for
// unauthenticated requests it falls back to the client IP address.
//
// To prevent unbounded memory growth on horizontally scaled Cloud Run instances,
// the number of individually tracked clients is capped at maxClients. When the
// cap is reached, new unknown clients share a single stricter overflow limiter
// rather than being rejected outright. Existing tracked clients are unaffected.
type PerClientRateLimiter struct {
	clients    sync.Map // map[string]*clientEntry
	rps        float64
	burst      int
	ttl        time.Duration
	logger     *zap.Logger
	done       chan struct{}
	maxClients int64
	count      atomic.Int64
	overflow   *rate.Limiter
}

// NewPerClientRateLimiter creates a per-client rate limiter that allows rps
// sustained requests per second with the given burst size per unique client.
// Idle client entries are evicted after ttl. At most maxClients unique clients
// are tracked individually; additional clients share a stricter overflow limiter
// at half the per-client rate and burst. The caller must call Stop() to release
// the background cleanup goroutine.
func NewPerClientRateLimiter(logger *zap.Logger, rps float64, burst int, ttl time.Duration, maxClients int64) *PerClientRateLimiter {
	// Overflow limiter uses half the per-client rate/burst — stricter but not
	// a hard rejection. All overflow clients share this single bucket.
	overflowRPS := rps / 2
	if overflowRPS < 1 {
		overflowRPS = 1
	}
	overflowBurst := burst / 2
	if overflowBurst < 1 {
		overflowBurst = 1
	}

	rl := &PerClientRateLimiter{
		rps:        rps,
		burst:      burst,
		ttl:        ttl,
		logger:     logger,
		done:       make(chan struct{}),
		maxClients: maxClients,
		overflow:   rate.NewLimiter(rate.Limit(overflowRPS), overflowBurst),
	}
	go rl.cleanup(5 * time.Minute)
	return rl
}

// Stop halts the background cleanup goroutine. It is safe to call multiple
// times but only the first call has any effect.
func (rl *PerClientRateLimiter) Stop() {
	select {
	case <-rl.done:
		// Already closed.
	default:
		close(rl.done)
	}
}

// Middleware returns a Gin handler that enforces per-client rate limits.
func (rl *PerClientRateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := rl.clientKey(c)
		limiter := rl.getOrCreate(key)

		if !limiter.Allow() {
			reservation := limiter.Reserve()
			delay := reservation.Delay()
			reservation.Cancel()

			rl.logger.Warn("Per-client rate limit exceeded",
				zap.String("client_key", key),
				zap.String("trace_id", c.GetString(TraceIDKey)),
			)

			retryAfter := int(delay.Seconds()) + 1
			c.Header("Retry-After", strconv.Itoa(retryAfter))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, dto.ErrorResponse{
				Error: dto.ErrorPayload{
					Code:      dto.ErrorCodeRateLimited,
					Message:   "Rate limit exceeded for this client, please retry later",
					RequestID: c.GetString(TraceIDKey),
				},
			})
			return
		}
		c.Next()
	}
}

// clientKey returns the rate-limit key: user_id from JWT claims if present,
// otherwise the client IP address.
func (rl *PerClientRateLimiter) clientKey(c *gin.Context) string {
	if userID, ok := c.Request.Context().Value(constants.CtxUserIDKey).(string); ok && userID != "" {
		return "user:" + userID
	}
	return "ip:" + c.ClientIP()
}

// getOrCreate returns an existing client entry or creates a new one atomically.
// When the number of tracked clients has reached maxClients, new unknown clients
// receive the shared overflow limiter instead of a dedicated bucket.
func (rl *PerClientRateLimiter) getOrCreate(key string) *rate.Limiter {
	now := time.Now()

	// Fast path: client already tracked.
	if val, ok := rl.clients.Load(key); ok {
		entry := val.(*clientEntry)
		entry.lastSeen = now
		return entry.limiter
	}

	// Check if we've hit the cap before allocating a new entry.
	if rl.count.Load() >= rl.maxClients {
		rl.logger.Warn("Per-client rate limiter at capacity, using overflow limiter",
			zap.String("client_key", key),
			zap.Int64("max_clients", rl.maxClients),
		)
		return rl.overflow
	}

	entry := &clientEntry{
		limiter:  rate.NewLimiter(rate.Limit(rl.rps), rl.burst),
		lastSeen: now,
	}
	actual, loaded := rl.clients.LoadOrStore(key, entry)
	if !loaded {
		// We inserted a new entry — increment the counter.
		rl.count.Add(1)
	}
	return actual.(*clientEntry).limiter
}

// cleanup periodically evicts client entries that haven't been seen within the TTL.
// The atomic counter is decremented for each evicted entry so that previously
// overflow-routed clients can be promoted to individual buckets.
func (rl *PerClientRateLimiter) cleanup(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-rl.done:
			return
		case <-ticker.C:
			now := time.Now()
			rl.clients.Range(func(key, value any) bool {
				entry := value.(*clientEntry)
				if now.Sub(entry.lastSeen) > rl.ttl {
					rl.clients.Delete(key)
					rl.count.Add(-1)
					rl.logger.Debug("Evicted stale rate limiter entry",
						zap.String("key", key.(string)),
					)
				}
				return true
			})
		}
	}
}
