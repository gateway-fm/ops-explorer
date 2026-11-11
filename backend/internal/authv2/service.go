package authv2

import (
	"context"
	"crypto/subtle"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// Service owns only this confidential application's bounded operational state.
// The approved Source and signing keys remain immutable for its lifetime.
type Service struct {
	source *Source
	codec  *codec
	store  *store
	clock  Clock
	client *http.Client
	slots  chan struct{}
	stop   context.CancelFunc
	done   chan struct{}
	once   sync.Once
}

func New(ctx context.Context, source *Source, bindings []SecretBinding, dsn string, clock Clock) (*Service, error) {
	if clock == nil {
		clock = func() int64 { return time.Now().Unix() }
	}
	c, e := newCodec(source, bindings)
	if e != nil {
		return nil, e
	}
	st, e := openStore(ctx, source, dsn, clock)
	if e != nil {
		return nil, e
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.MaxResponseHeaderBytes = 16 << 10
	transport.ResponseHeaderTimeout = 20 * time.Second
	client := &http.Client{Transport: transport, Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	loop, cancel := context.WithCancel(ctx)
	s := &Service{source: source, codec: c, store: st, clock: clock, client: client, slots: make(chan struct{}, 32), stop: cancel, done: make(chan struct{})}
	go func() {
		defer close(s.done)
		if !source.spec.Retention.Enabled {
			<-loop.Done()
			return
		}
		timer := time.NewTicker(time.Duration(source.spec.Retention.IntervalMillis) * time.Millisecond)
		defer timer.Stop()
		for {
			select {
			case <-loop.Done():
				return
			case <-timer.C:
				_ = st.retain(loop)
			}
		}
	}()
	return s, nil
}
func (s *Service) Close() {
	if s == nil {
		return
	}
	s.once.Do(func() {
		s.stop()
		<-s.done
		s.store.close()
		if t, ok := s.client.Transport.(*http.Transport); ok {
			t.CloseIdleConnections()
		}
	})
}
func (s *Service) Origin() string {
	if s == nil {
		return ""
	}
	return s.source.spec.AppOrigin
}
func (s *Service) OPSOrigin() string {
	if s == nil {
		return ""
	}
	return s.source.spec.OPSOrigin
}
func (s *Service) SetupJSON() []byte {
	if s == nil {
		return nil
	}
	return s.source.SetupJSON()
}
func (s *Service) Ready() bool {
	return s != nil && s.source != nil && s.store != nil && !s.store.closed.Load()
}

func returnPath(raw string) bool {
	if raw == "" || len(raw) > 1024 || !utf8.ValidString(raw) || raw[0] != '/' || strings.HasPrefix(raw, "//") || strings.Contains(raw, "\\") {
		return false
	}
	for _, c := range raw {
		if c < 0x20 || c == 0x7f {
			return false
		}
	}
	u, e := url.Parse(raw)
	if e != nil || u.IsAbs() || u.Host != "" || u.User != nil || u.Fragment != "" || u.RawFragment != "" || u.Opaque != "" || u.String() != raw || path.Clean(u.Path) != u.Path || strings.HasPrefix(u.Path, "//") || strings.Contains(u.Path, "\\") {
		return false
	}
	// Encoded controls and network-path aliases cannot become a later redirect.
	for _, c := range u.Path {
		if c < 0x20 || c == 0x7f {
			return false
		}
	}
	return true
}
func equalToken(a, b string) bool {
	return len(a) == len(b) && subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func (s *store) retain(parent context.Context) error {
	tx, ctx, cancel, e := s.begin(parent, true)
	if e != nil {
		return e
	}
	defer cancel()
	defer tx.Rollback(ctx)
	q, e := s.state(ctx, tx, true)
	if e != nil {
		return e
	}
	now, e := s.now()
	if e != nil {
		return e
	}
	rows, e := tx.Query(ctx, `SELECT browser_hash,COALESCE(octet_length(token_cipher),0) FROM explorer_sso_v2.contexts WHERE retain_until<=$1 AND config_digest=$3 AND recovery_epoch=$4 ORDER BY retain_until,browser_hash LIMIT $2 FOR UPDATE`, now, s.source.spec.Retention.BatchSize, s.source.digest[:], s.source.spec.RecoveryEpoch)
	if e != nil {
		return ErrUnavailable
	}
	var ids [][]byte
	var removed int64
	for rows.Next() {
		var id []byte
		var n int64
		if rows.Scan(&id, &n) != nil {
			rows.Close()
			return ErrUnavailable
		}
		ids = append(ids, id)
		removed += n
	}
	rows.Close()
	if rows.Err() != nil {
		return ErrUnavailable
	}
	if len(ids) > 0 {
		tag, e := tx.Exec(ctx, `DELETE FROM explorer_sso_v2.contexts WHERE browser_hash=ANY($1::bytea[]) AND retain_until<=$2 AND config_digest=$3 AND recovery_epoch=$4`, ids, now, s.source.digest[:], s.source.spec.RecoveryEpoch)
		if e != nil || tag.RowsAffected() != int64(len(ids)) {
			return ErrUnavailable
		}
		q.rows -= len(ids)
		q.cipher -= removed
		if q.rows < 0 || q.cipher < 0 {
			return ErrUnavailable
		}
		if _, e = tx.Exec(ctx, `UPDATE explorer_sso_v2.store_state SET context_count=$1,cipher_bytes=$2 WHERE id=1`, q.rows, q.cipher); e != nil {
			return ErrUnavailable
		}
	}
	if tx.Commit(ctx) != nil {
		return ErrUnknownCommit
	}
	return nil
}

type requestSession struct {
	capture activeCapture
	raw     string
	claims  *claims
}
type sessionKey struct{}

// Token returns the original signed OPS token solely for an authenticated,
// request-scoped upstream call. It is never a browser response credential.
func Token(ctx context.Context) string {
	v, _ := ctx.Value(sessionKey{}).(*requestSession)
	if v == nil {
		return ""
	}
	return v.raw
}
func Actor(ctx context.Context) string {
	v, _ := ctx.Value(sessionKey{}).(*requestSession)
	if v == nil {
		return ""
	}
	return v.claims.Subject
}
func (s *Service) session(ctx context.Context, browser, auth string) (*requestSession, error) {
	if !token32(browser) || !token32(auth) {
		return nil, ErrUnauthenticated
	}
	b := handleHash("browser", browser)
	r, e := s.store.read(ctx, b)
	if e != nil {
		return nil, e
	}
	a, ok := r.active()
	if !ok || a.auth != handleHash("auth", auth) {
		return nil, ErrUnauthenticated
	}
	raw, c, e := s.codec.openToken(a, s.clock())
	if e != nil {
		return nil, e
	}
	return &requestSession{a, raw, c}, nil
}
func (s *Service) recheck(ctx context.Context, session *requestSession) error {
	if session == nil {
		return ErrUnauthenticated
	}
	r, e := s.store.read(ctx, session.capture.browser)
	if e != nil {
		return e
	}
	a, ok := r.active()
	if !ok || !sameActive(a, session.capture) {
		return ErrChanged
	}
	if ctx.Err() != nil {
		return ErrUnavailable
	}
	if _, _, e = s.codec.openToken(a, s.clock()); e != nil {
		return e
	}
	return nil
}
