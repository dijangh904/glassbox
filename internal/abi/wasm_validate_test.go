// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package abi

import (
	"strings"
	"testing"
)

func TestValidateWasmMagic_Valid(t *testing.T) {
	// Minimal valid WASM: magic + version
	data := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
	if err := ValidateWasmMagic(data, "contract.wasm"); err != nil {
		t.Errorf("expected nil for valid WASM magic, got: %v", err)
	}
}

func TestValidateWasmMagic_TooShort(t *testing.T) {
	data := []byte{0x00, 0x61}
	err := ValidateWasmMagic(data, "tiny.wasm")
	if err == nil {
		t.Fatal("expected error for too-short file")
	}
	if !strings.Contains(err.Error(), "tiny.wasm") {
		t.Errorf("error should contain path, got: %v", err)
	}
	if !strings.Contains(err.Error(), "too small") {
		t.Errorf("error should say 'too small', got: %v", err)
	}
}

func TestValidateWasmMagic_BadMagic(t *testing.T) {
	// ELF magic bytes
	data := []byte{0x7f, 0x45, 0x4c, 0x46, 0x00, 0x00, 0x00, 0x00}
	err := ValidateWasmMagic(data, "/build/contract.wasm")
	if err == nil {
		t.Fatal("expected error for bad magic bytes")
	}
	if !strings.Contains(err.Error(), "/build/contract.wasm") {
		t.Errorf("error should include the file path, got: %v", err)
	}
	if !strings.Contains(err.Error(), "not a valid WASM binary") {
		t.Errorf("error should say 'not a valid WASM binary', got: %v", err)
	}
	// Must mention the expected magic pattern so the user knows what to look for.
	if !strings.Contains(err.Error(), `\0asm`) {
		t.Errorf("error should mention expected magic '\\0asm', got: %v", err)
	}
}

func TestValidateWasmMagic_SourceFile(t *testing.T) {
	// Rust source file content
	data := append([]byte("fn main() { }"), make([]byte, 4)...)
	err := ValidateWasmMagic(data, "lib.rs")
	if err == nil {
		t.Fatal("expected error for source file")
	}
	if !strings.Contains(err.Error(), "not a valid WASM binary") {
		t.Errorf("error should say 'not a valid WASM binary', got: %v", err)
	}
	// Should hint to compile first.
	if !strings.Contains(err.Error(), "source file") {
		t.Errorf("error should mention 'source file' as a likely cause, got: %v", err)
	}
}

func TestValidateWasmMagic_PathIncludedInError(t *testing.T) {
	data := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	err := ValidateWasmMagic(data, "/some/specific/path.wasm")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "/some/specific/path.wasm") {
		t.Errorf("error must contain the file path, got: %v", err)
	}
}
