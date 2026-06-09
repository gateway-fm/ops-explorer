package log

import (
	"bufio"
	"errors"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// RedactHTTPPaths controls whether HTTPMiddleware masks on-chain identifiers in
// the logged request path. It must be enabled in privacy mode (set from
// cmd/api/main.go: log.RedactHTTPPaths = cfg.PrivacyProxyURL != "") so access
// logs cannot correlate an authenticated DID with the addresses/grants it
// viewed (PROD_READINESS_AUDIT §P-3). Standalone keeps it false: that data is
// public and full paths aid debugging.
var RedactHTTPPaths bool

// hexIdentifier matches a 0x-prefixed hex run long enough to be an on-chain
// identifier (address = 40 hex chars, tx hash = 64). The {6,} floor avoids
// masking short hex like "0x0" while still catching any real address/hash.
var hexIdentifier = regexp.MustCompile(`(?i)0x[0-9a-f]{6,}`)

const redactedSegment = "[redacted]"

// redactPath masks privacy-sensitive identifiers in a request path so that
// privacy-mode access logs cannot correlate an authenticated DID with the real
// addresses / grants it viewed (PROD_READINESS_AUDIT §P-3). It masks:
//
//   - any 0x-hex run of >= 6 hex digits (covers both addresses and tx hashes),
//     wherever it appears in the path; and
//   - the two opaque segments immediately following a "/grant/" segment,
//     regardless of their textual format (grant/address IDs are UUIDs today but
//     are equally sensitive in any format).
//
// Query strings are not handled here because HTTPMiddleware logs r.URL.Path
// (path only), so no query parameters reach the log line.
func redactPath(path string) string {
	// Mask the two segments after /grant/ first (positional redaction), then
	// run the hex-identifier mask over what remains.
	segs := strings.Split(path, "/")
	for i := 0; i < len(segs); i++ {
		if segs[i] != "grant" {
			continue
		}
		// Redact the next up-to-two non-empty segments (grantId, addressId).
		// We redact whatever occupies those positions even if a trailing
		// sub-resource like "activity" follows, so single-id grant routes
		// still mask both positions deterministically.
		for n, j := 0, i+1; j < len(segs) && n < 2; j++ {
			if segs[j] == "" {
				continue
			}
			segs[j] = redactedSegment
			n++
		}
	}
	out := strings.Join(segs, "/")
	return hexIdentifier.ReplaceAllString(out, redactedSegment)
}

// HTTPMiddleware logs each request after it completes using this package's
// structured format. Drop-in replacement for chi's middleware.Logger — the
// difference is that the line goes through Info() with key/value pairs
// instead of being formatted by the stdlib logger.
func HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
		start := time.Now()
		next.ServeHTTP(rec, r)
		path := r.URL.Path
		if RedactHTTPPaths {
			path = redactPath(path)
		}
		Info("http request",
			"method", r.Method,
			"path", path,
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
