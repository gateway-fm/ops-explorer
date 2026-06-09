package verifier

// Validation-ladder contract for VerifyFromJSON (plan §3, no solc needed). Every
// case below returns BEFORE v.Verify() reaches the compiler, so the mock DB/RPC
// are never invoked. Locks: invalid JSON, the required-field order, and the
// legacy sourceCode->sourceFiles conversion.

import (
	"context"
	"strings"
	"testing"
)

func newValidationVerifier(t *testing.T) *Verifier {
	t.Helper()
	// Pre-compiler validation never touches DB/RPC; nil-ish mocks suffice.
	return NewVerifier(new(MockDatabase), new(MockRPCClient), &Config{SolcPath: "/nonexistent/solc"})
}

func TestVerifyFromJSON_ValidationLadder(t *testing.T) {
	v := newValidationVerifier(t)
	cases := []struct {
		name    string
		body    string
		wantErr string // substring expected in VerifyResponse.Error
	}{
		{"invalid-json", `{not json`, "invalid request"},
		{"missing-address", `{}`, "address is required"},
		{
			"missing-source",
			`{"address":"0xabc"}`,
			"sourceFiles or sourceCode is required",
		},
		{
			"missing-contract-name",
			`{"address":"0xabc","sourceCode":"contract C {}"}`,
			"contractName is required",
		},
		{
			"missing-compiler-version",
			`{"address":"0xabc","sourceCode":"contract C {}","contractName":"C"}`,
			"compilerVersion is required",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := v.VerifyFromJSON(context.Background(), []byte(tc.body))
			if err != nil {
				t.Fatalf("VerifyFromJSON returned a Go error %v; validation failures use VerifyResponse.Error", err)
			}
			if resp == nil || !strings.Contains(resp.Error, tc.wantErr) {
				t.Errorf("Error = %q, want substring %q", respErr(resp), tc.wantErr)
			}
		})
	}
}

// TestVerifyFromJSON_LegacySourceCodeConversion proves the legacy sourceCode ->
// sourceFiles conversion runs: a request carrying ONLY sourceCode (no
// sourceFiles) and no contractName must fail at "contractName is required" —
// NOT "sourceFiles or sourceCode is required". If the conversion didn't happen
// the source check would trip first.
func TestVerifyFromJSON_LegacySourceCodeConversion(t *testing.T) {
	v := newValidationVerifier(t)
	resp, err := v.VerifyFromJSON(context.Background(), []byte(`{"address":"0xabc","sourceCode":"contract C {}"}`))
	if err != nil {
		t.Fatalf("unexpected go error: %v", err)
	}
	if strings.Contains(resp.Error, "sourceFiles or sourceCode is required") {
		t.Fatalf("source check tripped -> legacy sourceCode was NOT converted to sourceFiles")
	}
	if !strings.Contains(resp.Error, "contractName is required") {
		t.Errorf("Error = %q, want contractName-required (proves conversion ran, then later checks apply)", resp.Error)
	}
}

func respErr(r *VerifyResponse) string {
	if r == nil {
		return "<nil response>"
	}
	return r.Error
}
