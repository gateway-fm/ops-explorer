// Package umlgen renders a UML class diagram for a verified contract's
// Solidity source by shelling out to sol2uml (https://github.com/naddison36/sol2uml),
// mirroring the "View UML diagram" feature Blockscout exposes on the contract tab.
//
// The backend is pure Go, so — just like the solc verifier (see
// internal/verifier/compiler.go) — we invoke the tool as an external process.
// sol2uml is a Node CLI; the api container installs it globally (see
// backend/Dockerfile). The binary path is configurable via SOL2UML_PATH so the
// same code works locally (`sol2uml` on PATH) and in the container.
package umlgen

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Generator renders UML diagrams via the sol2uml CLI.
type Generator struct {
	path    string
	timeout time.Duration
}

// New returns a Generator. If path is empty it falls back to the SOL2UML_PATH
// env var, then to "sol2uml" on PATH.
func New(path string) *Generator {
	if path == "" {
		path = os.Getenv("SOL2UML_PATH")
	}
	if path == "" {
		path = "sol2uml"
	}
	return &Generator{path: path, timeout: 60 * time.Second}
}

// GenerateSVG writes the contract source to a scratch directory, runs sol2uml,
// and returns the rendered SVG. sourceCode is the verified source as stored on
// the contract record — either a single flattened .sol file or a Solidity
// standard-JSON input ({"sources": {...}}) / Etherscan double-brace wrapper.
func (g *Generator) GenerateSVG(ctx context.Context, contractName, sourceCode string) ([]byte, error) {
	if strings.TrimSpace(sourceCode) == "" {
		return nil, fmt.Errorf("contract has no source code")
	}

	dir, err := os.MkdirTemp("", "sol2uml-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create scratch dir: %w", err)
	}
	defer os.RemoveAll(dir)

	if err := writeSources(dir, contractName, sourceCode); err != nil {
		return nil, err
	}

	outPath := filepath.Join(dir, "diagram.svg")

	ctx, cancel := context.WithTimeout(ctx, g.timeout)
	defer cancel()

	// `sol2uml class <dir> -f svg -o <file>` parses every .sol under <dir>
	// (resolving local imports) and renders a class diagram.
	cmd := exec.CommandContext(ctx, g.path, "class", dir, "-f", "svg", "-o", outPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("UML generation timed out")
		}
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("sol2uml failed: %s", msg)
	}

	svg, err := os.ReadFile(outPath)
	if err != nil {
		return nil, fmt.Errorf("sol2uml produced no output: %w", err)
	}
	if len(svg) == 0 {
		return nil, fmt.Errorf("sol2uml produced an empty diagram")
	}
	return svg, nil
}

// writeSources materialises the verified source into dir as one or more .sol
// files so sol2uml can parse them.
func writeSources(dir, contractName, sourceCode string) error {
	src := strings.TrimSpace(sourceCode)

	// Etherscan wraps standard-JSON in an extra set of braces: {{ ... }}.
	if strings.HasPrefix(src, "{{") && strings.HasSuffix(src, "}}") {
		src = src[1 : len(src)-1]
	}

	// Multi-file: a Solidity standard-JSON input with a "sources" map, or a
	// bare { "path": {"content": "..."} } map. Try to split it into files.
	if strings.HasPrefix(src, "{") {
		if files, ok := parseStandardJSON(src); ok && len(files) > 0 {
			for name, content := range files {
				if err := writeSourceFile(dir, name, content); err != nil {
					return err
				}
			}
			return nil
		}
	}

	// Single flattened source file.
	name := contractName
	if name == "" {
		name = "Contract"
	}
	return writeSourceFile(dir, name+".sol", src)
}

// parseStandardJSON extracts {filename: content} pairs from a Solidity
// standard-JSON input or a plain sources map. Returns ok=false if the blob is
// not a recognisable sources container.
func parseStandardJSON(src string) (map[string]string, bool) {
	var root map[string]json.RawMessage
	if json.Unmarshal([]byte(src), &root) != nil {
		return nil, false
	}

	raw := root
	if s, ok := root["sources"]; ok {
		var inner map[string]json.RawMessage
		if json.Unmarshal(s, &inner) == nil {
			raw = inner
		}
	}

	out := make(map[string]string)
	for name, entry := range raw {
		var obj struct {
			Content string `json:"content"`
		}
		if json.Unmarshal(entry, &obj) == nil && obj.Content != "" {
			out[name] = obj.Content
		}
	}
	return out, len(out) > 0
}

// writeSourceFile writes content to dir/name, creating parent dirs and
// containing the path within dir (defends against ../ in source paths).
func writeSourceFile(dir, name, content string) error {
	clean := filepath.Clean("/" + strings.ReplaceAll(name, "\\", "/"))
	full := filepath.Join(dir, clean)
	if !strings.HasPrefix(full, filepath.Clean(dir)+string(os.PathSeparator)) && full != dir {
		return fmt.Errorf("invalid source path: %s", name)
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return fmt.Errorf("failed to create source dir: %w", err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		return fmt.Errorf("failed to write source file: %w", err)
	}
	return nil
}
