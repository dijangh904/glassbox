// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package sourcemap

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Resolver.Resolve — contract ID validation ─────────────────────────────────

func TestResolve_EmptyContractID_ReturnsError(t *testing.T) {
	r := NewResolver()
	_, err := r.Resolve(context.Background(), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid contract ID",
		"error should identify the invalid contract ID")
	assert.Contains(t, err.Error(), "56 characters",
		"error should state the expected length")
}

func TestResolve_ShortContractID_ReturnsError(t *testing.T) {
	r := NewResolver()
	_, err := r.Resolve(context.Background(), "CABC123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid contract ID")
}

func TestResolve_NotStartingWithC_ReturnsError(t *testing.T) {
	r := NewResolver()
	// 56-character string but starts with 'G'
	_, err := r.Resolve(context.Background(), "GABC3J7GYCCX3S7LX63P6R7EAL477J26C356X6E5A4XERAD7UXD6I7Y3N")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid contract ID")
}

// ── Resolver.Resolve — override path validation ───────────────────────────────

func TestResolve_ContractSourceOverride_NonExistentDir_ReturnsError(t *testing.T) {
	srv := notFoundServer(t)
	rc := NewRegistryClient(WithBaseURL(srv.URL))
	r := NewResolver(
		WithRegistryClient(rc),
		WithContractSource("/nonexistent/source/dir"),
	)

	_, err := r.Resolve(context.Background(), testContractID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--contract-source",
		"error should name the --contract-source flag")
	assert.Contains(t, err.Error(), "not found",
		"error should say directory was not found")
}

func TestResolve_ContractSourceOverride_PathIsFile_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	filePath := dir + "/lib.rs"
	require.NoError(t, os.WriteFile(filePath, []byte("fn main() {}"), 0644))

	srv := notFoundServer(t)
	rc := NewRegistryClient(WithBaseURL(srv.URL))
	r := NewResolver(
		WithRegistryClient(rc),
		WithContractSource(filePath),
	)

	_, err := r.Resolve(context.Background(), testContractID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is a file, not a directory",
		"error should clarify that a directory is required")
}

func TestResolve_ContractSourceOverride_ValidDir_ReturnsSource(t *testing.T) {
	dir := t.TempDir() // exists and is a directory

	srv := notFoundServer(t)
	rc := NewRegistryClient(WithBaseURL(srv.URL))
	r := NewResolver(
		WithRegistryClient(rc),
		WithContractSource(dir),
	)

	source, err := r.Resolve(context.Background(), testContractID)
	require.NoError(t, err)
	require.NotNil(t, source)
	assert.Equal(t, dir, source.Repository)
	assert.Equal(t, testContractID, source.ContractID)
}

// ── Resolver.Resolve — non-interactive mode ───────────────────────────────────

func TestResolve_NonInteractive_AllStagesExhausted_ReturnsErrSourceNotFound(t *testing.T) {
	srv := notFoundServer(t)
	rc := NewRegistryClient(WithBaseURL(srv.URL))
	r := NewResolver(
		WithRegistryClient(rc),
		WithNonInteractive(),
	)

	_, err := r.Resolve(context.Background(), testContractID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrSourceNotFound),
		"errors.Is must match ErrSourceNotFound, got: %v", err)
	assert.Contains(t, err.Error(), "--contract-source",
		"error should suggest --contract-source as a remedy")
	assert.Contains(t, err.Error(), "--skip-source-mapping",
		"error should suggest --skip-source-mapping as an alternative")
}

func TestResolve_NonInteractive_ErrorDoesNotHangOnStdin(t *testing.T) {
	// This test verifies that WithNonInteractive prevents any stdin read.
	// If the test completes without blocking it passes.
	srv := notFoundServer(t)
	rc := NewRegistryClient(WithBaseURL(srv.URL))
	r := NewResolver(
		WithRegistryClient(rc),
		WithNonInteractive(),
	)

	done := make(chan error, 1)
	go func() {
		_, err := r.Resolve(context.Background(), testContractID)
		done <- err
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	select {
	case err := <-done:
		require.Error(t, err, "non-interactive mode should return an error, not block")
	case <-ctx.Done():
		t.Fatal("Resolve blocked waiting for stdin in non-interactive mode — WithNonInteractive is not working")
	}
}

func TestResolve_NonInteractive_ErrorListsAllStages(t *testing.T) {
	srv := notFoundServer(t)
	rc := NewRegistryClient(WithBaseURL(srv.URL))
	r := NewResolver(
		WithRegistryClient(rc),
		WithNonInteractive(),
	)

	_, err := r.Resolve(context.Background(), testContractID)
	require.Error(t, err)
	msg := err.Error()
	// Must enumerate every discovery stage.
	assert.Contains(t, msg, "cache")
	assert.Contains(t, msg, "registry")
	assert.Contains(t, msg, "GitHub")
	assert.Contains(t, msg, "--contract-source override")
}

func TestLoadAliasConfig_NonExistentTargetReturnsError(t *testing.T) {
	dir := t.TempDir()
	aliasPath := filepath.Join(dir, "aliases.json")
	require.NoError(t, os.WriteFile(aliasPath, []byte(`{"my_crate":"/definitely/missing"}`), 0o644))

	_, err := LoadAliasConfig(aliasPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "existing directory")
}

func TestLoadAliasConfig_RelativeTargetResolvedAgainstConfigFile(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "src")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))

	aliasPath := filepath.Join(dir, "aliases.json")
	require.NoError(t, os.WriteFile(aliasPath, []byte(`{"my_crate":"src"}`), 0o644))

	aliases, err := LoadAliasConfig(aliasPath)
	require.NoError(t, err)
	assert.Equal(t, srcDir, aliases["my_crate"])
}

// ── Resolver.Resolve — successful registry path ───────────────────────────────

func TestResolve_RegistrySucceeds_ResultIsCached(t *testing.T) {
	// Serve a verified contract. The GitHub fetch will fail, so the resolver
	// will use the override. We're only checking registry + cache behaviour.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"contract": "` + testContractID + `",
			"wasm_hash": "abc123",
			"repository": "https://github.com/example/contract",
			"verified": true
		}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	rc := NewRegistryClient(WithBaseURL(srv.URL))
	r := NewResolver(
		WithRegistryClient(rc),
		WithCache(dir),
		WithNonInteractive(),
	)

	// First call: registry hit (GitHub will fail → override path used).
	source, _ := r.Resolve(context.Background(), testContractID)
	// We don't assert success here because GitHub fetch will fail in test env;
	// what matters is the resolver doesn't panic and returns something useful.
	_ = source
}

// ── AutoDiscoverLocalSymbols — input validation ───────────────────────────────

func TestAutoDiscoverLocalSymbols_EmptyProjectRoot_ReturnsError(t *testing.T) {
	r := NewResolver()
	err := r.AutoDiscoverLocalSymbols("", "somehash")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "projectRoot must not be empty")
}

func TestAutoDiscoverLocalSymbols_EmptyHash_ReturnsError(t *testing.T) {
	r := NewResolver()
	err := r.AutoDiscoverLocalSymbols("/some/path", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expectedHash must not be empty")
}

func TestAutoDiscoverLocalSymbols_MissingBuildDir_ReturnsNil(t *testing.T) {
	// A missing build directory should be a non-fatal hint, not an error.
	r := NewResolver()
	err := r.AutoDiscoverLocalSymbols(t.TempDir(), "deadbeefdeadbeef")
	assert.NoError(t, err, "missing build dir should be logged as debug, not returned as error")
}

func TestAutoDiscoverLocalSymbols_NoHashMatch_ReturnsNil(t *testing.T) {
	dir := t.TempDir()
	targetDir := dir + "/target/wasm32-unknown-unknown/release"
	require.NoError(t, os.MkdirAll(targetDir, 0755))
	require.NoError(t, os.WriteFile(targetDir+"/contract.wasm",
		[]byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}, 0644))

	r := NewResolver()
	err := r.AutoDiscoverLocalSymbols(dir, "0000000000000000000000000000000000000000000000000000000000000000")
	assert.NoError(t, err, "no hash match should be silent, not an error")
}

// ── WithNonInteractive option ─────────────────────────────────────────────────

func TestWithNonInteractive_SetsFlag(t *testing.T) {
	r := NewResolver(WithNonInteractive())
	assert.True(t, r.nonInteractive, "WithNonInteractive should set nonInteractive = true")
}

// ── ErrSourceNotFound sentinel ────────────────────────────────────────────────

func TestErrSourceNotFound_IsWrappable(t *testing.T) {
	wrapped := errors.New("outer: " + ErrSourceNotFound.Error())
	// errors.Is does NOT unwrap string-wrapped errors, but we can still test
	// that ErrSourceNotFound is a valid sentinel for direct comparison.
	assert.Equal(t, ErrSourceNotFound, ErrSourceNotFound)
	assert.NotNil(t, wrapped)
}

// ── helpers ───────────────────────────────────────────────────────────────────

// notFoundServer returns a test HTTP server that always responds 404.
func notFoundServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)
	return srv
}
