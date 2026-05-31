// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package bindings

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/dotandev/glassbox/internal/abi"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// buildJSONSpecBytes returns a minimal JSON ABI document for a token contract.
func buildJSONSpecBytes(t *testing.T) []byte {
	t.Helper()
	type jsonField struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	type jsonFunction struct {
		Name    string      `json:"name"`
		Doc     string      `json:"doc,omitempty"`
		Inputs  []jsonField `json:"inputs,omitempty"`
		Outputs []string    `json:"outputs,omitempty"`
	}
	type jsonEnum struct {
		Name  string `json:"name"`
		Cases []struct {
			Name  string `json:"name"`
			Value uint32 `json:"value"`
		} `json:"cases,omitempty"`
	}
	type spec struct {
		Functions []jsonFunction `json:"functions"`
		Enums     []jsonEnum     `json:"enums,omitempty"`
	}

	s := spec{
		Functions: []jsonFunction{
			{
				Name: "mint",
				Doc:  "Mint new tokens.",
				Inputs: []jsonField{
					{Name: "to", Type: "Address"},
					{Name: "amount", Type: "U128"},
				},
				Outputs: []string{"Void"},
			},
		},
		Enums: []jsonEnum{
			{
				Name: "Role",
				Cases: []struct {
					Name  string `json:"name"`
					Value uint32 `json:"value"`
				}{
					{Name: "Admin", Value: 0},
					{Name: "User", Value: 1},
				},
			},
		},
	}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("failed to marshal test JSON spec: %v", err)
	}
	return data
}

// buildXDRSpecBytes returns raw XDR-encoded ScSpecEntry bytes for a simple contract.
func buildXDRSpecBytes(t *testing.T) []byte {
	t.Helper()
	fn := xdr.ScSpecFunctionV0{
		Name: "burn",
		Inputs: []xdr.ScSpecFunctionInputV0{
			{Name: "from", Type: xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeAddress}},
			{Name: "amount", Type: xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeU128}},
		},
		Outputs: []xdr.ScSpecTypeDef{{Type: xdr.ScSpecTypeScSpecTypeVoid}},
	}
	entry := xdr.ScSpecEntry{Kind: xdr.ScSpecEntryKindScSpecEntryFunctionV0, FunctionV0: &fn}
	data, err := entry.MarshalBinary()
	if err != nil {
		t.Fatalf("failed to marshal XDR spec: %v", err)
	}
	return data
}

// ─── Generate() from JSON spec ────────────────────────────────────────────────

func TestGenerate_FromJSONSpec(t *testing.T) {
	jsonBytes := buildJSONSpecBytes(t)

	config := GeneratorConfig{
		SpecBytes:   jsonBytes,
		SpecFormat:  abi.ImportFormatJSON,
		PackageName: "token-contract",
		Network:     "testnet",
	}
	g := NewGenerator(config)
	files, err := g.Generate()
	if err != nil {
		t.Fatalf("Generate() from JSON spec failed: %v", err)
	}

	fileMap := make(map[string]string)
	for _, f := range files {
		fileMap[f.Path] = f.Content
	}

	// All expected files should be present.
	for _, name := range []string{"types.ts", "metadata.ts", "client.ts", "Glassbox-integration.ts", "index.ts", "package.json", "README.md"} {
		if _, ok := fileMap[name]; !ok {
			t.Errorf("expected file %s not generated", name)
		}
	}

	// The generated client should contain the mint method.
	if !strings.Contains(fileMap["client.ts"], "async mint(") {
		t.Error("client.ts should contain mint method")
	}
	// The generated types should contain the Role enum.
	if !strings.Contains(fileMap["types.ts"], "export enum Role") {
		t.Error("types.ts should contain Role enum")
	}
	// Metadata should reference mint.
	if !strings.Contains(fileMap["metadata.ts"], "mint:") {
		t.Error("metadata.ts should reference mint function")
	}
}

func TestGenerate_FromJSONSpec_AutoDetect(t *testing.T) {
	jsonBytes := buildJSONSpecBytes(t)

	// SpecFormat is empty – auto-detection should kick in.
	config := GeneratorConfig{
		SpecBytes:   jsonBytes,
		SpecFormat:  "", // auto-detect
		PackageName: "token-contract",
		Network:     "testnet",
	}
	g := NewGenerator(config)
	files, err := g.Generate()
	if err != nil {
		t.Fatalf("Generate() with auto-detect failed: %v", err)
	}
	if len(files) == 0 {
		t.Error("expected files to be generated")
	}
}

// ─── Generate() from XDR spec ─────────────────────────────────────────────────

func TestGenerate_FromXDRSpec(t *testing.T) {
	xdrBytes := buildXDRSpecBytes(t)

	config := GeneratorConfig{
		SpecBytes:   xdrBytes,
		SpecFormat:  abi.ImportFormatXDR,
		PackageName: "burn-contract",
		Network:     "testnet",
	}
	g := NewGenerator(config)
	files, err := g.Generate()
	if err != nil {
		t.Fatalf("Generate() from XDR spec failed: %v", err)
	}

	fileMap := make(map[string]string)
	for _, f := range files {
		fileMap[f.Path] = f.Content
	}

	if !strings.Contains(fileMap["client.ts"], "async burn(") {
		t.Error("client.ts should contain burn method")
	}
}

// ─── Generate() error cases ───────────────────────────────────────────────────

func TestGenerate_NoSource_ReturnsError(t *testing.T) {
	config := GeneratorConfig{
		PackageName: "empty",
		Network:     "testnet",
	}
	g := NewGenerator(config)
	_, err := g.Generate()
	if err == nil {
		t.Error("expected error when no spec source is provided")
	}
}

func TestGenerate_InvalidJSONSpec_ReturnsError(t *testing.T) {
	config := GeneratorConfig{
		SpecBytes:   []byte(`{not valid json`),
		SpecFormat:  abi.ImportFormatJSON,
		PackageName: "bad",
		Network:     "testnet",
	}
	g := NewGenerator(config)
	_, err := g.Generate()
	if err == nil {
		t.Error("expected error for invalid JSON spec")
	}
}

func TestGenerate_InvalidXDRSpec_ReturnsError(t *testing.T) {
	config := GeneratorConfig{
		SpecBytes:   []byte{0xFF, 0xFE, 0xFD},
		SpecFormat:  abi.ImportFormatXDR,
		PackageName: "bad",
		Network:     "testnet",
	}
	g := NewGenerator(config)
	_, err := g.Generate()
	if err == nil {
		t.Error("expected error for invalid XDR spec")
	}
}

func TestGenerate_UnsupportedFormat_ReturnsError(t *testing.T) {
	config := GeneratorConfig{
		SpecBytes:   []byte(`some data`),
		SpecFormat:  abi.ImportFormat("yaml"),
		PackageName: "bad",
		Network:     "testnet",
	}
	g := NewGenerator(config)
	_, err := g.Generate()
	if err == nil {
		t.Error("expected error for unsupported spec format")
	}
}
