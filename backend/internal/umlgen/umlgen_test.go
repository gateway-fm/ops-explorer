package umlgen

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteSourcesSingleFile(t *testing.T) {
	dir := t.TempDir()
	// Trailing whitespace is trimmed before writing.
	src := "// SPDX-License-Identifier: MIT\npragma solidity ^0.8.20;\ncontract Foo {}"

	if err := writeSources(dir, "Foo", src+"\n"); err != nil {
		t.Fatalf("writeSources: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dir, "Foo.sol"))
	if err != nil {
		t.Fatalf("expected Foo.sol: %v", err)
	}
	if string(got) != src {
		t.Errorf("content mismatch: got %q", string(got))
	}
}

func TestWriteSourcesEmptyNameFallsBack(t *testing.T) {
	dir := t.TempDir()
	if err := writeSources(dir, "", "contract Bar {}"); err != nil {
		t.Fatalf("writeSources: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "Contract.sol")); err != nil {
		t.Errorf("expected Contract.sol fallback: %v", err)
	}
}

func TestWriteSourcesStandardJSON(t *testing.T) {
	dir := t.TempDir()
	src := `{
		"language": "Solidity",
		"sources": {
			"contracts/A.sol": {"content": "contract A {}"},
			"lib/B.sol": {"content": "contract B {}"}
		}
	}`

	if err := writeSources(dir, "A", src); err != nil {
		t.Fatalf("writeSources: %v", err)
	}

	a, err := os.ReadFile(filepath.Join(dir, "contracts", "A.sol"))
	if err != nil || string(a) != "contract A {}" {
		t.Errorf("contracts/A.sol wrong: %q err=%v", string(a), err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "lib", "B.sol"))
	if err != nil || string(b) != "contract B {}" {
		t.Errorf("lib/B.sol wrong: %q err=%v", string(b), err)
	}
}

func TestWriteSourcesEtherscanDoubleBrace(t *testing.T) {
	dir := t.TempDir()
	// Etherscan wraps standard-JSON in an extra set of braces.
	src := `{{
		"language": "Solidity",
		"sources": {"Token.sol": {"content": "contract Token {}"}}
	}}`

	if err := writeSources(dir, "Token", src); err != nil {
		t.Fatalf("writeSources: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "Token.sol"))
	if err != nil || string(got) != "contract Token {}" {
		t.Errorf("Token.sol wrong: %q err=%v", string(got), err)
	}
}

func TestWriteSourcesContainsPathTraversal(t *testing.T) {
	dir := t.TempDir()
	src := `{"sources": {"../escape.sol": {"content": "contract Evil {}"}}}`

	if err := writeSources(dir, "Evil", src); err != nil {
		t.Fatalf("writeSources: %v", err)
	}

	// The "../" must be clamped inside dir — nothing may be written to the
	// parent directory.
	if _, err := os.Stat(filepath.Join(filepath.Dir(dir), "escape.sol")); err == nil {
		t.Fatal("path traversal escaped the scratch dir")
	}
	if _, err := os.Stat(filepath.Join(dir, "escape.sol")); err != nil {
		t.Errorf("expected contained escape.sol inside dir: %v", err)
	}
}

func TestParseStandardJSONPlainMap(t *testing.T) {
	// A bare {path: {content}} map (no "sources" wrapper) is also accepted.
	files, ok := parseStandardJSON(`{"X.sol": {"content": "contract X {}"}}`)
	if !ok || files["X.sol"] != "contract X {}" {
		t.Errorf("plain map not parsed: ok=%v files=%v", ok, files)
	}
}

func TestParseStandardJSONNotJSON(t *testing.T) {
	if _, ok := parseStandardJSON("pragma solidity ^0.8.0;"); ok {
		t.Error("plain Solidity should not parse as standard JSON")
	}
}
