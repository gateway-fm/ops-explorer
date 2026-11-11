package authv2

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"
)

// Owned-fixture pgwire relay: PostgreSQL actually commits, then its complete
// response is withheld. It does not forge a transaction result or retry SQL.
type commitRelay struct {
	listener  net.Listener
	remaining atomic.Int64
	withheld  atomic.Int64
}

func newCommitRelay(t *testing.T, backend string) *commitRelay {
	t.Helper()
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	p := &commitRelay{listener: listener}
	t.Cleanup(func() { listener.Close() })
	go func() {
		for {
			client, e := listener.Accept()
			if e != nil {
				return
			}
			server, e := net.Dial("tcp", backend)
			if e != nil {
				client.Close()
				continue
			}
			go p.connection(client, server)
		}
	}()
	return p
}
func (p *commitRelay) connection(client, server net.Conn) {
	defer client.Close()
	defer server.Close()
	var lost atomic.Bool
	go func() {
		defer client.Close()
		defer server.Close()
		for {
			kind, length, data, e := packet(server)
			if e != nil {
				return
			}
			if lost.Load() {
				if kind == 'Z' {
					p.withheld.Add(1)
					return
				}
				continue
			}
			if _, e = client.Write(append(append([]byte{kind}, length...), data...)); e != nil {
				return
			}
		}
	}()
	var first [4]byte
	if _, e := io.ReadFull(client, first[:]); e != nil {
		return
	}
	n := binary.BigEndian.Uint32(first[:])
	if n < 8 || n > 16384 {
		return
	}
	data := make([]byte, n-4)
	if _, e := io.ReadFull(client, data); e != nil {
		return
	}
	if _, e := server.Write(append(first[:], data...)); e != nil {
		return
	}
	for {
		kind, length, data, e := packet(client)
		if e != nil {
			return
		}
		if kind == 'Q' && strings.EqualFold(strings.TrimSuffix(string(data), "\x00"), "commit") {
			for {
				n := p.remaining.Load()
				if n <= 0 {
					break
				}
				if p.remaining.CompareAndSwap(n, n-1) {
					if n == 1 {
						lost.Store(true)
					}
					break
				}
			}
		}
		if _, e = server.Write(append(append([]byte{kind}, length...), data...)); e != nil {
			return
		}
	}
}
func packet(r io.Reader) (byte, []byte, []byte, error) {
	var head [5]byte
	if _, e := io.ReadFull(r, head[:]); e != nil {
		return 0, nil, nil, e
	}
	n := binary.BigEndian.Uint32(head[1:])
	if n < 4 || n > 1<<20 {
		return 0, nil, nil, io.ErrUnexpectedEOF
	}
	body := make([]byte, n-4)
	_, e := io.ReadFull(r, body)
	return head[0], head[1:], body, e
}

func TestDurableCallbackCommitWithheldNeverEmitsOrReplays(t *testing.T) {
	s, owner, clock, key := fixture(t)
	calls := atomic.Int64{}
	ops := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		safeJSON(w, 200, map[string]any{"access_token": sign(t, key, testClaims(s.source, clock.Load())), "token_type": "Bearer", "expires_in": 300})
	}))
	defer ops.Close()
	fixtureRebind(t, s, ops.URL)
	s.client.Transport = ops.Client().Transport
	ownerURL, e := url.Parse(os.Getenv("E2B_TEST_OWNER_DSN"))
	if e != nil {
		t.Fatal(e)
	}
	proxy := newCommitRelay(t, ownerURL.Host)
	ownerURL.Host = proxy.listener.Addr().String()
	ownerURL.User = url.UserPassword(AppRole, "owned-app-fixture")
	st, e := openStore(context.Background(), s.source, ownerURL.String(), clock.Load)
	if e != nil {
		t.Fatal(e)
	}
	s.store.close()
	s.store = st
	start := authCall(s, req("POST", "/api/auth/login", strings.NewReader(`{"mode":"login","returnPath":"/"}`), nil))
	if start.Code != 200 {
		t.Fatal("start", start.Code)
	}
	var value struct {
		URL string `json:"authorizationUrl"`
	}
	_ = json.Unmarshal(start.Body.Bytes(), &value)
	u, _ := url.Parse(value.URL)
	code, _ := randomToken()
	path := "/api/auth/callback?" + url.Values{"code": {code}, "state": {u.Query().Get("state")}}.Encode()
	proxy.remaining.Store(2) // initial known read succeeds; activation COMMIT lost.
	result := authCall(s, req("GET", path, nil, start.Result().Cookies()))
	if result.Code != 503 || len(result.Result().Cookies()) != 0 || result.Header().Get("Location") != "" || calls.Load() != 1 || proxy.withheld.Load() != 1 {
		t.Fatal("uncertain activation emitted output or retried", result.Code, calls.Load(), proxy.withheld.Load())
	}
	var active int
	if e = owner.QueryRow(context.Background(), `SELECT count(*) FROM explorer_sso_v2.contexts WHERE active_auth_hash IS NOT NULL AND pending_hash IS NULL`).Scan(&active); e != nil || active != 1 {
		t.Fatal("owned relay did not withhold an actual durable activation", e)
	}
	replay := authCall(s, req("GET", path, nil, start.Result().Cookies()))
	if replay.Code != 401 || calls.Load() != 1 || len(replay.Result().Cookies()) != 0 {
		t.Fatal("durable-but-withheld callback replayed consumed code")
	}
}
