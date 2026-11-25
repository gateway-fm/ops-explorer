package verifier

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func TestMaskImmutablesZeroesRanges(t *testing.T) {
	code := bytes.Repeat([]byte{0xaa}, 64)
	refs := map[string][]LinkReference{
		"5": {{Start: 4, Length: 8}, {Start: 40, Length: 4}},
	}

	masked := MaskImmutables(code, refs)

	for i, b := range masked {
		inRange := (i >= 4 && i < 12) || (i >= 40 && i < 44)
		if inRange && b != 0 {
			t.Fatalf("byte %d not zeroed", i)
		}
		if !inRange && b != 0xaa {
			t.Fatalf("byte %d changed outside immutable ranges", i)
		}
	}
	if !bytes.Equal(code, bytes.Repeat([]byte{0xaa}, 64)) {
		t.Fatal("input slice was modified")
	}
}

func TestMaskImmutablesIgnoresOutOfBoundsRanges(t *testing.T) {
	code := bytes.Repeat([]byte{0xaa}, 16)
	refs := map[string][]LinkReference{
		"1": {{Start: 10, Length: 32}, {Start: -1, Length: 4}, {Start: 2, Length: 0}},
	}

	if masked := MaskImmutables(code, refs); !bytes.Equal(masked, code) {
		t.Fatalf("expected code unchanged, got %x", masked)
	}
}

func TestMaskImmutablesNoRefs(t *testing.T) {
	code := []byte{1, 2, 3}
	if masked := MaskImmutables(code, nil); !bytes.Equal(masked, code) {
		t.Fatalf("expected code unchanged, got %x", masked)
	}
}

func TestCompiledImmutablesMatchDeployedCode(t *testing.T) {
	solcPath := findSolc()
	if solcPath == "" {
		t.Skip("solc not available")
	}

	output := compileContract(t, solcPath, "SelfRef.sol", readContract(t, "SelfRef.sol"), true)
	contract, err := GetContractOutput(output, "SelfRef.sol", "SelfRef")
	if err != nil {
		t.Fatalf("GetContractOutput: %v", err)
	}

	deployed := contract.EVM.DeployedBytecode
	if len(deployed.ImmutableReferences) != 2 {
		t.Fatalf("expected 2 immutables, got %d", len(deployed.ImmutableReferences))
	}

	compiled, err := hex.DecodeString(deployed.Object)
	if err != nil {
		t.Fatalf("decode compiled bytecode: %v", err)
	}
	onChain := append([]byte(nil), compiled...)
	slots := 0
	for _, ranges := range deployed.ImmutableReferences {
		for _, r := range ranges {
			for i := r.Start; i < r.Start+r.Length; i++ {
				onChain[i] = 0x56
			}
			slots++
		}
	}
	if slots < 3 {
		t.Fatalf("expected at least 3 immutable slots, got %d", slots)
	}

	if got := CompareBytecode(hex.EncodeToString(onChain), deployed.Object, "").MatchType; got != MatchTypeNone {
		t.Fatalf("unmasked comparison: expected %s, got %s", MatchTypeNone, got)
	}

	masked := MaskImmutables(onChain, deployed.ImmutableReferences)
	if got := CompareBytecode(hex.EncodeToString(masked), deployed.Object, "").MatchType; got != MatchTypeExact {
		t.Fatalf("masked comparison: expected %s, got %s", MatchTypeExact, got)
	}
}
