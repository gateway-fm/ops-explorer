package verifier

import (
	"encoding/hex"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

type BytecodeComparison struct {
	MatchType        MatchType
	OnChainHash      string
	CompiledHash     string
	OnChainLength    int
	CompiledLength   int
	MetadataStripped bool
}

func CompareBytecode(onChain, compiled, constructorArgs string) *BytecodeComparison {
	onChain = strings.TrimPrefix(strings.ToLower(onChain), "0x")
	compiled = strings.TrimPrefix(strings.ToLower(compiled), "0x")
	constructorArgs = strings.TrimPrefix(strings.ToLower(constructorArgs), "0x")

	if constructorArgs != "" && strings.HasSuffix(onChain, constructorArgs) {
		onChain = onChain[:len(onChain)-len(constructorArgs)]
	}

	result := &BytecodeComparison{
		OnChainLength:  len(onChain) / 2,
		CompiledLength: len(compiled) / 2,
	}

	if onChainBytes, err := hex.DecodeString(onChain); err == nil {
		result.OnChainHash = common.BytesToHash(crypto.Keccak256(onChainBytes)).Hex()
	}
	if compiledBytes, err := hex.DecodeString(compiled); err == nil {
		result.CompiledHash = common.BytesToHash(crypto.Keccak256(compiledBytes)).Hex()
	}

	if onChain == compiled {
		result.MatchType = MatchTypeExact
		return result
	}

	onChainStripped := stripMetadata(onChain)
	compiledStripped := stripMetadata(compiled)

	if onChainStripped != "" && compiledStripped != "" && onChainStripped == compiledStripped {
		result.MatchType = MatchTypePartial
		result.MetadataStripped = true
		return result
	}

	result.MatchType = MatchTypeNone
	return result
}

// stripMetadata removes the CBOR-encoded metadata hash appended by solc at the end of bytecode.
func stripMetadata(bytecode string) string {
	if len(bytecode) < 86 { // Minimum length for metadata
		return bytecode
	}

	// Look for CBOR metadata marker (a2 64 69 70 66 73 = 0xa26469706673)
	// This is the start of the metadata: a2 64 'i' 'p' 'f' 's'
	marker := "a264697066"

	idx := strings.LastIndex(bytecode, marker)
	if idx == -1 {
		// Try older metadata format (a1 65 62 7a 7a 72 30 = 0xa165627a7a7230 for bzzr0)
		marker = "a165627a7a72"
		idx = strings.LastIndex(bytecode, marker)
	}

	if idx == -1 {
		return bytecode
	}

	return bytecode[:idx]
}

func ExtractMetadataHash(bytecode string) string {
	bytecode = strings.TrimPrefix(strings.ToLower(bytecode), "0x")

	if len(bytecode) < 86 {
		return ""
	}

	marker := "a264697066"
	idx := strings.LastIndex(bytecode, marker)

	if idx != -1 && idx+86 <= len(bytecode) {
		// Extract the IPFS hash (starts after marker + "73" + "58" + "22" = 6 chars)
		// The hash is 34 bytes (68 hex chars)
		hashStart := idx + 12
		if hashStart+68 <= len(bytecode) {
			return bytecode[hashStart : hashStart+68]
		}
	}

	marker = "a165627a7a72"
	idx = strings.LastIndex(bytecode, marker)

	if idx != -1 && idx+70 <= len(bytecode) {
		hashStart := idx + 14
		if hashStart+64 <= len(bytecode) {
			return bytecode[hashStart : hashStart+64]
		}
	}

	return ""
}

func BytecodeHash(bytecode string) string {
	bytecode = strings.TrimPrefix(bytecode, "0x")
	bytes, err := hex.DecodeString(bytecode)
	if err != nil {
		return ""
	}
	return common.BytesToHash(crypto.Keccak256(bytes)).Hex()
}

func NormalizeBytecode(bytecode string) string {
	bytecode = strings.TrimPrefix(strings.ToLower(bytecode), "0x")
	return strings.TrimRight(bytecode, "0")
}
