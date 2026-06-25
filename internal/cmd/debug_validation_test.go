// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"strings"
	"testing"
)

// TestDebugPreRunE_CompareNetworkSameAsNetwork verifies that specifying
// --compare-network equal to --network is rejected with a clear error.
func TestDebugPreRunE_CompareNetworkSameAsNetwork(t *testing.T) {
	t.Cleanup(func() {
		networkFlag = "mainnet"
		compareNetworkFlag = ""
		hotReloadFlag = false
		wasmPath = ""
		xdrFileFlag = ""
		jsonFileFlag = ""
		demoMode = false
		loadSnapshotsFlag = ""
		opIndexFlag = -1
		watchFlag = false
		watchTimeoutFlag = 30
		traceVerbosityFlag = "normal"
		debugFormatFlag = "text"
	})

	networkFlag = "testnet"
	compareNetworkFlag = "testnet" // same as --network — should be rejected

	validHash := "5c0a1234567890abcdef1234567890abcdef1234567890abcdef1234567890ab"
	err := debugCmd.PreRunE(debugCmd, []string{validHash})
	if err == nil {
		t.Fatal("expected error when --compare-network equals --network")
	}
	if !strings.Contains(err.Error(), "must be different") {
		t.Errorf("expected 'must be different' in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "testnet") {
		t.Errorf("expected network name in error, got: %v", err)
	}
}

// TestDebugPreRunE_CompareNetworkDiffFromNetwork verifies that valid distinct
// networks are accepted without error.
func TestDebugPreRunE_CompareNetworkDiffFromNetwork(t *testing.T) {
	t.Cleanup(func() {
		networkFlag = "mainnet"
		compareNetworkFlag = ""
		hotReloadFlag = false
		wasmPath = ""
		xdrFileFlag = ""
		jsonFileFlag = ""
		demoMode = false
		loadSnapshotsFlag = ""
		opIndexFlag = -1
		watchFlag = false
		watchTimeoutFlag = 30
		traceVerbosityFlag = "normal"
		debugFormatFlag = "text"
	})

	networkFlag = "testnet"
	compareNetworkFlag = "mainnet" // different — should be valid

	validHash := "5c0a1234567890abcdef1234567890abcdef1234567890abcdef1234567890ab"
	err := debugCmd.PreRunE(debugCmd, []string{validHash})
	// We may get other errors (RPC etc.) but NOT a compare-network error.
	if err != nil && strings.Contains(err.Error(), "must differ") {
		t.Errorf("should not reject distinct --compare-network, got: %v", err)
	}
}

// TestDebugPreRunE_WatchTimeoutZeroRejected verifies that --watch-timeout=0 is
// rejected when --watch is set.
func TestDebugPreRunE_WatchTimeoutZeroRejected(t *testing.T) {
	t.Cleanup(func() {
		networkFlag = "mainnet"
		compareNetworkFlag = ""
		hotReloadFlag = false
		wasmPath = ""
		xdrFileFlag = ""
		jsonFileFlag = ""
		demoMode = false
		loadSnapshotsFlag = ""
		opIndexFlag = -1
		watchFlag = false
		watchTimeoutFlag = 30
		traceVerbosityFlag = "normal"
		debugFormatFlag = "text"
	})

	networkFlag = "testnet"
	watchFlag = true
	watchTimeoutFlag = 0 // invalid

	validHash := "5c0a1234567890abcdef1234567890abcdef1234567890abcdef1234567890ab"
	err := debugCmd.PreRunE(debugCmd, []string{validHash})
	if err == nil {
		t.Fatal("expected error for --watch-timeout=0")
	}
	if !strings.Contains(err.Error(), "watch-timeout") {
		t.Errorf("expected 'watch-timeout' in error, got: %v", err)
	}
}

// TestDebugPreRunE_WatchTimeoutNegativeRejected verifies that --watch-timeout=-1
// is rejected when --watch is set.
func TestDebugPreRunE_WatchTimeoutNegativeRejected(t *testing.T) {
	t.Cleanup(func() {
		networkFlag = "mainnet"
		compareNetworkFlag = ""
		hotReloadFlag = false
		wasmPath = ""
		xdrFileFlag = ""
		jsonFileFlag = ""
		demoMode = false
		loadSnapshotsFlag = ""
		opIndexFlag = -1
		watchFlag = false
		watchTimeoutFlag = 30
		traceVerbosityFlag = "normal"
		debugFormatFlag = "text"
	})

	networkFlag = "testnet"
	watchFlag = true
	watchTimeoutFlag = -5

	validHash := "5c0a1234567890abcdef1234567890abcdef1234567890abcdef1234567890ab"
	err := debugCmd.PreRunE(debugCmd, []string{validHash})
	if err == nil {
		t.Fatal("expected error for negative --watch-timeout")
	}
	if !strings.Contains(err.Error(), "watch-timeout") {
		t.Errorf("expected 'watch-timeout' in error, got: %v", err)
	}
}

// TestDebugPreRunE_WatchTimeoutPositiveAccepted verifies a valid positive
// --watch-timeout is accepted.
func TestDebugPreRunE_WatchTimeoutPositiveAccepted(t *testing.T) {
	t.Cleanup(func() {
		networkFlag = "mainnet"
		compareNetworkFlag = ""
		hotReloadFlag = false
		wasmPath = ""
		xdrFileFlag = ""
		jsonFileFlag = ""
		demoMode = false
		loadSnapshotsFlag = ""
		opIndexFlag = -1
		watchFlag = false
		watchTimeoutFlag = 30
		traceVerbosityFlag = "normal"
		debugFormatFlag = "text"
	})

	networkFlag = "testnet"
	watchFlag = true
	watchTimeoutFlag = 60

	validHash := "5c0a1234567890abcdef1234567890abcdef1234567890abcdef1234567890ab"
	err := debugCmd.PreRunE(debugCmd, []string{validHash})
	// May fail for other reasons (e.g. op-index check path), but NOT for watch-timeout.
	if err != nil && strings.Contains(err.Error(), "watch-timeout") {
		t.Errorf("should not reject valid --watch-timeout=60, got: %v", err)
	}
}

// TestDebugPreRunE_InvalidTraceVerbosityRejected verifies that an unknown
// --trace-verbosity value is rejected at parse time.
func TestDebugPreRunE_InvalidTraceVerbosityRejected(t *testing.T) {
	t.Cleanup(func() {
		networkFlag = "mainnet"
		compareNetworkFlag = ""
		hotReloadFlag = false
		wasmPath = ""
		xdrFileFlag = ""
		jsonFileFlag = ""
		demoMode = false
		loadSnapshotsFlag = ""
		opIndexFlag = -1
		watchFlag = false
		watchTimeoutFlag = 30
		traceVerbosityFlag = "normal"
		debugFormatFlag = "text"
	})

	networkFlag = "testnet"
	traceVerbosityFlag = "ultra" // invalid

	validHash := "5c0a1234567890abcdef1234567890abcdef1234567890abcdef1234567890ab"
	err := debugCmd.PreRunE(debugCmd, []string{validHash})
	if err == nil {
		t.Fatal("expected error for invalid --trace-verbosity")
	}
	if !strings.Contains(err.Error(), "trace-verbosity") {
		t.Errorf("expected 'trace-verbosity' in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "ultra") {
		t.Errorf("expected invalid value 'ultra' echoed in error, got: %v", err)
	}
}

// TestDebugPreRunE_ValidTraceVerbosityAccepted verifies that valid
// --trace-verbosity values are accepted.
func TestDebugPreRunE_ValidTraceVerbosityAccepted(t *testing.T) {
	for _, v := range []string{"summary", "normal", "verbose"} {
		v := v
		t.Run(v, func(t *testing.T) {
			t.Cleanup(func() {
				networkFlag = "mainnet"
				compareNetworkFlag = ""
				hotReloadFlag = false
				wasmPath = ""
				xdrFileFlag = ""
				jsonFileFlag = ""
				demoMode = false
				loadSnapshotsFlag = ""
				opIndexFlag = -1
				watchFlag = false
				watchTimeoutFlag = 30
				traceVerbosityFlag = "normal"
				debugFormatFlag = "text"
			})

			networkFlag = "testnet"
			traceVerbosityFlag = v

			validHash := "5c0a1234567890abcdef1234567890abcdef1234567890abcdef1234567890ab"
			err := debugCmd.PreRunE(debugCmd, []string{validHash})
			if err != nil && strings.Contains(err.Error(), "trace-verbosity") {
				t.Errorf("should not reject valid --trace-verbosity=%s, got: %v", v, err)
			}
		})
	}
}

// TestDebugPreRunE_InvalidFormatRejected verifies that an unknown --format
// value is rejected at parse time with a clear error.
func TestDebugPreRunE_InvalidFormatRejected(t *testing.T) {
	t.Cleanup(func() {
		networkFlag = "mainnet"
		compareNetworkFlag = ""
		hotReloadFlag = false
		wasmPath = ""
		xdrFileFlag = ""
		jsonFileFlag = ""
		demoMode = false
		loadSnapshotsFlag = ""
		opIndexFlag = -1
		watchFlag = false
		watchTimeoutFlag = 30
		traceVerbosityFlag = "normal"
		debugFormatFlag = "text"
	})

	networkFlag = "testnet"
	debugFormatFlag = "yaml" // invalid

	validHash := "5c0a1234567890abcdef1234567890abcdef1234567890abcdef1234567890ab"
	err := debugCmd.PreRunE(debugCmd, []string{validHash})
	if err == nil {
		t.Fatal("expected error for invalid --format")
	}
	if !strings.Contains(err.Error(), "format") {
		t.Errorf("expected 'format' in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "yaml") {
		t.Errorf("expected invalid value 'yaml' echoed in error, got: %v", err)
	}
}

// TestDebugPreRunE_ValidFormatsAccepted verifies that valid --format values
// pass without a format-related error.
func TestDebugPreRunE_ValidFormatsAccepted(t *testing.T) {
	for _, f := range []string{"text", "json"} {
		f := f
		t.Run(f, func(t *testing.T) {
			t.Cleanup(func() {
				networkFlag = "mainnet"
				compareNetworkFlag = ""
				hotReloadFlag = false
				wasmPath = ""
				xdrFileFlag = ""
				jsonFileFlag = ""
				demoMode = false
				loadSnapshotsFlag = ""
				opIndexFlag = -1
				watchFlag = false
				watchTimeoutFlag = 30
				traceVerbosityFlag = "normal"
				debugFormatFlag = "text"
			})

			networkFlag = "testnet"
			debugFormatFlag = f

			validHash := "5c0a1234567890abcdef1234567890abcdef1234567890abcdef1234567890ab"
			err := debugCmd.PreRunE(debugCmd, []string{validHash})
			if err != nil && strings.Contains(err.Error(), "format") {
				t.Errorf("should not reject valid --format=%s, got: %v", f, err)
			}
		})
	}
}
