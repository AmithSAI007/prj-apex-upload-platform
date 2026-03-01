package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Prometheus metric collectors for HTTP request monitoring. These are
// auto-registered with the default Prometheus registry via promauto.
var (
	// httpRequestsTotal counts the total number of HTTP requests partitioned by
	// route path, HTTP method, and response status code.
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"path", "method", "status"},
	)

	// httpRequestDurationSeconds observes the latency of HTTP requests in seconds,
	// partitioned by route path and HTTP method. Uses the default histogram buckets.
	httpRequestDurationSeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duration of HTTP requests in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"path", "method"},
	)
)

// PrometheusMetrics returns a Gin middleware that records request count and
// latency metrics for every HTTP request. Metrics are exposed at the /metrics
// endpoint via the Prometheus HTTP handler.
func PrometheusMetrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		duration := time.Since(start).Seconds()
		statusCode := strconv.Itoa(c.Writer.Status())
		httpRequestsTotal.WithLabelValues(c.FullPath(), c.Request.Method, statusCode).Inc()
		httpRequestDurationSeconds.WithLabelValues(c.FullPath(), c.Request.Method).Observe(duration)
	}

}
