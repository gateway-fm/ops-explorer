package authv2

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

func mergeVary(h http.Header, names ...string) {
	seen := map[string]bool{}
	var out []string
	for _, v := range append(h.Values("Vary"), strings.Join(names, ",")) {
		for _, s := range strings.Split(v, ",") {
			s = strings.TrimSpace(s)
			if s != "" && !seen[strings.ToLower(s)] {
				seen[strings.ToLower(s)] = true
				out = append(out, s)
			}
		}
	}
	h.Set("Vary", strings.Join(out, ", "))
}
func cachePolicy(h http.Header) {
	for _, k := range []string{"ETag", "Last-Modified", "Age", "Surrogate-Control"} {
		h.Del(k)
	}
	h.Set("Cache-Control", "no-store")
	h.Set("Pragma", "no-cache")
	h.Set("Expires", "0")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "no-referrer")
	mergeVary(h, "Origin", "Cookie", SessionHeader)
}

type policyWriter struct {
	http.ResponseWriter
	origin string
	sent   bool
}

func (w *policyWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w *policyWriter) policy() {
	h := w.Header()
	cachePolicy(h)
	for _, key := range []string{"Access-Control-Allow-Origin", "Access-Control-Allow-Credentials", "Access-Control-Max-Age"} {
		h.Del(key)
	}
	if w.origin != "" {
		h.Set("Access-Control-Allow-Origin", w.origin)
		h.Set("Access-Control-Allow-Credentials", "true")
	}
}
func (w *policyWriter) WriteHeader(status int) {
	if w.sent {
		return
	}
	if status < 200 || status > 599 {
		status = 500
	}
	w.policy()
	w.sent = true
	w.ResponseWriter.WriteHeader(status)
}
func (w *policyWriter) Write(b []byte) (int, error) {
	if !w.sent {
		w.WriteHeader(200)
	}
	return w.ResponseWriter.Write(b)
}
func (w *policyWriter) FlushError() error {
	if !w.sent {
		w.WriteHeader(200)
	}
	return http.NewResponseController(w.ResponseWriter).Flush()
}
func (w *policyWriter) Flush() { _ = w.FlushError() }

// Boundary is the selected V2 ingress. methodAllowed must inspect the actual
// mounted route table, not an arbitrary path prefix, for credentialed preflight.
func (s *Service) Boundary(next http.Handler, methodAllowed func(string, string) bool) http.Handler {
	return http.HandlerFunc(func(out http.ResponseWriter, r *http.Request) {
		w := &policyWriter{ResponseWriter: out}
		cachePolicy(w.Header())
		if !s.Ready() {
			fail(w, ErrUnavailable)
			return
		}
		origins := r.Header.Values("Origin")
		if r.Host != s.source.host || len(origins) > 1 || len(r.URL.RawQuery) > 8192 || len(r.URL.Path) > 2048 || strings.ContainsAny(r.URL.Path, "\\\x00\r\n") || r.URL.RawPath != "" {
			fail(w, ErrInvalid)
			return
		}
		if len(origins) == 1 {
			if origins[0] != s.source.spec.AppOrigin {
				safeJSON(w, 403, map[string]string{"error": "origin_not_allowed"})
				return
			}
			w.origin = s.source.spec.AppOrigin
		}
		if r.Method == "OPTIONS" {
			mergeVary(w.Header(), "Access-Control-Request-Method", "Access-Control-Request-Headers")
			methods := r.Header.Values("Access-Control-Request-Method")
			headers := r.Header.Values("Access-Control-Request-Headers")
			if w.origin == "" || len(methods) != 1 || len(methods[0]) > 16 || len(headers) > 1 || len(headers) == 1 && len(headers[0]) > 512 {
				fail(w, ErrInvalid)
				return
			}
			if methodAllowed == nil || !methodAllowed(r.URL.Path, methods[0]) {
				w.origin = ""
				safeJSON(w, 403, map[string]string{"error": "method_not_allowed"})
				return
			}
			var accepted []string
			seen := map[string]bool{}
			if len(headers) == 1 {
				for _, h := range strings.Split(headers[0], ",") {
					h = strings.ToLower(strings.TrimSpace(h))
					if h == "" || seen[h] || h != "accept" && h != "content-type" && h != "x-ops-app-session" {
						w.origin = ""
						safeJSON(w, 403, map[string]string{"error": "headers_not_allowed"})
						return
					}
					if h == "x-ops-app-session" && (r.URL.Path == "/api/auth/callback" || strings.HasSuffix(r.URL.Path, "/chain-info")) {
						w.origin = ""
						safeJSON(w, 403, map[string]string{"error": "headers_not_allowed"})
						return
					}
					seen[h] = true
					accepted = append(accepted, h)
				}
			}
			w.Header().Set("Access-Control-Allow-Methods", methods[0])
			if len(accepted) > 0 {
				w.Header().Set("Access-Control-Allow-Headers", strings.Join(accepted, ", "))
			}
			w.WriteHeader(204)
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" && w.origin == "" {
			safeJSON(w, 403, map[string]string{"error": "origin_required"})
			return
		}
		if len(r.Header.Values("Authorization")) != 0 || len(r.Header.Values("Upgrade")) != 0 {
			fail(w, ErrInvalid)
			return
		}
		select {
		case s.slots <- struct{}{}:
			defer func() { <-s.slots }()
		default:
			fail(w, ErrUnavailable)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		deadline, _ := ctx.Deadline()
		rc := http.NewResponseController(w)
		if e := rc.SetReadDeadline(deadline); e != nil && !errors.Is(e, http.ErrNotSupported) {
			fail(w, ErrUnavailable)
			return
		}
		if e := rc.SetWriteDeadline(deadline); e != nil && !errors.Is(e, http.ErrNotSupported) {
			fail(w, ErrUnavailable)
			return
		}
		defer func() {
			if recover() != nil {
				if w.sent {
					panic(http.ErrAbortHandler)
				}
				for key := range w.Header() {
					w.Header().Del(key)
				}
				safeJSON(w, 500, map[string]string{"error": "internal_error"})
			}
		}()
		next.ServeHTTP(w, r.WithContext(ctx))
		if !w.sent {
			w.WriteHeader(200)
		}
	})
}

type privateWriter struct {
	out          http.ResponseWriter
	r            *http.Request
	service      *Service
	session      *requestSession
	header       http.Header
	status       int
	sent, failed bool
	written      int64
	deadline     time.Time
}

func newPrivateWriter(out http.ResponseWriter, r *http.Request, s *Service, session *requestSession) *privateWriter {
	deadline := time.Now().Add(30 * time.Second)
	if d, ok := r.Context().Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	remaining := session.capture.expires - s.clock()
	d := time.Now().Add(time.Duration(remaining) * time.Second)
	if d.Before(deadline) {
		deadline = d
	}
	return &privateWriter{out: out, r: r, service: s, session: session, header: make(http.Header), deadline: deadline}
}
func (w *privateWriter) Header() http.Header { return w.header }
func (w *privateWriter) WriteHeader(status int) {
	if w.sent || w.failed || w.status != 0 {
		return
	}
	if status < 200 || status > 599 {
		w.failed = true
		fail(w.out, ErrInvalid)
		return
	}
	w.status = status
}
func validHeaders(h http.Header) bool {
	if len(h) > 64 {
		return false
	}
	n := 0
	for key, values := range h {
		if key == "" || strings.ContainsAny(key, "\x00\r\n:") {
			return false
		}
		for _, v := range values {
			n += len(key) + len(v)
			if strings.ContainsAny(v, "\x00\r\n") {
				return false
			}
		}
	}
	return n <= 32<<10
}
func (w *privateWriter) release() error {
	if w.failed {
		return ErrUnavailable
	}
	if w.sent {
		return nil
	}
	e := w.service.recheck(w.r.Context(), w.session)
	if e == nil && !validHeaders(w.header) {
		e = ErrUnavailable
	}
	if e == nil {
		e = w.SetWriteDeadline(w.deadline)
		if errors.Is(e, http.ErrNotSupported) {
			e = nil
		}
	}
	if e != nil {
		w.failed = true
		fail(w.out, e)
		return e
	}
	for key, values := range w.header {
		w.out.Header()[key] = append([]string(nil), values...)
	}
	cachePolicy(w.out.Header())
	status := w.status
	if status == 0 {
		status = 200
	}
	w.sent = true
	w.out.WriteHeader(status)
	return nil
}
func (w *privateWriter) Write(b []byte) (int, error) {
	if w.written+int64(len(b)) > 16<<20 {
		if !w.sent {
			w.failed = true
			fail(w.out, ErrUnavailable)
		}
		return 0, ErrUnavailable
	}
	if e := w.release(); e != nil {
		return 0, e
	}
	if w.r.Context().Err() != nil || w.service.clock() >= w.session.capture.expires {
		return 0, ErrUnauthenticated
	}
	n, e := w.out.Write(b)
	w.written += int64(n)
	return n, e
}
func (w *privateWriter) WriteString(s string) (int, error) { return w.Write([]byte(s)) }
func (w *privateWriter) ReadFrom(reader io.Reader) (int64, error) {
	var total int64
	buf := make([]byte, 32<<10)
	for {
		n, e := reader.Read(buf)
		if n > 0 {
			m, err := w.Write(buf[:n])
			total += int64(m)
			if err != nil {
				return total, err
			}
			if m != n {
				return total, io.ErrShortWrite
			}
		}
		if e == io.EOF {
			return total, nil
		}
		if e != nil {
			return total, e
		}
		if w.r.Context().Err() != nil {
			return total, w.r.Context().Err()
		}
	}
}
func (w *privateWriter) FlushError() error {
	if e := w.release(); e != nil {
		return e
	}
	return http.NewResponseController(w.out).Flush()
}
func (w *privateWriter) Flush() { _ = w.FlushError() }
func (w *privateWriter) finish() {
	if !w.sent && !w.failed {
		_ = w.release()
	}
}
func (w *privateWriter) SetReadDeadline(d time.Time) error {
	if d.IsZero() || d.After(w.deadline) {
		d = w.deadline
	}
	return http.NewResponseController(w.out).SetReadDeadline(d)
}
func (w *privateWriter) SetWriteDeadline(d time.Time) error {
	if d.IsZero() || d.After(w.deadline) {
		d = w.deadline
	}
	return http.NewResponseController(w.out).SetWriteDeadline(d)
}
func (w *privateWriter) EnableFullDuplex() error { return http.ErrNotSupported }
func (w *privateWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return nil, nil, http.ErrNotSupported
}
func (w *privateWriter) Push(string, *http.PushOptions) error { return http.ErrNotSupported }
