// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateNetwork(t *testing.T) {
	tests := []struct {
		network string
		wantErr bool
	}{
		{"testnet", false},
		{"mainnet", false},
		{"futurenet", false},
		{"standalone", false},
		{"public", false},
		{"TESTNET", false}, // case-insensitive
		{"", false},        // optional
		{"devnet", true},
		{"invalid", true},
		{"prod", true},
	}
	for _, tt := range tests {
		err := validateNetwork(tt.network)
		if (err != nil) != tt.wantErr {
			t.Errorf("validateNetwork(%q) error=%v, wantErr=%v", tt.network, err, tt.wantErr)
		}
		if err != nil && tt.wantErr {
			// Error message must include the invalid value.
			if !containsStr(err.Error(), tt.network) {
				t.Errorf("validateNetwork(%q) error %q should mention the invalid value", tt.network, err.Error())
			}
		}
	}
}

func TestValidateFilePath(t *testing.T) {
	// Existing file — should pass.
	tmp := filepath.Join(t.TempDir(), "test.wasm")
	if err := os.WriteFile(tmp, []byte{0x00}, 0600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if err := validateFilePath("wasm", tmp); err != nil {
		t.Errorf("validateFilePath existing file: unexpected error %v", err)
	}

	// Non-existent file — should fail with descriptive message.
	err := validateFilePath("wasm", "/nonexistent/path/contract.wasm")
	if err == nil {
		t.Error("validateFilePath non-existent: expected error")
	} else if !containsStr(err.Error(), "/nonexistent/path/contract.wasm") {
		t.Errorf("error %q should mention the path", err.Error())
	}

	// Empty path — should pass (optional).
	if err := validateFilePath("wasm", ""); err != nil {
		t.Errorf("validateFilePath empty: unexpected error %v", err)
	}
}

func TestValidatePositiveInt(t *testing.T) {
	if err := validatePositiveInt("timeout", 30); err != nil {
		t.Errorf("positive int: unexpected error %v", err)
	}
	if err := validatePositiveInt("timeout", 0); err != nil {
		t.Errorf("zero int: unexpected error %v", err)
	}
	err := validatePositiveInt("timeout", -1)
	if err == nil {
		t.Error("negative int: expected error")
	} else if !containsStr(err.Error(), "timeout") {
		t.Errorf("error %q should mention the flag name", err.Error())
	}
}

func TestValidateMutuallyExclusive(t *testing.T) {
	// Neither set — OK.
	if err := validateMutuallyExclusive(map[string]bool{}, "payload", "payload-file"); err != nil {
		t.Errorf("neither set: unexpected error %v", err)
	}

	// One set — OK.
	if err := validateMutuallyExclusive(map[string]bool{"payload": true}, "payload", "payload-file"); err != nil {
		t.Errorf("one set: unexpected error %v", err)
	}

	// Both set — error.
	err := validateMutuallyExclusive(
		map[string]bool{"payload": true, "payload-file": true},
		"payload", "payload-file",
	)
	if err == nil {
		t.Error("both set: expected error")
	} else {
		if !containsStr(err.Error(), "--payload") || !containsStr(err.Error(), "--payload-file") {
			t.Errorf("error %q should mention both flags", err.Error())
		}
	}
}

func TestValidateGenerateBindingsArgs(t *testing.T) {
	tmp := t.TempDir()
	wasmFile := filepath.Join(tmp, "contract.wasm")
	if err := os.WriteFile(wasmFile, []byte{0x00}, 0600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	tests := []struct {
		name      string
		network   string
		wasmPath  string
		outputDir string
		wantErr   bool
	}{
		{"valid testnet + existing wasm", "testnet", wasmFile, tmp, false},
		{"valid empty args", "", "", "", false},
		{"invalid network", "devnet", wasmFile, tmp, true},
		{"missing wasm file", "testnet", "/no/such/file.wasm", tmp, true},
		{"output is a file not dir", "testnet", wasmFile, wasmFile, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateGenerateBindingsArgs(tt.network, tt.wasmPath, tt.outputDir)
			if (err != nil) != tt.wantErr {
				t.Errorf("error=%v, wantErr=%v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateAuditSignArgs(t *testing.T) {
	tests := []struct {
		name        string
		payload     string
		payloadFile string
		provider    string
		wantErr     bool
	}{
		{"valid payload only", `{"x":1}`, "", "software", false},
		{"valid payload-file only", "", "/some/file.json", "pkcs11", false},
		{"both payload and file", `{"x":1}`, "/some/file.json", "", true},
		{"invalid provider", "", "", "hsm-custom", true},
		{"empty provider ok", `{"x":1}`, "", "", false},
		{"case-insensitive provider", `{"x":1}`, "", "PKCS11", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAuditSignArgs(tt.payload, tt.payloadFile, tt.provider)
			if (err != nil) != tt.wantErr {
				t.Errorf("error=%v, wantErr=%v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateNetwork_ErrorMentionsSuggestion(t *testing.T) {
	err := validateNetwork("prod")
	if err == nil {
		t.Fatal("expected error")
	}
	// Error should mention valid options.
	if !containsStr(err.Error(), "testnet") {
		t.Errorf("error %q should suggest valid networks", err.Error())
	}
}

// containsStr is a simple substring helper to avoid importing strings in tests.
func containsStr(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && func() bool {
		for i := 0; i <= len(s)-len(sub); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	}())
}

// ─── validateGenerateBindingsFlags ────────────────────────────────────────────

func TestValidateGenerateBindingsFlags_Runtime(t *testing.T) {
	tests := []struct {
		runtime string
		wantErr bool
	}{
		{"node", false},
		{"browser", false},
		{"universal", false},
		{"NODE", false},    // case-insensitive
		{"BROWSER", false}, // case-insensitive
		{"electron", true},
		{"deno", true},
		{"", false}, // empty = default
	}
	for _, tt := range tests {
		t.Run(tt.runtime, func(t *testing.T) {
			err := validateGenerateBindingsFlags("testnet", nil, "", tt.runtime, "", "")
			if (err != nil) != tt.wantErr {
				t.Errorf("runtime=%q error=%v, wantErr=%v", tt.runtime, err, tt.wantErr)
			}
		})
	}
}

func TestValidateGenerateBindingsFlags_SpecFormat(t *testing.T) {
	tests := []struct {
		format  string
		wantErr bool
	}{
		{"json", false},
		{"xdr", false},
		{"JSON", false}, // case-insensitive
		{"XDR", false},  // case-insensitive
		{"yaml", true},
		{"toml", true},
		{"", false}, // empty = auto-detect
	}
	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			err := validateGenerateBindingsFlags("testnet", nil, "", "", "", tt.format)
			if (err != nil) != tt.wantErr {
				t.Errorf("format=%q error=%v, wantErr=%v", tt.format, err, tt.wantErr)
			}
		})
	}
}

func TestValidateGenerateBindingsFlags_MutuallyExclusive(t *testing.T) {
	tmp := t.TempDir()

	// Create a dummy wasm file and spec file.
	wasmFile := tmp + "/contract.wasm"
	specFile := tmp + "/contract.json"
	if err := os.WriteFile(wasmFile, []byte{0x00}, 0600); err != nil {
		t.Fatalf("setup wasm: %v", err)
	}
	if err := os.WriteFile(specFile, []byte(`{}`), 0600); err != nil {
		t.Fatalf("setup spec: %v", err)
	}

	// Both wasm and spec-file provided → error.
	err := validateGenerateBindingsFlags("testnet", []string{wasmFile}, "", "", specFile, "")
	if err == nil {
		t.Error("expected error when both wasm-file and --spec-file are provided")
	}
	if !containsStr(err.Error(), "mutually exclusive") {
		t.Errorf("error %q should mention mutually exclusive", err.Error())
	}

	// Only wasm → OK.
	if err := validateGenerateBindingsFlags("testnet", []string{wasmFile}, "", "", "", ""); err != nil {
		t.Errorf("wasm only: unexpected error %v", err)
	}

	// Only spec-file → OK.
	if err := validateGenerateBindingsFlags("testnet", nil, "", "", specFile, ""); err != nil {
		t.Errorf("spec-file only: unexpected error %v", err)
	}
}

func TestValidateGenerateBindingsFlags_MissingSpecFile(t *testing.T) {
	err := validateGenerateBindingsFlags("testnet", nil, "", "", "/no/such/file.json", "")
	if err == nil {
		t.Error("expected error for missing spec file")
	}
}
