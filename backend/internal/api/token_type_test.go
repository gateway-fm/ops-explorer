package api

import "testing"

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
