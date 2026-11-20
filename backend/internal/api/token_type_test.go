package api

import (
	"context"
	"explorer/internal/types"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestNormalizeTokenType(t *testing.T) {
	cases := map[string]string{
		"ERC20":     "ERC20",
		"erc20":     "ERC20",
		"  ERC721 ": "ERC721",
		"Erc1155":   "ERC1155",
		"":          "",
		"erc777":    "", // unknown standard → no filter
		"garbage":   "",
		"' OR 1=1":  "", // not an allowlisted value
	}
	for in, want := range cases {
		if got := NormalizeTokenType(in); got != want {
			t.Errorf("NormalizeTokenType(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseAllTransferTypeDoesNotBroadenUnknownOrDuplicate(t *testing.T) {
	for _, x := range []struct{ raw, want string }{{"", ""}, {"type=", ""}, {"type=erc20", "ERC20"}, {"type=+ERC721+", "ERC721"}, {"type=Erc1155", "ERC1155"}} {
		v, _ := url.ParseQuery(x.raw)
		got, e := ParseTransferTokenType(v)
		if e != nil || got != x.want {
			t.Fatal(x, got, e)
		}
	}
	for _, raw := range []string{"type=garbage", "type=+", "type=ERC20&type=ERC20", "type=ERC20&type=ERC721"} {
		v, _ := url.ParseQuery(raw)
		if _, e := ParseTransferTokenType(v); e == nil {
			t.Fatal("invalid type became unfiltered", raw)
		}
	}
}

type transferTypeProvider struct {
	DataProvider
	kind  string
	calls int
}

func (p *transferTypeProvider) GetAllTransfers(_ context.Context, kind string, _, _ int) ([]types.TokenTransfer, int64, error) {
	p.kind = kind
	p.calls++
	return []types.TokenTransfer{}, 0, nil
}
func TestAllTransferTypeValidatedBeforePrivateProvider(t *testing.T) {
	p := &transferTypeProvider{}
	s := &Server{provider: p}
	get := func(raw string) int {
		w := httptest.NewRecorder()
		s.handleGetAllTransfers(w, httptest.NewRequest("GET", "/token-transfers?"+raw, nil))
		return w.Code
	}
	for _, raw := range []string{"type=erc20", "type=+ERC721+", "type=Erc1155"} {
		if get(raw) != 200 || p.kind == "" {
			t.Fatal(raw, p.kind)
		}
	}
	before := p.calls
	for _, raw := range []string{"type=ERC777", "type=+", "type=ERC20&type=ERC20", "type=%zz"} {
		if get(raw) != 400 || p.calls != before {
			t.Fatal("invalid filter reached provider", raw)
		}
	}
}
