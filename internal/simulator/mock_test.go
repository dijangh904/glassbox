// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package simulator

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// validBase64 returns a valid base64-encoded string from s.
func validBase64(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

// ── LoadLedgerOverrideManifest ────────────────────────────────────────────────

func TestLoadLedgerOverrideManifest_FileNotFound(t *testing.T) {
	_, err := LoadLedgerOverrideManifest("/nonexistent/manifest.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !strings.Contains(err.Error(), "--mock-ledger-manifest") {
		t.Errorf("error should mention --mock-ledger-manifest flag, got: %v", err)
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should say 'not found', got: %v", err)
	}
	// Must include a remediation hint.
	if !strings.Contains(err.Error(), "Ensure the path") {
		t.Errorf("error should include a remediation hint, got: %v", err)
	}
}

func TestLoadLedgerOverrideManifest_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte("{bad json}"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadLedgerOverrideManifest(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "--mock-ledger-manifest") {
		t.Errorf("error should mention --mock-ledger-manifest, got: %v", err)
	}
	if !strings.Contains(err.Error(), "JSON") {
		t.Errorf("error should mention JSON, got: %v", err)
	}
	if !strings.Contains(err.Error(), "ledger_entries") {
		t.Errorf("error should describe expected format, got: %v", err)
	}
}

func TestLoadLedgerOverrideManifest_EmptyValue(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")
	content := `{"ledger_entries": {"myKey": ""}}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadLedgerOverrideManifest(path)
	if err == nil {
		t.Fatal("expected error for empty ledger entry value")
	}
	if !strings.Contains(err.Error(), "empty value") {
		t.Errorf("error should mention 'empty value', got: %v", err)
	}
	if !strings.Contains(err.Error(), "myKey") {
		t.Errorf("error should include the offending key, got: %v", err)
	}
}

func TestLoadLedgerOverrideManifest_InvalidBase64Value(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")
	content := `{"ledger_entries": {"myKey": "not!!valid!!base64"}}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadLedgerOverrideManifest(path)
	if err == nil {
		t.Fatal("expected error for invalid base64 value")
	}
	if !strings.Contains(err.Error(), "invalid base64") {
		t.Errorf("error should mention 'invalid base64', got: %v", err)
	}
	if !strings.Contains(err.Error(), "myKey") {
		t.Errorf("error should include the offending key, got: %v", err)
	}
}

func TestLoadLedgerOverrideManifest_Success(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")
	v := validBase64("xdr-payload")
	content := `{"ledger_entries": {"keyA": "` + v + `"}}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	entries, err := LoadLedgerOverrideManifest(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entries["keyA"] != v {
		t.Errorf("expected keyA=%q, got %q", v, entries["keyA"])
	}
}

func TestLoadLedgerOverrideManifest_EmptyLedgerEntries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")
	if err := os.WriteFile(path, []byte(`{"ledger_entries": {}}`), 0644); err != nil {
		t.Fatal(err)
	}
	entries, err := LoadLedgerOverrideManifest(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestLoadLedgerOverrideManifest_NullLedgerEntries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")
	if err := os.WriteFile(path, []byte(`{"ledger_entries": null}`), 0644); err != nil {
		t.Fatal(err)
	}
	entries, err := LoadLedgerOverrideManifest(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for null, got %d", len(entries))
	}
}

// ── ParseLedgerOverrideFlags ──────────────────────────────────────────────────

func TestParseLedgerOverrideFlags_InvalidFormat(t *testing.T) {
	_, err := ParseLedgerOverrideFlags([]string{"nokeyvalue"})
	if err == nil {
		t.Fatal("expected error for missing colon separator")
	}
	if !strings.Contains(err.Error(), "--mock-ledger-entry") {
		t.Errorf("error should mention --mock-ledger-entry, got: %v", err)
	}
	if !strings.Contains(err.Error(), "key:value") {
		t.Errorf("error should show expected format key:value, got: %v", err)
	}
}

func TestParseLedgerOverrideFlags_EmptyKey(t *testing.T) {
	_, err := ParseLedgerOverrideFlags([]string{":somevalue"})
	if err == nil {
		t.Fatal("expected error for empty key")
	}
	if !strings.Contains(err.Error(), "--mock-ledger-entry") {
		t.Errorf("error should mention --mock-ledger-entry, got: %v", err)
	}
}

func TestParseLedgerOverrideFlags_EmptyValue(t *testing.T) {
	_, err := ParseLedgerOverrideFlags([]string{"somekey:"})
	if err == nil {
		t.Fatal("expected error for empty value")
	}
	if !strings.Contains(err.Error(), "empty value") {
		t.Errorf("error should mention 'empty value', got: %v", err)
	}
}

func TestParseLedgerOverrideFlags_InvalidBase64Value(t *testing.T) {
	_, err := ParseLedgerOverrideFlags([]string{"somekey:not!!base64"})
	if err == nil {
		t.Fatal("expected error for invalid base64 value")
	}
	if !strings.Contains(err.Error(), "invalid base64") {
		t.Errorf("error should mention 'invalid base64', got: %v", err)
	}
}

func TestParseLedgerOverrideFlags_Success(t *testing.T) {
	v := validBase64("xdr-data")
	overrides, err := ParseLedgerOverrideFlags([]string{"key1:" + v})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if overrides["key1"] != v {
		t.Errorf("expected key1=%q, got %q", v, overrides["key1"])
	}
}

func TestParseLedgerOverrideFlags_MultipleEntries(t *testing.T) {
	v1 := validBase64("entry-one")
	v2 := validBase64("entry-two")
	overrides, err := ParseLedgerOverrideFlags([]string{"k1:" + v1, "k2:" + v2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if overrides["k1"] != v1 || overrides["k2"] != v2 {
		t.Errorf("unexpected overrides: %v", overrides)
	}
}

func TestParseLedgerOverrideFlags_Empty(t *testing.T) {
	overrides, err := ParseLedgerOverrideFlags([]string{})
	if err != nil {
		t.Fatalf("unexpected error for empty input: %v", err)
	}
	if len(overrides) != 0 {
		t.Errorf("expected empty map, got %v", overrides)
	}
}

// ── MergeLedgerOverrides ──────────────────────────────────────────────────────

func TestMergeLedgerOverrides_NilBase(t *testing.T) {
	result := MergeLedgerOverrides(nil, map[string]string{"k": "v"})
	if result["k"] != "v" {
		t.Errorf("expected k=v in result, got %v", result)
	}
}

func TestMergeLedgerOverrides_EmptyOverrides(t *testing.T) {
	base := map[string]string{"existing": "val"}
	result := MergeLedgerOverrides(base, map[string]string{})
	if result["existing"] != "val" {
		t.Errorf("base entry should be preserved, got %v", result)
	}
}

func TestMergeLedgerOverrides_OverrideWins(t *testing.T) {
	base := map[string]string{"k": "old"}
	result := MergeLedgerOverrides(base, map[string]string{"k": "new"})
	if result["k"] != "new" {
		t.Errorf("override should win, got %v", result)
	}
}
