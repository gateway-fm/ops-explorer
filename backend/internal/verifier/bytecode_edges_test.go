package verifier

// Edge/robustness contract for bytecode comparison + metadata extraction
// (plan §3, no solc needed). These guard the untrusted-input and boundary
// behaviors the happy-path tests don't: constructor-arg stripping only when it
// is a real suffix, odd-length / non-hex no-panic, the len<86 metadata guard,
// both CBOR markers, multi-marker LastIndex, and truncated metadata.

import (
	"strings"
	"testing"
)

func TestCompareBytecode_ConstructorArgsNotASuffix(t *testing.T) {
	// constructorArgs that are NOT a suffix of onChain must NOT be stripped.
	// onChain == compiled already, so this is still an exact match — and the
	// non-suffix args must not corrupt onChain into a mismatch.
	compiled := "6080604052348015600f57"
	onChain := compiled
	args := "deadbeef" // not a suffix of onChain
	result := CompareBytecode(onChain, compiled, args)
	if result.MatchType != MatchTypeExact {
		t.Errorf("non-suffix args changed match to %s, want exact (args must not be stripped)", result.MatchType)
	}
	if result.OnChainLength != len(compiled)/2 {
		t.Errorf("OnChainLength = %d, want %d (args must not be stripped when not a suffix)", result.OnChainLength, len(compiled)/2)
	}
}

func TestCompareBytecode_EmptyConstructorArgs(t *testing.T) {
	b := "6080604052"
	if got := CompareBytecode(b, b, ""); got.MatchType != MatchTypeExact {
		t.Errorf("empty args exact match -> %s, want exact", got.MatchType)
	}
}

func TestCompareBytecode_ArgsWith0xPrefixStripped(t *testing.T) {
	// The 0x prefix on each input is trimmed, so a 0x-prefixed constructorArgs
	// still matches the (un-prefixed) suffix of onChain.
	compiled := "6080604052348015"
	args := "00000001"
	onChain := "0x" + compiled + args
	result := CompareBytecode(onChain, "0x"+compiled, "0x"+args)
	if result.MatchType != MatchTypeExact {
		t.Errorf("0x-prefixed args -> %s, want exact after stripping", result.MatchType)
	}
}

func TestCompareBytecode_OddLengthHexNoPanic(t *testing.T) {
	// Odd-length / non-hex strings must not panic the hex decode; the hashes are
	// just left empty and the match falls through to None.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("CompareBytecode panicked on odd-length input: %v", r)
		}
	}()
	result := CompareBytecode("abc", "abcde", "z") // odd length + non-hex
	if result == nil {
		t.Fatal("nil result")
	}
	if result.MatchType != MatchTypeNone {
		t.Errorf("MatchType = %s, want none for non-matching garbage", result.MatchType)
	}
}

func TestStripMetadata_LenGuard(t *testing.T) {
	// Below the 86-char minimum, the input is returned unchanged (even if it
	// happens to contain a marker substring — there can't be a valid trailer).
	short := "a264697066" // shorter than 86
	if got := stripMetadata(short); got != short {
		t.Errorf("stripMetadata(short) = %q, want unchanged %q", got, short)
	}
}

func TestStripMetadata_MultiMarkerUsesLast(t *testing.T) {
	// Two IPFS markers: stripMetadata uses LastIndex, so it strips at the LAST
	// marker (the real trailer), keeping everything before it.
	head := "6080604052aabbccddeeff00112233445566778899" // contains no marker
	trailer := "a264697066735822" + strings.Repeat("11", 34) + "0033"
	// Embed a marker earlier in the "code" too.
	withEarlyMarker := head + "a264697066" + "cafe" + trailer
	got := stripMetadata(withEarlyMarker)
	want := head + "a264697066" + "cafe"
	if got != want {
		t.Errorf("stripMetadata multi-marker = %q, want %q (LastIndex)", got, want)
	}
}

func TestExtractMetadataHash_TruncatedNoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("ExtractMetadataHash panicked: %v", r)
		}
	}()
	// Marker present but the trailer is truncated (not enough bytes for the
	// hash) -> "" (the hashStart+68 bound guards the slice).
	truncated := strings.Repeat("60", 40) + "a264697066735822" + "1111" // far short of 68
	if got := ExtractMetadataHash(truncated); got != "" {
		t.Errorf("ExtractMetadataHash(truncated) = %q, want \"\"", got)
	}
}

func TestExtractMetadataHash_LenGuardAndBzzr0(t *testing.T) {
	// len<86 -> "".
	if got := ExtractMetadataHash("a264697066"); got != "" {
		t.Errorf("len<86 -> %q, want \"\"", got)
	}
	// bzzr0 marker path returns the 64-char hash.
	bzzr := strings.Repeat("60", 30) + "a165627a7a72" + "3058" + strings.Repeat("ab", 32) + "0029"
	got := ExtractMetadataHash(bzzr)
	if len(got) != 64 {
		t.Errorf("bzzr0 hash len = %d, want 64 (got %q)", len(got), got)
	}
}

func TestExtractMetadataHash_IPFSHash(t *testing.T) {
	// Locks the EXISTING extraction offset: marker "a264697066" (10 chars) then
	// hashStart = idx+12 (skips marker + 2), 68 hex chars taken. Build the input
	// so the 68-char hash begins exactly at idx+12.
	hash := strings.Repeat("cd", 34) // 68 hex chars
	// The extractor also requires idx+86 <= len (a full solc metadata trailer),
	// so append a realistic solc-version tail after the hash.
	code := strings.Repeat("60", 30) + "a264697066" + "73" + hash + "64736f6c634300081300" + "0033"
	got := ExtractMetadataHash(code)
	if len(got) != 68 {
		t.Fatalf("IPFS hash len = %d, want 68 (got %q)", len(got), got)
	}
	if got != hash {
		t.Errorf("IPFS hash = %q, want %q", got, hash)
	}
}
