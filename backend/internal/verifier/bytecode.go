package verifier

import (
	"encoding/hex"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// BytecodeComparison represents the result of comparing two bytecodes
type BytecodeComparison struct {
	MatchType        MatchType
	OnChainHash      string
	CompiledHash     string
	OnChainLength    int
	CompiledLength   int
	MetadataStripped bool
}

// CompareBytecode compares on-chain bytecode with compiled bytecode
// It handles metadata hash stripping and constructor arguments
func CompareBytecode(onChain, compiled, constructorArgs string) *BytecodeComparison {
	// Normalize bytecodes (remove 0x prefix if present)
	onChain = strings.TrimPrefix(strings.ToLower(onChain), "0x")
	compiled = strings.TrimPrefix(strings.ToLower(compiled), "0x")
	constructorArgs = strings.TrimPrefix(strings.ToLower(constructorArgs), "0x")

	// Remove constructor arguments from on-chain bytecode if provided
	if constructorArgs != "" && strings.HasSuffix(onChain, constructorArgs) {
		onChain = onChain[:len(onChain)-len(constructorArgs)]
	}

	result := &BytecodeComparison{
		OnChainLength:  len(onChain) / 2,
		CompiledLength: len(compiled) / 2,
	}

	// Calculate hashes
	if onChainBytes, err := hex.DecodeString(onChain); err == nil {
		result.OnChainHash = common.BytesToHash(crypto.Keccak256(onChainBytes)).Hex()
	}
	if compiledBytes, err := hex.DecodeString(compiled); err == nil {
		result.CompiledHash = common.BytesToHash(crypto.Keccak256(compiledBytes)).Hex()
	}

	// Exact match
	if onChain == compiled {
		result.MatchType = MatchTypeExact
		return result
	}

	// Try matching with metadata stripped
	onChainStripped := stripMetadata(onChain)
	compiledStripped := stripMetadata(compiled)

	if onChainStripped != "" && compiledStripped != "" && onChainStripped == compiledStripped {
		result.MatchType = MatchTypePartial
		result.MetadataStripped = true
		return result
	}

	// No match
	result.MatchType = MatchTypeNone
	return result
}

// stripMetadata removes the CBOR-encoded metadata hash from bytecode
// The metadata is typically appended at the end of the bytecode
// Format: 0xa264... (CBOR encoded) followed by length bytes
func stripMetadata(bytecode string) string {
	if len(bytecode) < 86 { // Minimum length for metadata
		return bytecode
	}

	// Look for CBOR metadata marker (a2 64 69 70 66 73 = 0xa26469706673)
	// This is the start of the metadata: a2 64 'i' 'p' 'f' 's'
	marker := "a264697066"

	// Search for the marker from the end of the bytecode
	idx := strings.LastIndex(bytecode, marker)
	if idx == -1 {
		// Try older metadata format (a1 65 62 7a 7a 72 30 = 0xa165627a7a7230 for bzzr0)
		marker = "a165627a7a72"
		idx = strings.LastIndex(bytecode, marker)
	}

	if idx == -1 {
		// No metadata found
		return bytecode
	}

	// Return bytecode without metadata
	return bytecode[:idx]
}

// ExtractMetadataHash extracts the metadata hash from bytecode
func ExtractMetadataHash(bytecode string) string {
	bytecode = strings.TrimPrefix(strings.ToLower(bytecode), "0x")

	if len(bytecode) < 86 {
		return ""
	}

	// Look for IPFS metadata marker
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

	// Try bzzr0 format
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

// BytecodeHash computes the keccak256 hash of bytecode
func BytecodeHash(bytecode string) string {
	bytecode = strings.TrimPrefix(bytecode, "0x")
	bytes, err := hex.DecodeString(bytecode)
	if err != nil {
		return ""
	}
	return common.BytesToHash(crypto.Keccak256(bytes)).Hex()
}

// NormalizeBytecode normalizes bytecode for comparison
func NormalizeBytecode(bytecode string) string {
	bytecode = strings.TrimPrefix(strings.ToLower(bytecode), "0x")
	// Remove trailing zeros (padding)
	return strings.TrimRight(bytecode, "0")
}
