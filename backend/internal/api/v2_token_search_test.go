package api

import (
	"net/url"
	"strings"
	"testing"
)

func TestV2TokenSearchIsFiniteAndTokenOnly(t *testing.T) {
	for _, text := range []string{"BBD", "Example coin", "0x" + strings.Repeat("a", 40), "é"} {
		raw := url.Values{"search": {text}, "type": {"ERC20"}, "limit": {"25"}}.Encode()
		actual, err := v2ReadQuery(raw, "tokens")
		if err != nil {
			t.Fatal(err)
		}
		q, _ := url.ParseQuery(actual)
		if q.Get("search") != text {
			t.Fatal("search changed")
		}
	}
	for _, text := range []string{"", " padded", "padded ", "line\nfeed", strings.Repeat("é", 65)} {
		if _, err := v2ReadQuery(url.Values{"search": {text}}.Encode(), "tokens"); err == nil {
			t.Fatal("invalid token search passed")
		}
	}
	for _, kind := range []string{"page", "transactions", "logs", "search", ""} {
		if _, err := v2ReadQuery("search=BBD", kind); err == nil {
			t.Fatalf("search widened %s", kind)
		}
	}
	if _, err := v2ReadQuery("search=BBD&search=other", "tokens"); err == nil {
		t.Fatal("duplicate search passed")
	}
	if _, err := v2ReadQuery("q=BBD", "tokens"); err == nil {
		t.Fatal("suggestion query changed token contract")
	}
}
