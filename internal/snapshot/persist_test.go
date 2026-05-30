// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package snapshot

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// helpers

func makeTestMeta(txHash, network string) *ReplayMetadata {
	return &ReplayMetadata{
		GlassboxVersion: "v1.0.0",
		TxHash:          txHash,
		Network:         network,
		EnvelopeXdr:     "envelopeXDR",
		ResultMetaXdr:   "resultMetaXDR",
	}
}

func makeTestSnap(entries map[string]string) *Snapshot {
	return FromMap(entries)
}

// --- SavePersisted / LoadPersisted ---

func TestSaveAndLoadPersisted_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "snap.json")

	meta := makeTestMeta("abc123", "testnet")
	snap := makeTestSnap(map[string]string{"key-1": "val-1", "key-2": "val-2"})

	if err := SavePersisted(path, meta, snap); err != nil {
		t.Fatalf("SavePersisted failed: %v", err)
	}

	loaded, err := LoadPersisted(path)
	if err != nil {
		t.Fatalf("LoadPersisted failed: %v", err)
	}

	if loaded.Metadata.TxHash != "abc123" {
		t.Errorf("expected TxHash abc123, got %s", loaded.Metadata.TxHash)
	}
	if loaded.Metadata.Network != "testnet" {
		t.Errorf("expected Network testnet, got %s", loaded.Metadata.Network)
	}
	if loaded.Metadata.SchemaVersion != PersistSchemaVersion {
		t.Errorf("expected schema version %d, got %d", PersistSchemaVersion, loaded.Metadata.SchemaVersion)
	}

	m := loaded.Snapshot.ToMap()
	if m["key-1"] != "val-1" {
		t.Errorf("snapshot data corrupted: %v", m)
	}
}

func TestSaveAndLoadPersisted_SetsSchemaVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "snap.json")

	meta := makeTestMeta("tx", "testnet")
	meta.SchemaVersion = 0 // should be overwritten

	if err := SavePersisted(path, meta, makeTestSnap(nil)); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadPersisted(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Metadata.SchemaVersion != PersistSchemaVersion {
		t.Errorf("expected schema version %d, got %d", PersistSchemaVersion, loaded.Metadata.SchemaVersion)
	}
}

func TestSaveAndLoadPersisted_SetsSavedAt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "snap.json")

	meta := makeTestMeta("tx", "testnet")
	before := time.Now()

	if err := SavePersisted(path, meta, makeTestSnap(nil)); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadPersisted(path)
	if err != nil {
		t.Fatal(err)
	}

	if loaded.Metadata.SavedAt.Before(before) {
		t.Errorf("SavedAt %v is before save time %v", loaded.Metadata.SavedAt, before)
	}
}

func TestSaveAndLoadPersisted_NilSnapshot(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "snap.json")

	meta := makeTestMeta("tx", "testnet")
	if err := SavePersisted(path, meta, nil); err != nil {
		t.Fatalf("SavePersisted with nil snapshot failed: %v", err)
	}

	loaded, err := LoadPersisted(path)
	if err != nil {
		t.Fatalf("LoadPersisted failed: %v", err)
	}
	if loaded.Snapshot == nil {
		t.Error("expected non-nil snapshot after loading nil input")
	}
}

func TestSavePersisted_NilMetadata(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "snap.json")
	err := SavePersisted(path, nil, makeTestSnap(nil))
	if err == nil {
		t.Error("expected error for nil metadata")
	}
}

func TestSavePersisted_CreatesParentDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "deep", "snap.json")

	meta := makeTestMeta("tx", "testnet")
	if err := SavePersisted(path, meta, makeTestSnap(nil)); err != nil {
		t.Fatalf("SavePersisted failed to create parent dirs: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected file to exist: %v", err)
	}
}

func TestSavePersisted_AtomicWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "snap.json")

	meta := makeTestMeta("tx", "testnet")
	if err := SavePersisted(path, meta, makeTestSnap(nil)); err != nil {
		t.Fatal(err)
	}

	// Temp file must not remain.
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Error("expected temp file to be cleaned up")
	}
}

func TestLoadPersisted_NotFound(t *testing.T) {
	_, err := LoadPersisted("/nonexistent/snap.json")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoadPersisted_InvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(path, []byte("{bad json"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadPersisted(path)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestLoadPersisted_WrongSchemaVersion(t *testing.T) {
	ps := &PersistedSnapshot{
		Metadata: &ReplayMetadata{SchemaVersion: 999, TxHash: "tx", Network: "testnet"},
		Snapshot: makeTestSnap(nil),
	}
	data, _ := json.MarshalIndent(ps, "", "  ")
	path := filepath.Join(t.TempDir(), "snap.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadPersisted(path)
	if err == nil {
		t.Error("expected error for wrong schema version")
	}
	if !strings.Contains(err.Error(), "schema version") {
		t.Errorf("expected schema version error, got: %v", err)
	}
}

func TestLoadPersisted_MissingMetadata(t *testing.T) {
	ps := &PersistedSnapshot{Snapshot: makeTestSnap(nil)}
	data, _ := json.MarshalIndent(ps, "", "  ")
	path := filepath.Join(t.TempDir(), "snap.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadPersisted(path)
	if err == nil {
		t.Error("expected error for missing metadata")
	}
}

func TestLoadPersisted_MissingSnapshot(t *testing.T) {
	ps := &PersistedSnapshot{
		Metadata: &ReplayMetadata{SchemaVersion: PersistSchemaVersion, TxHash: "tx", Network: "testnet"},
	}
	data, _ := json.MarshalIndent(ps, "", "  ")
	path := filepath.Join(t.TempDir(), "snap.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadPersisted(path)
	if err == nil {
		t.Error("expected error for missing snapshot")
	}
}

// --- Validate ---

func TestValidate_Clean(t *testing.T) {
	meta := makeTestMeta("abc123", "testnet")
	snap := makeTestSnap(map[string]string{"k": "v"})

	dir := t.TempDir()
	path := filepath.Join(dir, "snap.json")
	if err := SavePersisted(path, meta, snap); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadPersisted(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := loaded.Validate("abc123", "testnet"); err != nil {
		t.Errorf("expected valid snapshot, got: %v", err)
	}
}

func TestValidate_TxHashMismatch(t *testing.T) {
	meta := makeTestMeta("abc123", "testnet")
	snap := makeTestSnap(map[string]string{"k": "v"})

	dir := t.TempDir()
	path := filepath.Join(dir, "snap.json")
	if err := SavePersisted(path, meta, snap); err != nil {
		t.Fatal(err)
	}

	loaded, _ := LoadPersisted(path)
	err := loaded.Validate("different-hash", "testnet")
	if err == nil {
		t.Error("expected error for tx hash mismatch")
	}
	if !strings.Contains(err.Error(), "tx hash") {
		t.Errorf("expected tx hash error, got: %v", err)
	}
}

func TestValidate_NetworkMismatch(t *testing.T) {
	meta := makeTestMeta("abc123", "testnet")
	snap := makeTestSnap(map[string]string{"k": "v"})

	dir := t.TempDir()
	path := filepath.Join(dir, "snap.json")
	if err := SavePersisted(path, meta, snap); err != nil {
		t.Fatal(err)
	}

	loaded, _ := LoadPersisted(path)
	err := loaded.Validate("abc123", "mainnet")
	if err == nil {
		t.Error("expected error for network mismatch")
	}
	if !strings.Contains(err.Error(), "network") {
		t.Errorf("expected network error, got: %v", err)
	}
}

func TestValidate_FingerprintMismatch(t *testing.T) {
	meta := makeTestMeta("abc123", "testnet")
	snap := makeTestSnap(map[string]string{"k": "v"})

	dir := t.TempDir()
	path := filepath.Join(dir, "snap.json")
	if err := SavePersisted(path, meta, snap); err != nil {
		t.Fatal(err)
	}

	loaded, _ := LoadPersisted(path)
	// Tamper with the fingerprint.
	loaded.Snapshot.Fingerprint = "deadbeef"

	err := loaded.Validate("abc123", "testnet")
	if err == nil {
		t.Error("expected error for fingerprint mismatch")
	}
	if !strings.Contains(err.Error(), "fingerprint") {
		t.Errorf("expected fingerprint error, got: %v", err)
	}
}

func TestValidate_EmptyExpectedValues(t *testing.T) {
	meta := makeTestMeta("abc123", "testnet")
	snap := makeTestSnap(map[string]string{"k": "v"})

	dir := t.TempDir()
	path := filepath.Join(dir, "snap.json")
	if err := SavePersisted(path, meta, snap); err != nil {
		t.Fatal(err)
	}

	loaded, _ := LoadPersisted(path)
	// Empty expected values skip identity checks.
	if err := loaded.Validate("", ""); err != nil {
		t.Errorf("expected no error for empty expected values, got: %v", err)
	}
}

// --- IsStale ---

func TestIsStale_FreshParams(t *testing.T) {
	meta := makeTestMeta("tx", "testnet")
	params := map[string]string{"network": "testnet", "tx": "abc"}
	meta.ParamFingerprint = BuildParamFingerprint(params)

	ps := &PersistedSnapshot{Metadata: meta, Snapshot: makeTestSnap(nil)}
	if ps.IsStale(params, "") {
		t.Error("expected snapshot to be fresh")
	}
}

func TestIsStale_StaleParams(t *testing.T) {
	meta := makeTestMeta("tx", "testnet")
	meta.ParamFingerprint = BuildParamFingerprint(map[string]string{"network": "testnet"})

	ps := &PersistedSnapshot{Metadata: meta, Snapshot: makeTestSnap(nil)}
	if !ps.IsStale(map[string]string{"network": "mainnet"}, "") {
		t.Error("expected snapshot to be stale")
	}
}

func TestIsStale_StaleSourceHash(t *testing.T) {
	meta := makeTestMeta("tx", "testnet")
	meta.SourceHash = "oldhash"

	ps := &PersistedSnapshot{Metadata: meta, Snapshot: makeTestSnap(nil)}
	if !ps.IsStale(nil, "newhash") {
		t.Error("expected snapshot to be stale due to source hash change")
	}
}

func TestIsStale_EmptyFingerprintAlwaysFresh(t *testing.T) {
	meta := makeTestMeta("tx", "testnet")
	// No fingerprint set.
	ps := &PersistedSnapshot{Metadata: meta, Snapshot: makeTestSnap(nil)}
	if ps.IsStale(map[string]string{"any": "params"}, "anyhash") {
		t.Error("expected empty fingerprint to be treated as fresh")
	}
}

// --- DefaultSnapshotPath ---

func TestDefaultSnapshotPath_Format(t *testing.T) {
	path := DefaultSnapshotPath("/cache", "testnet", "abcdef1234567890xyz")
	if !strings.Contains(path, "testnet") {
		t.Errorf("expected path to contain network, got: %s", path)
	}
	if !strings.Contains(path, "abcdef1234567890") {
		t.Errorf("expected path to contain short hash, got: %s", path)
	}
	if !strings.HasSuffix(path, ".snap.json") {
		t.Errorf("expected .snap.json suffix, got: %s", path)
	}
}

func TestDefaultSnapshotPath_ShortHash(t *testing.T) {
	longHash := strings.Repeat("a", 64)
	path := DefaultSnapshotPath("/cache", "mainnet", longHash)
	// Only first 16 chars should appear in the filename.
	base := filepath.Base(path)
	if len(base) > len("aaaaaaaaaaaaaaaa.snap.json")+5 {
		t.Errorf("expected short hash in filename, got: %s", base)
	}
}

func TestDefaultSnapshotPath_EmptyCacheDir(t *testing.T) {
	path := DefaultSnapshotPath("", "testnet", "abc123")
	if path == "" {
		t.Error("expected non-empty path when cacheDir is empty")
	}
	if !strings.Contains(path, ".glassbox") {
		t.Errorf("expected default .glassbox path, got: %s", path)
	}
}

// --- HashWasmSource ---

func TestHashWasmSource_Empty(t *testing.T) {
	if got := HashWasmSource(nil); got != "" {
		t.Errorf("expected empty hash for nil, got %s", got)
	}
	if got := HashWasmSource([]byte{}); got != "" {
		t.Errorf("expected empty hash for empty bytes, got %s", got)
	}
}

func TestHashWasmSource_Deterministic(t *testing.T) {
	data := []byte("wasm binary content")
	h1 := HashWasmSource(data)
	h2 := HashWasmSource(data)
	if h1 != h2 {
		t.Errorf("hashes differ: %s vs %s", h1, h2)
	}
}

func TestHashWasmSource_ChangesOnMutation(t *testing.T) {
	h1 := HashWasmSource([]byte("version1"))
	h2 := HashWasmSource([]byte("version2"))
	if h1 == h2 {
		t.Error("expected different hashes for different content")
	}
}

// --- BuildParamFingerprint ---

func TestBuildParamFingerprint_Deterministic(t *testing.T) {
	params := map[string]string{"network": "testnet", "tx": "abc123", "protocol": "21"}
	f1 := BuildParamFingerprint(params)
	f2 := BuildParamFingerprint(params)
	if f1 != f2 {
		t.Errorf("fingerprints differ: %s vs %s", f1, f2)
	}
}

func TestBuildParamFingerprint_OrderIndependent(t *testing.T) {
	// Same params in different insertion order must produce the same fingerprint.
	p1 := map[string]string{"a": "1", "b": "2", "c": "3"}
	p2 := map[string]string{"c": "3", "a": "1", "b": "2"}
	if BuildParamFingerprint(p1) != BuildParamFingerprint(p2) {
		t.Error("expected order-independent fingerprint")
	}
}

func TestBuildParamFingerprint_ChangesOnDifferentValues(t *testing.T) {
	p1 := map[string]string{"network": "testnet"}
	p2 := map[string]string{"network": "mainnet"}
	if BuildParamFingerprint(p1) == BuildParamFingerprint(p2) {
		t.Error("expected different fingerprints for different values")
	}
}

func TestBuildParamFingerprint_EmptyMap(t *testing.T) {
	if got := BuildParamFingerprint(nil); got != "" {
		t.Errorf("expected empty fingerprint for nil map, got %s", got)
	}
}

// --- Full invalidation scenario ---

func TestInvalidation_SourceHashChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "snap.json")

	wasmV1 := []byte("wasm-version-1")
	meta := makeTestMeta("tx", "testnet")
	meta.SourceHash = HashWasmSource(wasmV1)
	meta.ParamFingerprint = BuildParamFingerprint(map[string]string{"network": "testnet"})

	if err := SavePersisted(path, meta, makeTestSnap(map[string]string{"k": "v"})); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadPersisted(path)
	if err != nil {
		t.Fatal(err)
	}

	// Same WASM: not stale.
	if loaded.IsStale(map[string]string{"network": "testnet"}, HashWasmSource(wasmV1)) {
		t.Error("expected snapshot to be fresh with same WASM")
	}

	// Different WASM: stale.
	wasmV2 := []byte("wasm-version-2")
	if !loaded.IsStale(map[string]string{"network": "testnet"}, HashWasmSource(wasmV2)) {
		t.Error("expected snapshot to be stale after WASM change")
	}
}

func TestInvalidation_NetworkChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "snap.json")

	params := map[string]string{"network": "testnet", "tx": "abc"}
	meta := makeTestMeta("abc", "testnet")
	meta.ParamFingerprint = BuildParamFingerprint(params)

	if err := SavePersisted(path, meta, makeTestSnap(nil)); err != nil {
		t.Fatal(err)
	}

	loaded, _ := LoadPersisted(path)

	// Same params: fresh.
	if loaded.IsStale(params, "") {
		t.Error("expected fresh with same params")
	}

	// Different network: stale.
	newParams := map[string]string{"network": "mainnet", "tx": "abc"}
	if !loaded.IsStale(newParams, "") {
		t.Error("expected stale after network change")
	}
}
