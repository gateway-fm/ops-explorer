//go:build !privacy

package indexerclient

import "testing"

// offsetPage must never panic on a zero/negative limit (the old offset/limit
// expression did) and must clamp the page so the downstream int32(page) cast
// can't overflow.
func TestOffsetPage(t *testing.T) {
	cases := []struct {
		name           string
		offset, limit  int
		wantPage       int
		wantNormLimit  int
	}{
		{"first page", 0, 25, 1, 25},
		{"second page", 25, 25, 2, 25},
		{"deep page", 1000, 25, 41, 25},
		{"zero limit defaults", 0, 0, 1, 25},
		{"negative limit defaults", 50, -5, 3, 25},
		{"negative offset floored", -10, 25, 1, 25},
		{"huge offset clamps page", 1 << 60, 25, 1 << 30, 25},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			page, norm := offsetPage(c.offset, c.limit)
			if page != c.wantPage {
				t.Errorf("page = %d, want %d", page, c.wantPage)
			}
			if norm != c.wantNormLimit {
				t.Errorf("normLimit = %d, want %d", norm, c.wantNormLimit)
			}
			// The cast the guard exists to protect must be lossless.
			if int(int32(page)) != page {
				t.Errorf("page %d overflows int32", page)
			}
		})
	}
}
