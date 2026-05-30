// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package bindings

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dotandev/glassbox/internal/abi"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// buildTestGenerator returns a Generator pre-loaded with a representative
// mock ContractSpec so tests do not need a real WASM file.
func buildTestGenerator() *Generator {
	spec := &abi.ContractSpec{
		Functions: []xdr.ScSpecFunctionV0{
			{
				Name: "transfer",
				Doc:  "Transfer tokens from one account to another.",
				Inputs: []xdr.ScSpecFunctionInputV0{
					{Name: "from", Type: xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeAddress}},
					{Name: "to", Type: xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeAddress}},
					{Name: "amount", Type: xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeU128}},
				},
				Outputs: []xdr.ScSpecTypeDef{
					{Type: xdr.ScSpecTypeScSpecTypeVoid},
				},
			},
			{
				Name: "balance",
				Doc:  "Return the token balance of an account.",
				Inputs: []xdr.ScSpecFunctionInputV0{
					{Name: "account", Type: xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeAddress}},
				},
				Outputs: []xdr.ScSpecTypeDef{
					{Type: xdr.ScSpecTypeScSpecTypeU128},
				},
			},
		},
		Structs: []xdr.ScSpecUdtStructV0{
			{
				Name: "TokenInfo",
				Fields: []xdr.ScSpecUdtStructFieldV0{
					{Name: "name", Type: xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeString}},
					{Name: "symbol", Type: xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeString}},
					{Name: "decimals", Type: xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeU32}},
				},
			},
		},
		Enums: []xdr.ScSpecUdtEnumV0{
			{
				Name: "Status",
				Cases: []xdr.ScSpecUdtEnumCaseV0{
					{Name: "Active", Value: 0},
					{Name: "Inactive", Value: 1},
				},
			},
		},
		ErrorEnums: []xdr.ScSpecUdtErrorEnumV0{
			{
				Name: "TokenError",
				Cases: []xdr.ScSpecUdtErrorEnumCaseV0{
					{Name: "InsufficientBalance", Value: 1},
					{Name: "Unauthorized", Value: 2},
				},
			},
		},
	}

	return &Generator{
		config: GeneratorConfig{
			PackageName:      "test-contract",
			ContractID:       "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQAHHAGCN4B2",
			Network:          "testnet",
			IncludeDebugMeta: true,
			WasmSourcePath:   "/path/to/contract.wasm",
		},
		spec: spec,
	}
}

// ─── types.ts ─────────────────────────────────────────────────────────────────

func TestGenerateTypes_BaseTypes(t *testing.T) {
	g := buildTestGenerator()
	types := g.generateTypes()

	for _, want := range []string{
		"export type Address = string;",
		"export type Bytes = Uint8Array;",
		"export type SorobanSymbol = string;",
		"export type Result<T, E = Error>",
	} {
		if !strings.Contains(types, want) {
			t.Errorf("missing %q in types.ts", want)
		}
	}
}

func TestGenerateTypes_Struct(t *testing.T) {
	g := buildTestGenerator()
	types := g.generateTypes()
	if !strings.Contains(types, "export interface TokenInfo") {
		t.Error("missing TokenInfo struct")
	}
	if !strings.Contains(types, "name: string;") {
		t.Error("missing name field")
	}
	if !strings.Contains(types, "decimals: number;") {
		t.Error("missing decimals field")
	}
}

func TestGenerateTypes_Enum(t *testing.T) {
	g := buildTestGenerator()
	types := g.generateTypes()
	if !strings.Contains(types, "export enum Status") {
		t.Error("missing Status enum")
	}
	if !strings.Contains(types, "Active = 0,") {
		t.Error("missing Active case")
	}
}

func TestGenerateTypes_ErrorEnum(t *testing.T) {
	g := buildTestGenerator()
	types := g.generateTypes()
	if !strings.Contains(types, "export enum TokenError") {
		t.Error("missing TokenError enum")
	}
	if !strings.Contains(types, "export class TokenErrorError extends Error") {
		t.Error("missing TokenErrorError class")
	}
	if !strings.Contains(types, "public readonly code: TokenError") {
		t.Error("missing typed code field")
	}
}

// ─── metadata.ts ──────────────────────────────────────────────────────────────

func TestGenerateMetadata_Structure(t *testing.T) {
	g := buildTestGenerator()
	meta := g.generateMetadata()

	for _, want := range []string{
		"export interface SourceHint",
		"export interface ParamDescriptor",
		"export interface FunctionMetadata",
		"export const CONTRACT_METADATA",
		"transfer:",
		"balance:",
		`sourcePath: "/path/to/contract.wasm"`,
		`operationIndex: 0`,
		`operationIndex: 1`,
	} {
		if !strings.Contains(meta, want) {
			t.Errorf("missing %q in metadata.ts", want)
		}
	}
}

func TestGenerateMetadata_ParamDescriptors(t *testing.T) {
	g := buildTestGenerator()
	meta := g.generateMetadata()

	// transfer has inputs: from (Address), to (Address), amount (U128)
	if !strings.Contains(meta, `sorobanType: "Address"`) {
		t.Error("missing sorobanType Address in metadata")
	}
	if !strings.Contains(meta, `sorobanType: "U128"`) {
		t.Error("missing sorobanType U128 in metadata")
	}
	if !strings.Contains(meta, `tsType: "bigint"`) {
		t.Error("missing tsType bigint in metadata")
	}
}

// ─── client.ts ────────────────────────────────────────────────────────────────

func TestGenerateClient_Class(t *testing.T) {
	g := buildTestGenerator()
	client := g.generateClient()

	if !strings.Contains(client, "export class TestContractClient") {
		t.Error("missing client class")
	}
	if !strings.Contains(client, "import { CONTRACT_METADATA, FunctionMetadata } from './metadata';") {
		t.Error("missing metadata import")
	}
}

func TestGenerateClient_Methods(t *testing.T) {
	g := buildTestGenerator()
	client := g.generateClient()

	if !strings.Contains(client, "async transfer(") {
		t.Error("missing transfer method")
	}
	if !strings.Contains(client, "async balance(") {
		t.Error("missing balance method")
	}
}

func TestGenerateClient_InputValidation(t *testing.T) {
	g := buildTestGenerator()
	client := g.generateClient()

	// Address params should have null-guard.
	if !strings.Contains(client, "Parameter from must not be null or undefined") {
		t.Error("missing null-guard for 'from' param")
	}
}

func TestGenerateClient_DebugMetadata(t *testing.T) {
	g := buildTestGenerator()
	client := g.generateClient()

	if !strings.Contains(client, "withDebugMetadata?: boolean;") {
		t.Error("missing withDebugMetadata option")
	}
	if !strings.Contains(client, "debugMetadata?: FunctionMetadata;") {
		t.Error("missing debugMetadata in CallResult")
	}
	if !strings.Contains(client, "CONTRACT_METADATA['transfer']") {
		t.Error("missing CONTRACT_METADATA lookup for transfer")
	}
}

func TestGenerateClient_GlassboxIntegration(t *testing.T) {
	g := buildTestGenerator()
	client := g.generateClient()
	if !strings.Contains(client, "ErstSimulator") {
		t.Error("missing ErstSimulator reference")
	}
}

// ─── Glassbox-integration.ts ──────────────────────────────────────────────────

func TestGenerateErstIntegration(t *testing.T) {
	g := buildTestGenerator()
	integration := g.generateErstIntegration()

	for _, want := range []string{
		"export class ErstSimulator",
		"async simulate(",
		"async debugTransaction(",
		"private runGlassbox(",
	} {
		if !strings.Contains(integration, want) {
			t.Errorf("missing %q in Glassbox-integration.ts", want)
		}
	}
}

// ─── index.ts ─────────────────────────────────────────────────────────────────

func TestGenerateIndex(t *testing.T) {
	g := buildTestGenerator()
	index := g.generateIndex()

	for _, want := range []string{
		"export * from './types';",
		"export * from './metadata';",
		"export * from './client';",
		"export * from './Glassbox-integration';",
	} {
		if !strings.Contains(index, want) {
			t.Errorf("missing %q in index.ts", want)
		}
	}
}

// ─── package.json ─────────────────────────────────────────────────────────────

func TestGeneratePackageJSON(t *testing.T) {
	g := buildTestGenerator()
	pkg := g.generatePackageJSON()
	if !strings.Contains(pkg, `"name": "test-contract"`) {
		t.Error("missing package name")
	}
	if !strings.Contains(pkg, "@stellar/stellar-sdk") {
		t.Error("missing stellar-sdk dependency")
	}
}

// ─── README.md ────────────────────────────────────────────────────────────────

func TestGenerateReadme(t *testing.T) {
	g := buildTestGenerator()
	readme := g.generateReadme()

	for _, want := range []string{
		"# test-contract",
		"## Usage",
		"## Debug Metadata",
		"withDebugMetadata: true",
		"## Contract Methods",
		"### `transfer`",
		"### `balance`",
	} {
		if !strings.Contains(readme, want) {
			t.Errorf("missing %q in README.md", want)
		}
	}
}

// ─── Full Generate() pipeline ─────────────────────────────────────────────────

func TestGenerateAllFiles(t *testing.T) {
	g := buildTestGenerator()
	// Bypass WASM extraction by setting spec directly.
	files, err := generateFromSpec(g)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := map[string]bool{
		"types.ts":               false,
		"metadata.ts":            false,
		"client.ts":              false,
		"Glassbox-integration.ts": false,
		"index.ts":               false,
		"package.json":           false,
		"README.md":              false,
	}
	for _, f := range files {
		if _, ok := expected[f.Path]; ok {
			expected[f.Path] = true
		}
		if len(f.Content) == 0 {
			t.Errorf("file %s has empty content", f.Path)
		}
	}
	for name, found := range expected {
		if !found {
			t.Errorf("expected file %s was not generated", name)
		}
	}
}

// generateFromSpec calls the internal generation methods directly, bypassing
// WASM extraction (which requires a real WASM binary).
func generateFromSpec(g *Generator) ([]GeneratedFile, error) {
	return []GeneratedFile{
		{Path: "types.ts", Content: g.generateTypes()},
		{Path: "metadata.ts", Content: g.generateMetadata()},
		{Path: "client.ts", Content: g.generateClient()},
		{Path: "Glassbox-integration.ts", Content: g.generateErstIntegration()},
		{Path: "index.ts", Content: g.generateIndex()},
		{Path: "package.json", Content: g.generatePackageJSON()},
		{Path: "README.md", Content: g.generateReadme()},
	}, nil
}

// ─── End-to-end with real WASM ────────────────────────────────────────────────

func TestGenerateBindingsEndToEnd(t *testing.T) {
	testWasmPath := os.Getenv("TEST_WASM_PATH")
	if testWasmPath == "" {
		t.Skip("TEST_WASM_PATH not set, skipping end-to-end test")
	}

	wasmBytes, err := os.ReadFile(testWasmPath)
	if err != nil {
		t.Fatalf("failed to read test WASM: %v", err)
	}

	tmpDir := t.TempDir()
	config := GeneratorConfig{
		WasmBytes:        wasmBytes,
		OutputDir:        tmpDir,
		PackageName:      "test-contract",
		ContractID:       "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQAHHAGCN4B2",
		Network:          "testnet",
		IncludeDebugMeta: true,
		WasmSourcePath:   testWasmPath,
	}

	generator := NewGenerator(config)
	files, err := generator.Generate()
	if err != nil {
		t.Fatalf("failed to generate bindings: %v", err)
	}

	expectedFiles := []string{
		"types.ts", "metadata.ts", "client.ts",
		"Glassbox-integration.ts", "index.ts", "package.json", "README.md",
	}

	for _, expectedFile := range expectedFiles {
		found := false
		for _, file := range files {
			if file.Path == expectedFile {
				found = true
				if len(file.Content) == 0 {
					t.Errorf("file %s has no content", expectedFile)
				}
				fullPath := filepath.Join(tmpDir, file.Path)
				if err := os.WriteFile(fullPath, []byte(file.Content), 0644); err != nil {
					t.Errorf("failed to write %s: %v", expectedFile, err)
				}
				break
			}
		}
		if !found {
			t.Errorf("expected file %s not generated", expectedFile)
		}
	}
}
