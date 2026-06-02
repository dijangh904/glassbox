// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"testing"

	"github.com/dotandev/glassbox/internal/simulator"
)

func TestApplyDebugSimulationOptions_SkipSourceMapping(t *testing.T) {
	skipSourceMappingFlag = true
	contractSourceFlag = "/tmp/src"
	defer func() {
		skipSourceMappingFlag = false
		contractSourceFlag = ""
	}()

	req := &simulator.SimulationRequest{}
	applyDebugSimulationOptions(req)
	if !req.SkipSourceMapping {
		t.Fatal("expected SkipSourceMapping to be set")
	}
	if req.ContractSourcePath == nil || *req.ContractSourcePath != "/tmp/src" {
		t.Fatalf("ContractSourcePath = %v", req.ContractSourcePath)
	}
}

func TestNewLocalWasmSimulationRequest_SkipSourceMapping(t *testing.T) {
	skipSourceMappingFlag = true
	wasmPath = "contract.wasm"
	wasmBase64 = "YWJj"
	defer func() {
		skipSourceMappingFlag = false
		wasmPath = ""
		wasmBase64 = ""
	}()

	req := newLocalWasmSimulationRequest(false)
	if req == nil || !req.SkipSourceMapping {
		t.Fatal("expected skip source mapping on local replay request")
	}
}
