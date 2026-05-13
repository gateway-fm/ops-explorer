package log

import (
	"bufio"
	"errors"
	"net"
	"net/http"
	"time"
)

// HTTPMiddleware logs each request after it completes using this package's
// structured format. Drop-in replacement for chi's middleware.Logger — the
// difference is that the line goes through Info() with key/value pairs
// instead of being formatted by the stdlib logger.
func HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
		start := time.Now()
		next.ServeHTTP(rec, r)
		Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"bytes", rec.bytes,
			"duration", time.Since(start),
			"remote", r.RemoteAddr,
		)
	})
}

// responseRecorder is a thin http.ResponseWriter wrapper that captures the
// status code and number of bytes written. Hijacker and Flusher are forwarded
// so websocket upgrades and SSE streams keep working through the middleware.
type responseRecorder struct {
	http.ResponseWriter
	status      int
	bytes       int
	wroteHeader bool
}

func (r *responseRecorder) WriteHeader(code int) {
	if r.wroteHeader {
		return
	}
	r.status = code
	r.wroteHeader = true
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	r.wroteHeader = true
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err
}

func (r *responseRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("log: underlying ResponseWriter does not implement http.Hijacker")
	}
	return h.Hijack()
}

func (r *responseRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
