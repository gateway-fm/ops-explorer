package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds all Prometheus metrics
type Metrics struct {
	// HTTP metrics
	httpRequestsTotal    *prometheus.CounterVec
	httpRequestDuration  *prometheus.HistogramVec

	// WebSocket metrics
	wsConnectionsActive  prometheus.Gauge

	// Indexer metrics
	indexerBlocksProcessed prometheus.Counter
	indexerLastBlockNumber prometheus.Gauge
	indexerSyncLag         prometheus.Gauge

	// Database metrics
	dbQueryDuration *prometheus.HistogramVec
}

// NewMetrics creates and registers all Prometheus metrics
func NewMetrics() *Metrics {
	return &Metrics{
		httpRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "explorer_http_requests_total",
				Help: "Total number of HTTP requests",
			},
			[]string{"method", "endpoint", "status"},
		),
		httpRequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "explorer_http_request_duration_seconds",
				Help:    "HTTP request duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "endpoint"},
		),
		wsConnectionsActive: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "explorer_websocket_connections_active",
				Help: "Number of active WebSocket connections",
			},
		),
		indexerBlocksProcessed: promauto.NewCounter(
			prometheus.CounterOpts{
				Name: "explorer_indexer_blocks_processed_total",
				Help: "Total number of blocks processed by the indexer",
			},
		),
		indexerLastBlockNumber: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "explorer_indexer_last_block_number",
				Help: "Last block number processed by the indexer",
			},
		),
		indexerSyncLag: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "explorer_indexer_sync_lag_blocks",
				Help: "Number of blocks the indexer is behind the chain head",
			},
		),
		dbQueryDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "explorer_db_query_duration_seconds",
				Help:    "Database query duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"query_type"},
		),
	}
}

// RecordHTTPRequest records an HTTP request
func (m *Metrics) RecordHTTPRequest(method, endpoint string, status int, duration time.Duration) {
	m.httpRequestsTotal.WithLabelValues(method, endpoint, strconv.Itoa(status)).Inc()
	m.httpRequestDuration.WithLabelValues(method, endpoint).Observe(duration.Seconds())
}

// SetWSConnections sets the number of active WebSocket connections
func (m *Metrics) SetWSConnections(count int) {
	m.wsConnectionsActive.Set(float64(count))
}

// RecordBlockProcessed records a block being processed
func (m *Metrics) RecordBlockProcessed(blockNumber uint64) {
	m.indexerBlocksProcessed.Inc()
	m.indexerLastBlockNumber.Set(float64(blockNumber))
}

// SetSyncLag sets the sync lag in blocks
func (m *Metrics) SetSyncLag(lag int64) {
	m.indexerSyncLag.Set(float64(lag))
}

// RecordDBQuery records a database query
func (m *Metrics) RecordDBQuery(queryType string, duration time.Duration) {
	m.dbQueryDuration.WithLabelValues(queryType).Observe(duration.Seconds())
}

// Handler returns the Prometheus HTTP handler
func (m *Metrics) Handler() http.Handler {
	return promhttp.Handler()
}

// metricsMiddleware creates a middleware that records HTTP metrics
func metricsMiddleware(metrics *Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap response writer to capture status code
			ww := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			next.ServeHTTP(ww, r)

			// Get the route pattern if available
			routeCtx := chi.RouteContext(r.Context())
			pattern := routeCtx.RoutePattern()
			if pattern == "" {
				pattern = r.URL.Path
			}

			metrics.RecordHTTPRequest(r.Method, pattern, ww.statusCode, time.Since(start))
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture the status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// etagMiddleware adds ETag support for cacheable GET responses
func etagMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only apply to GET requests
		if r.Method != http.MethodGet {
			next.ServeHTTP(w, r)
			return
		}

		// Create a response recorder to capture the response
		rec := &etagResponseWriter{
			ResponseWriter: w,
			body:           make([]byte, 0),
		}

		next.ServeHTTP(rec, r)

		// If response was already sent (e.g., streaming), skip ETag
		if rec.written {
			return
		}

		// Calculate ETag from response body
		if len(rec.body) > 0 {
			etag := calculateETag(rec.body)
			w.Header().Set("ETag", etag)

			// Check If-None-Match header
			if r.Header.Get("If-None-Match") == etag {
				w.WriteHeader(http.StatusNotModified)
				return
			}
		}

		// Write the captured response
		if rec.statusCode != 0 {
			w.WriteHeader(rec.statusCode)
		}
		w.Write(rec.body)
	})
}

// etagResponseWriter captures the response for ETag calculation
type etagResponseWriter struct {
	http.ResponseWriter
	body       []byte
	statusCode int
	written    bool
}

func (rw *etagResponseWriter) WriteHeader(code int) {
	rw.statusCode = code
}

func (rw *etagResponseWriter) Write(b []byte) (int, error) {
	rw.body = append(rw.body, b...)
	return len(b), nil
}

// calculateETag calculates a simple ETag from response body
func calculateETag(body []byte) string {
	// Use a simple hash for ETag
	var hash uint32
	for _, b := range body {
		hash = hash*31 + uint32(b)
	}
	return `"` + strconv.FormatUint(uint64(hash), 16) + `"`
}
