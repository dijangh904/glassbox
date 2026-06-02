// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package simulator

import "testing"

func TestValidateSandboxPolicyRequiresMemoryLimit(t *testing.T) {
	v := NewValidator(false)
	req := &SimulationRequest{
		EnvelopeXdr:           "AAAA",
		ResultMetaXdr:         "AAAA",
		SandboxMode:           true,
		AllowedHostFunctions:  []string{"storage_get"},
	}

	err := v.ValidateRequest(req)
	if err == nil {
		t.Fatal("expected sandbox memory limit validation error")
	}
	if got := err.(*ValidationError).Code; got != "ERR_SANDBOX_MEMORY_LIMIT_REQUIRED" {
		t.Fatalf("code = %s, want ERR_SANDBOX_MEMORY_LIMIT_REQUIRED", got)
	}
}

func TestValidateSandboxPolicyRequiresHostAllowlist(t *testing.T) {
	v := NewValidator(false)
	limit := uint64(64 * 1024 * 1024)
	req := &SimulationRequest{
		EnvelopeXdr:   "AAAA",
		ResultMetaXdr: "AAAA",
		SandboxMode:   true,
		MemoryLimit:   &limit,
	}

	err := v.ValidateRequest(req)
	if err == nil {
		t.Fatal("expected sandbox host allowlist validation error")
	}
	if got := err.(*ValidationError).Code; got != "ERR_SANDBOX_HOST_ALLOWLIST_REQUIRED" {
		t.Fatalf("code = %s, want ERR_SANDBOX_HOST_ALLOWLIST_REQUIRED", got)
	}
}

func TestApplySandboxConfig(t *testing.T) {
	limit := uint64(1024)
	req := &SimulationRequest{
		SandboxMode:          true,
		MemoryLimit:          &limit,
		AllowedHostFunctions: []string{"storage_get", "storage_put"},
	}

	r := &Runner{}
	r.applySandboxConfig(req)

	if req.CustomAuthCfg["sandbox_mode"] != true {
		t.Fatalf("sandbox_mode not propagated: %#v", req.CustomAuthCfg)
	}
	if req.CustomAuthCfg["memory_limit"] != limit {
		t.Fatalf("memory_limit not propagated: %#v", req.CustomAuthCfg)
	}
}
