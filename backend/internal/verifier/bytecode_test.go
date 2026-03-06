package verifier

import (
	"strings"
	"testing"
)

func TestCompareBytecodeExactMatch(t *testing.T) {
	bytecode := "6080604052348015600f57600080fd5b506004361060325760003560e01c806360fe47b11460375780636d4ce63c14604f575b600080fd5b"

	result := CompareBytecode(bytecode, bytecode, "")
	if result.MatchType != MatchTypeExact {
		t.Errorf("expected MatchTypeExact, got %s", result.MatchType)
	}
}

func TestCompareBytecodeExactMatchWith0xPrefix(t *testing.T) {
	bytecode := "6080604052348015600f57600080fd5b506004361060325760003560e01c806360fe47b11460375780636d4ce63c14604f575b600080fd5b"

	result := CompareBytecode("0x"+bytecode, bytecode, "")
	if result.MatchType != MatchTypeExact {
		t.Errorf("expected MatchTypeExact with 0x prefix normalization, got %s", result.MatchType)
	}
}

func TestCompareBytecodePartialMatch(t *testing.T) {
	// Same bytecode body but with different IPFS metadata appended
	body := "6080604052348015600f57600080fd5b506004361060325760003560e01c806360fe47b11460375780636d4ce63c14604f575b600080fd5b"
	// a264697066 = CBOR IPFS metadata marker
	meta1 := "a264697066735822122011111111111111111111111111111111111111111111111111111111111111110064736f6c63430008130033"
	meta2 := "a264697066735822122022222222222222222222222222222222222222222222222222222222222222220064736f6c63430008130033"

	result := CompareBytecode(body+meta1, body+meta2, "")
	if result.MatchType != MatchTypePartial {
		t.Errorf("expected MatchTypePartial, got %s", result.MatchType)
	}
	if !result.MetadataStripped {
		t.Error("expected MetadataStripped to be true")
	}
}

func TestCompareBytecodeNoMatch(t *testing.T) {
	bytecodeA := "6080604052348015600f57600080fd5b50600436106032"
	bytecodeB := "7090705063459126711080fe5c61700537106143"

	result := CompareBytecode(bytecodeA, bytecodeB, "")
	if result.MatchType != MatchTypeNone {
		t.Errorf("expected MatchTypeNone, got %s", result.MatchType)
	}
}

func TestCompareBytecodeWithConstructorArgs(t *testing.T) {
	compiled := "6080604052348015600f57600080fd5b506004361060325760003560e01c"
	constructorArgs := "0000000000000000000000000000000000000000000000000000000000000001"
	onChain := compiled + constructorArgs

	result := CompareBytecode(onChain, compiled, constructorArgs)
	if result.MatchType != MatchTypeExact {
		t.Errorf("expected MatchTypeExact after stripping constructor args, got %s", result.MatchType)
	}
}

func TestStripMetadata(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "strip IPFS metadata",
			input:    "608060405234" + "a264697066735822122011111111111111111111111111111111111111111111111111111111111111110064736f6c63430008130033",
			expected: "608060405234",
		},
		{
			name:     "strip bzzr0 metadata",
			input:    "608060405234" + "a165627a7a72305820aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa0029",
			expected: "608060405234",
		},
		{
			name:     "no metadata - short bytecode",
			input:    "6080",
			expected: "6080",
		},
		{
			name:     "no metadata marker",
			input:    "6080604052348015600f57600080fd5b506004361060325760003560e01c806360fe47b11460375780636d4ce63c14604f575b600080fd5b",
			expected: "6080604052348015600f57600080fd5b506004361060325760003560e01c806360fe47b11460375780636d4ce63c14604f575b600080fd5b",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stripMetadata(tt.input)
			if result != tt.expected {
				t.Errorf("stripMetadata(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestBytecodeHash(t *testing.T) {
	bytecode := "6080604052"
	hash := BytecodeHash(bytecode)

	if hash == "" {
		t.Fatal("expected non-empty hash")
	}
	if !strings.HasPrefix(hash, "0x") {
		t.Errorf("expected hash to start with 0x, got %s", hash)
	}
	if len(hash) != 66 { // 0x + 64 hex chars
		t.Errorf("expected hash length 66, got %d", len(hash))
	}

	// Same input should produce same hash
	hash2 := BytecodeHash(bytecode)
	if hash != hash2 {
		t.Errorf("expected deterministic hash, got %s and %s", hash, hash2)
	}
}

func TestBytecodeHashWith0xPrefix(t *testing.T) {
	hash1 := BytecodeHash("6080604052")
	hash2 := BytecodeHash("0x6080604052")
	if hash1 != hash2 {
		t.Errorf("expected same hash with and without 0x prefix, got %s and %s", hash1, hash2)
	}
}

func TestNormalizeBytecode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "strip 0x prefix and lowercase",
			input:    "0xABCDEF",
			expected: "abcdef",
		},
		{
			name:     "trim trailing zeros",
			input:    "0x608060400000",
			expected: "6080604",
		},
		{
			name:     "already normalized",
			input:    "608060405234",
			expected: "608060405234",
		},
		{
			name:     "uppercase to lowercase",
			input:    "ABCDEF1234",
			expected: "abcdef1234",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeBytecode(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeBytecode(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
