package log

import "testing"

// P-3: redactPath must mask on-chain identifiers embedded in request paths so
// privacy-mode access logs cannot be used to correlate an authenticated DID
// with the real addresses / grants it viewed. The spec (PROD_READINESS_AUDIT
// §P-3) requires masking 0x-hex segments (addresses AND tx hashes) and the two
// opaque segments after /grant/, regardless of their textual format (UUIDs are
// just as sensitive as any other identifier), while leaving non-identifier
// paths intact.
func TestRedactPath(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "address path is masked",
			in:   "/api/addresses/0x52908400098527886E0F7030069857D2E4169EE7/transactions",
			want: "/api/addresses/[redacted]/transactions",
		},
		{
			name: "tx hash path is masked",
			in:   "/api/transactions/0x88df016429689c079f3b2f6ad39fa052532c56795b733da78a91ebe6a713944b",
			want: "/api/transactions/[redacted]",
		},
		{
			name: "grant uuid pair is masked regardless of format",
			in:   "/api/privacy/grant/3f2504e0-4f89-41d3-9a0c-0305e82c3301/9b7e4f5a-1c2d-4e3f-8a9b-0c1d2e3f4a5b/transactions",
			want: "/api/privacy/grant/[redacted]/[redacted]/transactions",
		},
		{
			name: "grant pair short ids still masked",
			in:   "/api/privacy/grant/g1/a1",
			want: "/api/privacy/grant/[redacted]/[redacted]",
		},
		{
			name: "grant activity single segment masked",
			in:   "/api/privacy/grant/3f2504e0-4f89-41d3-9a0c-0305e82c3301/activity",
			want: "/api/privacy/grant/[redacted]/[redacted]",
		},
		{
			name: "plain block listing is untouched",
			in:   "/api/v1/blocks",
			want: "/api/v1/blocks",
		},
		{
			name: "block-by-number is untouched (decimal, not hex 0x)",
			in:   "/api/v1/blocks/12345",
			want: "/api/v1/blocks/12345",
		},
		{
			name: "root is untouched",
			in:   "/",
			want: "/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := redactPath(tt.in)
			if got != tt.want {
				t.Errorf("redactPath(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// P-3: when RedactHTTPPaths is false (standalone default) redactPath must be a
// no-op so public-explorer access logs keep full paths for debugging.
func TestRedactPath_OffByDefault(t *testing.T) {
	if RedactHTTPPaths {
		t.Fatal("RedactHTTPPaths must default to false (standalone is permissive)")
	}
}
