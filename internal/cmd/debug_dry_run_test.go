// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// makeDebugCmdForTest creates a minimal cobra.Command that wraps runDebugDryRun
// with the provided flag values pre-set so we can test the dry-run path
// without exercising the full cobra flag-parsing machinery.
func makeDebugCmdForTest() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "debug",
		RunE: func(cmd *cobra.Command, args []string) error { return nil },
	}
	// Mirror the flags registered by the real debug init().
	cmd.Flags().StringVarP(&networkFlag, "network", "n", "mainnet", "")
	cmd.Flags().StringVar(&compareNetworkFlag, "compare-network", "", "")
	cmd.Flags().StringVar(&rpcURLFlag, "rpc-url", "", "")
	cmd.Flags().StringVar(&rpcTokenFlag, "rpc-token", "", "")
	return cmd
}

// TestRunDebugDryRun_InvalidHash ensures that an invalid transaction hash is
// reported as a failure with an actionable message.
func TestRunDebugDryRun_InvalidHash(t *testing.T) {
	t.Cleanup(func() {
		networkFlag = "mainnet"
		compareNetworkFlag = ""
		rpcURLFlag = ""
		rpcTokenFlag = ""
	})

	networkFlag = "testnet"

	var out, errBuf bytes.Buffer
	cmd := makeDebugCmdForTest()
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)

	err := runDebugDryRun(cmd, "not-a-valid-hash")
	if err == nil {
		t.Fatal("expected error for invalid transaction hash, got nil")
	}

	stderr := errBuf.String()
	if !strings.Contains(stderr, "[FAIL]") {
		t.Errorf("expected [FAIL] tag in stderr, got:\n%s", stderr)
	}
}

// TestRunDebugDryRun_InvalidNetwork ensures that an invalid --network value
// produces a clear failure message.
func TestRunDebugDryRun_InvalidNetwork(t *testing.T) {
	t.Cleanup(func() {
		networkFlag = "mainnet"
		compareNetworkFlag = ""
		rpcURLFlag = ""
		rpcTokenFlag = ""
	})

	networkFlag = "devnet" // invalid

	var out, errBuf bytes.Buffer
	cmd := makeDebugCmdForTest()
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)

	validHash := "5c0a1234567890abcdef1234567890abcdef1234567890abcdef1234567890ab"
	err := runDebugDryRun(cmd, validHash)
	if err == nil {
		t.Fatal("expected error for invalid network, got nil")
	}

	stderr := errBuf.String()
	if !strings.Contains(stderr, "[FAIL]") {
		t.Errorf("expected [FAIL] tag in stderr for invalid network, got:\n%s", stderr)
	}
	if !strings.Contains(stderr, "devnet") {
		t.Errorf("expected network name %q in stderr, got:\n%s", "devnet", stderr)
	}
}

// TestRunDebugDryRun_InvalidCompareNetwork ensures that an invalid
// --compare-network value is captured and reported correctly.
func TestRunDebugDryRun_InvalidCompareNetwork(t *testing.T) {
	t.Cleanup(func() {
		networkFlag = "mainnet"
		compareNetworkFlag = ""
		rpcURLFlag = ""
		rpcTokenFlag = ""
	})

	networkFlag = "testnet"
	compareNetworkFlag = "badnet"

	var out, errBuf bytes.Buffer
	cmd := makeDebugCmdForTest()
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)

	validHash := "5c0a1234567890abcdef1234567890abcdef1234567890abcdef1234567890ab"
	err := runDebugDryRun(cmd, validHash)
	if err == nil {
		t.Fatal("expected error for invalid compare-network, got nil")
	}

	stderr := errBuf.String()
	if !strings.Contains(stderr, "[FAIL]") {
		t.Errorf("expected [FAIL] tag in stderr for invalid compare-network, got:\n%s", stderr)
	}
	if !strings.Contains(stderr, "badnet") {
		t.Errorf("expected compare-network name %q in stderr, got:\n%s", "badnet", stderr)
	}
}

// TestRunDebugDryRun_OutputContainsHeader verifies that a successful hash
// and network validation writes a dry-run header to stdout. The RPC check and
// simulator check will likely fail in unit test context (no network, no binary),
// but we verify the output structure is correct.
func TestRunDebugDryRun_OutputContainsHeader(t *testing.T) {
	t.Cleanup(func() {
		networkFlag = "mainnet"
		compareNetworkFlag = ""
		rpcURLFlag = ""
		rpcTokenFlag = ""
	})

	networkFlag = "testnet"

	var out, errBuf bytes.Buffer
	cmd := makeDebugCmdForTest()
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)

	validHash := "5c0a1234567890abcdef1234567890abcdef1234567890abcdef1234567890ab"
	// Error is expected here (no actual RPC / simulator in unit tests).
	_ = runDebugDryRun(cmd, validHash)

	stdout := out.String()
	if !strings.Contains(stdout, "Dry-run: validating debug parameters") {
		t.Errorf("expected dry-run header in stdout, got:\n%s", stdout)
	}
	// Hash and network checks should pass and appear as [OK].
	if !strings.Contains(stdout, "[OK]") {
		t.Errorf("expected at least one [OK] line in stdout, got:\n%s", stdout)
	}
}

// TestRunDebugDryRun_ErrorSummaryFormat verifies the error summary line format:
// it should say "Dry-run FAILED" and enumerate failures.
func TestRunDebugDryRun_ErrorSummaryFormat(t *testing.T) {
	t.Cleanup(func() {
		networkFlag = "mainnet"
		compareNetworkFlag = ""
		rpcURLFlag = ""
		rpcTokenFlag = ""
	})

	networkFlag = "devnet"  // will fail validation
	compareNetworkFlag = "" // not set

	var out, errBuf bytes.Buffer
	cmd := makeDebugCmdForTest()
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)

	err := runDebugDryRun(cmd, "too-short")
	if err == nil {
		t.Fatal("expected errors for short hash + invalid network, got nil")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "dry-run validation failed") {
		t.Errorf("error message should mention 'dry-run validation failed', got: %q", errMsg)
	}

	stderr := errBuf.String()
	if !strings.Contains(stderr, "Dry-run FAILED") {
		t.Errorf("expected 'Dry-run FAILED' summary in stderr, got:\n%s", stderr)
	}
}

// TestDebugPreRunE_HotReloadRequiresWasm verifies the PreRunE hook rejects
// --hot-reload without --wasm.
func TestDebugPreRunE_HotReloadRequiresWasm(t *testing.T) {
	t.Cleanup(func() {
		hotReloadFlag = false
		wasmPath = ""
	})

	hotReloadFlag = true
	wasmPath = "" // not set

	err := debugCmd.PreRunE(debugCmd, []string{})
	if err == nil {
		t.Fatal("expected error when --hot-reload is set without --wasm")
	}
	if !strings.Contains(err.Error(), "--hot-reload requires --wasm") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// TestDebugPreRunE_BothLocalInputFilesRejected verifies the PreRunE hook
// rejects specifying both --xdr-file and --json-file simultaneously.
func TestDebugPreRunE_BothLocalInputFilesRejected(t *testing.T) {
	t.Cleanup(func() {
		xdrFileFlag = ""
		jsonFileFlag = ""
		hotReloadFlag = false
		wasmPath = ""
	})

	xdrFileFlag = "tx.xdr"
	jsonFileFlag = "tx.json"

	err := debugCmd.PreRunE(debugCmd, []string{})
	if err == nil {
		t.Fatal("expected error when both --xdr-file and --json-file are provided")
	}
	if !strings.Contains(err.Error(), "only one of") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// TestDebugPreRunE_MissingHashWithoutLocalMode verifies that when no transaction
// hash is given and no local mode flags are set, the PreRunE returns a clear error.
func TestDebugPreRunE_MissingHashWithoutLocalMode(t *testing.T) {
	t.Cleanup(func() {
		hotReloadFlag = false
		wasmPath = ""
		xdrFileFlag = ""
		jsonFileFlag = ""
		demoMode = false
		loadSnapshotsFlag = ""
	})

	// No local mode, no hash.
	err := debugCmd.PreRunE(debugCmd, []string{})
	if err == nil {
		t.Fatal("expected error when no hash and no local mode is set")
	}
	if !strings.Contains(err.Error(), "transaction hash is required") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// TestDebugPreRunE_OpIndexBelowMinusOneRejected ensures --op < -1 is rejected.
func TestDebugPreRunE_OpIndexBelowMinusOneRejected(t *testing.T) {
	t.Cleanup(func() {
		opIndexFlag = -1
		wasmPath = ""
		demoMode = false
		hotReloadFlag = false
		xdrFileFlag = ""
		jsonFileFlag = ""
		loadSnapshotsFlag = ""
	})

	opIndexFlag = -5 // invalid: only -1 (omit) or >= 0 are valid
	validHash := "5c0a1234567890abcdef1234567890abcdef1234567890abcdef1234567890ab"
	// Use a valid network
	networkFlag = "testnet"

	err := debugCmd.PreRunE(debugCmd, []string{validHash})
	if err == nil {
		t.Fatal("expected error when --op < -1")
	}
	if !strings.Contains(err.Error(), "--op must be a non-negative integer") {
		t.Errorf("unexpected error message: %v", err)
	}
}
