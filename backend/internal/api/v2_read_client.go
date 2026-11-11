package api

import (
	"explorer/internal/authv2"
	"net/http"
	"time"
)

// V2ReadClient is a concrete source-bound transport for the selected V2 REST
// adapter. It cannot select a legacy DataProvider or an arbitrary destination.
type V2ReadClient struct {
	origin string
	client *http.Client
}

func NewV2ReadClient(session *authv2.Service) *V2ReadClient {
	if session == nil || !session.Ready() {
		return nil
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.MaxResponseHeaderBytes = 32 << 10
	return &V2ReadClient{origin: session.OPSOrigin(), client: &http.Client{Transport: transport, Timeout: 25 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}
func (*V2ReadClient) CallerScoped() {}

var _ CallerScopedProvider = (*V2ReadClient)(nil)
