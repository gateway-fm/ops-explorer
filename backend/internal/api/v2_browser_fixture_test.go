//go:build privacy

package api

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"explorer/internal/authv2"
	"explorer/internal/authv2admin"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// This child fixture runs the actual selected BFF and built app against an
// independently running actual OPS broker. It adds no production route/seam.
func TestOwnedE2BBrowserApp(t *testing.T) {
	file := os.Getenv("E2B_OWNED_APP_CONFIG")
	if file == "" {
		t.Skip("owned cross-repository browser fixture only")
	}
	var config struct {
		Source                                                              json.RawMessage
		SHA256, OwnerDSN, AppDSN, Listen, OPSAddress, OPSCertificate, Build string
		Now                                                                 int64
		WallClock                                                           bool
		ClientSecret                                                        string
	}
	raw, e := os.ReadFile(file)
	if e != nil || json.Unmarshal(raw, &config) != nil {
		t.Fatal("owned app config")
	}
	digest, e := authv2.ExpectedSHA(config.SHA256)
	if e != nil {
		t.Fatal(e)
	}
	source, e := authv2.ParseSource(config.Source, digest)
	if e != nil {
		t.Fatal(e)
	}
	cert, e := base64.RawStdEncoding.DecodeString(config.OPSCertificate)
	if e != nil {
		t.Fatal(e)
	}
	certificate, e := x509.ParseCertificate(cert)
	if e != nil {
		t.Fatal(e)
	}
	roots := x509.NewCertPool()
	roots.AddCert(certificate)
	original := http.DefaultTransport
	transport := original.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.TLSClientConfig = &tls.Config{RootCAs: roots, ServerName: certificate.DNSNames[0], MinVersion: tls.VersionTLS12}
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		if address != config.OPSAddress {
			return nil, fmt.Errorf("owned fixture egress denied")
		}
		_, port, _ := net.SplitHostPort(address)
		return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, net.JoinHostPort("127.0.0.1", port))
	}
	http.DefaultTransport = transport
	t.Cleanup(func() { transport.CloseIdleConnections(); http.DefaultTransport = original })
	ctx, cancel := context.WithTimeout(t.Context(), 240*time.Second)
	defer cancel()
	if e = authv2admin.Run(ctx, config.OwnerDSN, source, "migrate", false); e != nil {
		t.Fatal("owned offline migration", e)
	}
	if e = authv2admin.Run(ctx, config.OwnerDSN, source, "initialize", false); e != nil {
		t.Fatal("owned initialization", e)
	}
	var clock atomic.Int64
	clock.Store(config.Now)
	bindings := []authv2.SecretBinding{{FromEnv: "E2B_CLIENT", Value: []byte(config.ClientSecret)}, {FromEnv: "E2B_STATE", Value: bytesOf(17)}, {FromEnv: "E2B_TOKEN", Value: bytesOf(18)}}
	selectedClock := clock.Load
	if config.WallClock {
		selectedClock = func() int64 { return time.Now().Unix() }
	}
	service, e := authv2.New(ctx, source, bindings, config.AppDSN, selectedClock)
	if e != nil {
		t.Fatal("actual restricted app", e)
	}
	defer service.Close()
	provider := NewV2ReadClient(service)
	server, e := NewV2(service, provider, 8080)
	if e != nil {
		t.Fatal(e)
	}
	if server.db != nil || server.rpc != nil || server.provider != nil || server.ssoClient != nil || server.jwtVerifier != nil || server.chainInfo != nil || server.wsHub != nil || server.impersonations != nil || server.verifier != nil {
		t.Fatal("selected V2 constructed a legacy data/auth component")
	}
	mux := http.NewServeMux()
	mux.Handle("/api/", server.Handler())
	mux.HandleFunc("/__test/advance", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.Header.Get("Origin") != service.Origin() {
			w.WriteHeader(403)
			return
		}
		var v struct{ Seconds int64 }
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 128)).Decode(&v) != nil || v.Seconds < 0 || v.Seconds > 301 {
			w.WriteHeader(400)
			return
		}
		clock.Add(v.Seconds)
		w.WriteHeader(204)
	})
	// Identity echo qualifies original bearer/context gating only. It is never
	// called a history endpoint or a substitute for the required H1d adapter.
	mux.Handle("/__test/private", service.Boundary(service.Protected(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"actor": authv2.Actor(r.Context()), "tokenDigest": hashText(authv2.Token(r.Context()))})
	})), nil))
	mux.HandleFunc("/config.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte(`window.__runtimeConfig={VITE_AUTH_PROTOCOL:"ops-v2",VITE_API_URL:"/api",VITE_PRIVACY_MODE:"true"};`))
	})
	files := http.FileServer(http.Dir(config.Build))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/assets/") {
			files.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(config.Build, "index-v2.html"))
	})
	listener, e := net.Listen("tcp", config.Listen)
	if e != nil {
		t.Fatal(e)
	}
	tlsServer := httptest.NewUnstartedServer(mux)
	_ = tlsServer.Listener.Close()
	tlsServer.Listener = listener
	tlsServer.StartTLS()
	defer tlsServer.Close()
	fmt.Println("E2B_APP_READY")
	signalCtx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()
	<-signalCtx.Done()
}
func bytesOf(v byte) []byte {
	out := make([]byte, 32)
	for i := range out {
		out[i] = v
	}
	return out
}
func hashText(v string) string { sum := sha256.Sum256([]byte(v)); return hex.EncodeToString(sum[:]) }
