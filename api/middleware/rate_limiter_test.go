package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/AmithSAI007/prj-apex-upload-platform/api/dto"
	"github.com/AmithSAI007/prj-apex-upload-platform/pkg/constants"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// ---------- GlobalRateLimiter ----------

func TestGlobalRateLimiter_AllowsRequestsUnderLimit(t *testing.T) {
	r := gin.New()
	r.Use(GlobalRateLimiter(10, 10))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		resp := httptest.NewRecorder()
		r.ServeHTTP(resp, req)
		if resp.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i, resp.Code)
		}
	}
}

func TestGlobalRateLimiter_RejectsOverLimit(t *testing.T) {
	// burst=2 means only 2 tokens available immediately.
	r := gin.New()
	r.Use(RequestContext()) // needed for trace_id in error response
	r.Use(GlobalRateLimiter(1, 2))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	// Exhaust the burst.
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		resp := httptest.NewRecorder()
		r.ServeHTTP(resp, req)
		if resp.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i, resp.Code)
		}
	}

	// Next request should be rate limited.
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", resp.Code)
	}

	retryAfter := resp.Header().Get("Retry-After")
	if retryAfter == "" {
		t.Fatal("expected Retry-After header to be set")
	}

	var errResp dto.ErrorResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to unmarshal error response: %v", err)
	}
	if errResp.Error.Code != dto.ErrorCodeRateLimited {
		t.Fatalf("expected error code %q, got %q", dto.ErrorCodeRateLimited, errResp.Error.Code)
	}
}

func TestGlobalRateLimiter_DoesNotCallNextOnReject(t *testing.T) {
	called := false
	r := gin.New()
	r.Use(GlobalRateLimiter(1, 1))
	r.GET("/test", func(c *gin.Context) {
		called = true
		c.String(http.StatusOK, "ok")
	})

	// First request consumes the single token.
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	called = false

	// Second request should be rejected without calling the handler.
	req = httptest.NewRequest(http.MethodGet, "/test", nil)
	resp = httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", resp.Code)
	}
	if called {
		t.Fatal("handler should not have been called on rate-limited request")
	}
}

// ---------- PerClientRateLimiter ----------

func newTestPerClientLimiter(rps float64, burst int) *PerClientRateLimiter {
	logger, _ := zap.NewDevelopment()
	return NewPerClientRateLimiter(logger, rps, burst, 10*time.Minute, 1000)
}

func TestPerClientRateLimiter_AllowsRequestsUnderLimit(t *testing.T) {
	rl := newTestPerClientLimiter(10, 10)
	defer rl.Stop()

	r := gin.New()
	r.Use(rl.Middleware())
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		resp := httptest.NewRecorder()
		r.ServeHTTP(resp, req)
		if resp.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i, resp.Code)
		}
	}
}

func TestPerClientRateLimiter_RejectsOverLimit(t *testing.T) {
	rl := newTestPerClientLimiter(1, 2)
	defer rl.Stop()

	r := gin.New()
	r.Use(RequestContext())
	r.Use(rl.Middleware())
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	// Exhaust the burst.
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		resp := httptest.NewRecorder()
		r.ServeHTTP(resp, req)
		if resp.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i, resp.Code)
		}
	}

	// Third request should be rate limited.
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", resp.Code)
	}

	retryAfter := resp.Header().Get("Retry-After")
	if retryAfter == "" {
		t.Fatal("expected Retry-After header to be set")
	}

	var errResp dto.ErrorResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to unmarshal error response: %v", err)
	}
	if errResp.Error.Code != dto.ErrorCodeRateLimited {
		t.Fatalf("expected error code %q, got %q", dto.ErrorCodeRateLimited, errResp.Error.Code)
	}
}

func TestPerClientRateLimiter_IndependentLimitsPerUser(t *testing.T) {
	rl := newTestPerClientLimiter(1, 1)
	defer rl.Stop()

	r := gin.New()
	r.Use(rl.Middleware())
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	// User A exhausts their token.
	reqA := httptest.NewRequest(http.MethodGet, "/test", nil)
	reqA = reqA.WithContext(context.WithValue(reqA.Context(), constants.CtxUserIDKey, "user_a"))
	respA := httptest.NewRecorder()
	r.ServeHTTP(respA, reqA)
	if respA.Code != http.StatusOK {
		t.Fatalf("user_a first request: expected 200, got %d", respA.Code)
	}

	// User A second request should be rate limited.
	reqA2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	reqA2 = reqA2.WithContext(context.WithValue(reqA2.Context(), constants.CtxUserIDKey, "user_a"))
	respA2 := httptest.NewRecorder()
	r.ServeHTTP(respA2, reqA2)
	if respA2.Code != http.StatusTooManyRequests {
		t.Fatalf("user_a second request: expected 429, got %d", respA2.Code)
	}

	// User B should still have their own independent limit.
	reqB := httptest.NewRequest(http.MethodGet, "/test", nil)
	reqB = reqB.WithContext(context.WithValue(reqB.Context(), constants.CtxUserIDKey, "user_b"))
	respB := httptest.NewRecorder()
	r.ServeHTTP(respB, reqB)
	if respB.Code != http.StatusOK {
		t.Fatalf("user_b first request: expected 200, got %d", respB.Code)
	}
}

func TestPerClientRateLimiter_FallsBackToIPWithoutUserID(t *testing.T) {
	rl := newTestPerClientLimiter(1, 1)
	defer rl.Stop()

	r := gin.New()
	r.Use(rl.Middleware())
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	// First request without user_id — uses IP.
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("first request: expected 200, got %d", resp.Code)
	}

	// Second from same IP should be rate limited.
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.RemoteAddr = "10.0.0.1:12346"
	resp2 := httptest.NewRecorder()
	r.ServeHTTP(resp2, req2)
	if resp2.Code != http.StatusTooManyRequests {
		t.Fatalf("second request: expected 429, got %d", resp2.Code)
	}

	// Different IP should pass.
	req3 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req3.RemoteAddr = "10.0.0.2:12345"
	resp3 := httptest.NewRecorder()
	r.ServeHTTP(resp3, req3)
	if resp3.Code != http.StatusOK {
		t.Fatalf("different IP request: expected 200, got %d", resp3.Code)
	}
}

func TestPerClientRateLimiter_StopIsIdempotent(t *testing.T) {
	rl := newTestPerClientLimiter(10, 10)

	// Calling Stop multiple times should not panic.
	rl.Stop()
	rl.Stop()
}

func TestPerClientRateLimiter_CleanupEvictsStaleEntries(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	rl := &PerClientRateLimiter{
		rps:        10,
		burst:      10,
		ttl:        200 * time.Millisecond, // fresh entries survive the sleep
		logger:     logger,
		done:       make(chan struct{}),
		maxClients: 1000,
		overflow:   rate.NewLimiter(5, 5),
	}

	// Manually insert entries.
	rl.clients.Store("user:stale", &clientEntry{
		limiter:  nil,
		lastSeen: time.Now().Add(-500 * time.Millisecond), // well past TTL
	})
	rl.clients.Store("user:fresh", &clientEntry{
		limiter:  nil,
		lastSeen: time.Now(),
	})
	rl.count.Store(2)

	// Run cleanup once via a short-interval goroutine.
	go rl.cleanup(10 * time.Millisecond)

	// Wait for at least one cleanup cycle.
	time.Sleep(50 * time.Millisecond)
	close(rl.done)

	if _, ok := rl.clients.Load("user:stale"); ok {
		t.Fatal("expected stale entry to be evicted")
	}
	if _, ok := rl.clients.Load("user:fresh"); !ok {
		t.Fatal("expected fresh entry to be retained")
	}
}

// ---------- MaxClients cap & overflow ----------

func TestPerClientRateLimiter_OverflowWhenCapReached(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	// Cap at 2 clients; overflow limiter with generous burst so requests still pass.
	rl := NewPerClientRateLimiter(logger, 10, 10, 10*time.Minute, 2)
	defer rl.Stop()

	r := gin.New()
	r.Use(rl.Middleware())
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	// Fill the cap with 2 distinct users.
	for _, uid := range []string{"user_a", "user_b"} {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req = req.WithContext(context.WithValue(req.Context(), constants.CtxUserIDKey, uid))
		resp := httptest.NewRecorder()
		r.ServeHTTP(resp, req)
		if resp.Code != http.StatusOK {
			t.Fatalf("user %s: expected 200, got %d", uid, resp.Code)
		}
	}

	if rl.count.Load() != 2 {
		t.Fatalf("expected count=2, got %d", rl.count.Load())
	}

	// Third user should use the overflow limiter but still get a response
	// (overflow limiter has tokens available).
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req = req.WithContext(context.WithValue(req.Context(), constants.CtxUserIDKey, "user_c"))
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("overflow user: expected 200 (overflow limiter has tokens), got %d", resp.Code)
	}

	// Count should still be 2 — overflow users are not tracked.
	if rl.count.Load() != 2 {
		t.Fatalf("expected count=2 after overflow, got %d", rl.count.Load())
	}
}

func TestPerClientRateLimiter_OverflowLimiterIsShared(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	// Cap at 1 client; overflow burst=2 so two overflow requests pass then third is rejected.
	rl := NewPerClientRateLimiter(logger, 1, 4, 10*time.Minute, 1)
	defer rl.Stop()

	r := gin.New()
	r.Use(RequestContext())
	r.Use(rl.Middleware())
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	// Fill the cap.
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req = req.WithContext(context.WithValue(req.Context(), constants.CtxUserIDKey, "tracked"))
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("tracked user: expected 200, got %d", resp.Code)
	}

	// Overflow limiter has burst = max(4/2, 1) = 2. Send requests from different
	// overflow users to exhaust the shared overflow bucket.
	overflowBurst := 2
	for i := 0; i < overflowBurst; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req = req.WithContext(context.WithValue(req.Context(), constants.CtxUserIDKey, "overflow_"+strconv.Itoa(i)))
		resp := httptest.NewRecorder()
		r.ServeHTTP(resp, req)
		if resp.Code != http.StatusOK {
			t.Fatalf("overflow request %d: expected 200, got %d", i, resp.Code)
		}
	}

	// Next overflow request should be rate limited — shared bucket is exhausted.
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2 = req2.WithContext(context.WithValue(req2.Context(), constants.CtxUserIDKey, "overflow_extra"))
	resp2 := httptest.NewRecorder()
	r.ServeHTTP(resp2, req2)
	if resp2.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 when overflow bucket exhausted, got %d", resp2.Code)
	}
}

func TestPerClientRateLimiter_ExistingClientUnaffectedByOverflow(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	// Cap at 1 client; overflow burst=1.
	rl := NewPerClientRateLimiter(logger, 1, 2, 10*time.Minute, 1)
	defer rl.Stop()

	r := gin.New()
	r.Use(rl.Middleware())
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	// Tracked user makes first request.
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req = req.WithContext(context.WithValue(req.Context(), constants.CtxUserIDKey, "tracked"))
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("tracked user first: expected 200, got %d", resp.Code)
	}

	// Overflow users exhaust the overflow bucket.
	// overflow burst = max(2/2, 1) = 1
	reqOF := httptest.NewRequest(http.MethodGet, "/test", nil)
	reqOF = reqOF.WithContext(context.WithValue(reqOF.Context(), constants.CtxUserIDKey, "untracked"))
	respOF := httptest.NewRecorder()
	r.ServeHTTP(respOF, reqOF)
	// Don't care if this passes or fails — the point is to drain overflow tokens.

	// Tracked user should still use their own limiter (burst=2, used 1).
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2 = req2.WithContext(context.WithValue(req2.Context(), constants.CtxUserIDKey, "tracked"))
	resp2 := httptest.NewRecorder()
	r.ServeHTTP(resp2, req2)
	if resp2.Code != http.StatusOK {
		t.Fatalf("tracked user second request: expected 200 (own bucket), got %d", resp2.Code)
	}
}

func TestPerClientRateLimiter_CleanupDecrementsCounter(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	rl := &PerClientRateLimiter{
		rps:        10,
		burst:      10,
		ttl:        200 * time.Millisecond,
		logger:     logger,
		done:       make(chan struct{}),
		maxClients: 100,
		overflow:   rate.NewLimiter(5, 5),
	}

	// Insert 3 entries: 2 stale (well past TTL), 1 fresh (just created).
	rl.clients.Store("user:stale1", &clientEntry{
		limiter:  rate.NewLimiter(10, 10),
		lastSeen: time.Now().Add(-500 * time.Millisecond),
	})
	rl.clients.Store("user:stale2", &clientEntry{
		limiter:  rate.NewLimiter(10, 10),
		lastSeen: time.Now().Add(-500 * time.Millisecond),
	})
	rl.clients.Store("user:fresh", &clientEntry{
		limiter:  rate.NewLimiter(10, 10),
		lastSeen: time.Now(),
	})
	rl.count.Store(3)

	go rl.cleanup(10 * time.Millisecond)
	time.Sleep(50 * time.Millisecond)
	close(rl.done)

	// 2 stale entries evicted: count should drop from 3 to 1.
	if got := rl.count.Load(); got != 1 {
		t.Fatalf("expected count=1 after eviction, got %d", got)
	}
}
