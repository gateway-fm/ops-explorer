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

type Metrics struct {
	httpRequestsTotal    *prometheus.CounterVec
	httpRequestDuration  *prometheus.HistogramVec

	wsConnectionsActive  prometheus.Gauge

	indexerBlocksProcessed prometheus.Counter
	indexerLastBlockNumber prometheus.Gauge
	indexerSyncLag         prometheus.Gauge

	dbQueryDuration *prometheus.HistogramVec
}

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

func (m *Metrics) RecordHTTPRequest(method, endpoint string, status int, duration time.Duration) {
	m.httpRequestsTotal.WithLabelValues(method, endpoint, strconv.Itoa(status)).Inc()
	m.httpRequestDuration.WithLabelValues(method, endpoint).Observe(duration.Seconds())
}

func (m *Metrics) SetWSConnections(count int) {
	m.wsConnectionsActive.Set(float64(count))
}

func (m *Metrics) RecordBlockProcessed(blockNumber uint64) {
	m.indexerBlocksProcessed.Inc()
	m.indexerLastBlockNumber.Set(float64(blockNumber))
}

func (m *Metrics) SetSyncLag(lag int64) {
	m.indexerSyncLag.Set(float64(lag))
}

func (m *Metrics) RecordDBQuery(queryType string, duration time.Duration) {
	m.dbQueryDuration.WithLabelValues(queryType).Observe(duration.Seconds())
}

func (m *Metrics) Handler() http.Handler {
	return promhttp.Handler()
}

func metricsMiddleware(metrics *Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			ww := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			next.ServeHTTP(ww, r)

			routeCtx := chi.RouteContext(r.Context())
			pattern := routeCtx.RoutePattern()
			if pattern == "" {
				pattern = r.URL.Path
			}

			metrics.RecordHTTPRequest(r.Method, pattern, ww.statusCode, time.Since(start))
		})
	}
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func etagMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			next.ServeHTTP(w, r)
			return
		}

		rec := &etagResponseWriter{
			ResponseWriter: w,
			body:           make([]byte, 0),
		}

		next.ServeHTTP(rec, r)

		if rec.written {
			return
		}

		if len(rec.body) > 0 {
			etag := calculateETag(rec.body)
			w.Header().Set("ETag", etag)

			if r.Header.Get("If-None-Match") == etag {
				w.WriteHeader(http.StatusNotModified)
				return
			}
		}

		if rec.statusCode != 0 {
			w.WriteHeader(rec.statusCode)
		}
		w.Write(rec.body)
	})
}

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

func calculateETag(body []byte) string {
	var hash uint32
	for _, b := range body {
		hash = hash*31 + uint32(b)
	}
	return `"` + strconv.FormatUint(uint64(hash), 16) + `"`
}
