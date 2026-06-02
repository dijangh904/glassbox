// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package compare

import (
	"strings"
	"testing"

	"github.com/dotandev/glassbox/internal/simulator"
)

// ─── classifyNetworkReason ───────────────────────────────────────────────────

func TestClassifyNetworkReason_CPUBudget(t *testing.T) {
	inputs := []string{
		"CpuLimitExceeded: execution ran out of instructions",
		"error(budget, cpu): limit reached",
		"cpu limit exceeded during contract execution",
	}
	for _, in := range inputs {
		got := classifyNetworkReason(in)
		if got != simulator.FailureCPUBudget {
			t.Errorf("input %q: expected %s, got %s", in, simulator.FailureCPUBudget, got)
		}
	}
}

func TestClassifyNetworkReason_MemoryBudget(t *testing.T) {
	inputs := []string{
		"MemLimitExceeded: too much memory allocated",
		"error(budget, mem): limit reached",
		"memory limit exceeded",
	}
	for _, in := range inputs {
		got := classifyNetworkReason(in)
		if got != simulator.FailureMemoryBudget {
			t.Errorf("input %q: expected %s, got %s", in, simulator.FailureMemoryBudget, got)
		}
	}
}

func TestClassifyNetworkReason_AuthFailure(t *testing.T) {
	inputs := []string{
		"Error(Auth, NotAuthorized): require_auth check failed",
		"not authorized to invoke this contract",
		"auth failed: missing signature",
		"missing authorization for account",
	}
	for _, in := range inputs {
		got := classifyNetworkReason(in)
		if got != simulator.FailureAuthFailure {
			t.Errorf("input %q: expected %s, got %s", in, simulator.FailureAuthFailure, got)
		}
	}
}

func TestClassifyNetworkReason_ContractTrap(t *testing.T) {
	inputs := []string{
		"wasm trap: unreachable",
		"panic: assertion failed",
		"contract_invocation_failed",
	}
	for _, in := range inputs {
		got := classifyNetworkReason(in)
		if got != simulator.FailureContractTrap {
			t.Errorf("input %q: expected %s, got %s", in, simulator.FailureContractTrap, got)
		}
	}
}

func TestClassifyNetworkReason_StorageOverflow(t *testing.T) {
	inputs := []string{
		"StorageFull: max entries exceeded",
		"storage full: ledger entry count limit exceeded",
		"storage limit exceeded for this transaction",
		"error(storage, full): too many entries",
	}
	for _, in := range inputs {
		got := classifyNetworkReason(in)
		if got != simulator.FailureStorageOverflow {
			t.Errorf("input %q: expected %s, got %s", in, simulator.FailureStorageOverflow, got)
		}
	}
}

func TestClassifyNetworkReason_Validation(t *testing.T) {
	inputs := []string{
		"tx_soroban_invalid: resource limits exceeded",
		"tx_malformed: bad fee",
		"bad sequence number for account",
		"insufficient fee for this transaction size",
	}
	for _, in := range inputs {
		got := classifyNetworkReason(in)
		if got != simulator.FailureValidation {
			t.Errorf("input %q: expected %s, got %s", in, simulator.FailureValidation, got)
		}
	}
}

func TestClassifyNetworkReason_Unknown(t *testing.T) {
	inputs := []string{"", "some completely opaque error string", "network timeout"}
	for _, in := range inputs {
		got := classifyNetworkReason(in)
		if got != simulator.FailureUnknown {
			t.Errorf("input %q: expected %s, got %s", in, simulator.FailureUnknown, got)
		}
	}
}

// ─── CrossCheckFailures matching ─────────────────────────────────────────────

func TestCrossCheck_ExactMatch_CPU(t *testing.T) {
	local := &simulator.FailureDiagnostic{
		Category: simulator.FailureCPUBudget,
		Summary:  "CPU budget exceeded.",
	}
	r := CrossCheckFailures(local, "CpuLimitExceeded: instruction count exceeded")
	if !r.Match {
		t.Errorf("expected match, got mismatch: %v", r.Discrepancies)
	}
	if r.LocalCategory != simulator.FailureCPUBudget {
		t.Errorf("unexpected LocalCategory: %s", r.LocalCategory)
	}
	if r.NetworkCategory != simulator.FailureCPUBudget {
		t.Errorf("unexpected NetworkCategory: %s", r.NetworkCategory)
	}
}

func TestCrossCheck_ExactMatch_Auth(t *testing.T) {
	local := &simulator.FailureDiagnostic{
		Category: simulator.FailureAuthFailure,
		Summary:  "Authorization failure.",
	}
	r := CrossCheckFailures(local, "Error(Auth, NotAuthorized): require_auth failed")
	if !r.Match {
		t.Errorf("expected match, got mismatch: %v", r.Discrepancies)
	}
}

func TestCrossCheck_ExactMatch_StorageOverflow(t *testing.T) {
	local := &simulator.FailureDiagnostic{
		Category: simulator.FailureStorageOverflow,
		Summary:  "Storage overflow.",
		StorageDetails: &simulator.StorageDiagnosticDetails{
			OverflowKind: "entry_count",
		},
	}
	r := CrossCheckFailures(local, "StorageFull: ledger entry count limit exceeded")
	if !r.Match {
		t.Errorf("expected match, got mismatch: %v", r.Discrepancies)
	}
}

// ─── CrossCheckFailures mismatch ─────────────────────────────────────────────

func TestCrossCheck_Mismatch_AuthVsTrap(t *testing.T) {
	local := &simulator.FailureDiagnostic{
		Category: simulator.FailureAuthFailure,
		Summary:  "Authorization failure.",
	}
	r := CrossCheckFailures(local, "wasm trap: unreachable instruction")
	if r.Match {
		t.Error("expected mismatch for auth (local) vs trap (network)")
	}
	if r.LocalCategory != simulator.FailureAuthFailure {
		t.Errorf("unexpected LocalCategory: %s", r.LocalCategory)
	}
	if r.NetworkCategory != simulator.FailureContractTrap {
		t.Errorf("unexpected NetworkCategory: %s", r.NetworkCategory)
	}
	if len(r.Discrepancies) == 0 {
		t.Error("expected discrepancies to be populated")
	}
	if r.Explanation == "" {
		t.Error("expected non-empty Explanation")
	}
}

func TestCrossCheck_Mismatch_TrapVsAuth(t *testing.T) {
	local := &simulator.FailureDiagnostic{
		Category: simulator.FailureContractTrap,
		Summary:  "WASM trap.",
	}
	r := CrossCheckFailures(local, "Error(Auth, NotAuthorized)")
	if r.Match {
		t.Error("expected mismatch")
	}
	hasGuidance := false
	for _, d := range r.Discrepancies {
		if strings.Contains(d, "auth") {
			hasGuidance = true
		}
	}
	if !hasGuidance {
		t.Errorf("expected auth-related guidance in discrepancies, got: %v", r.Discrepancies)
	}
}

func TestCrossCheck_Mismatch_StorageVsValidation(t *testing.T) {
	local := &simulator.FailureDiagnostic{
		Category: simulator.FailureStorageOverflow,
		Summary:  "Storage full.",
		StorageDetails: &simulator.StorageDiagnosticDetails{
			OverflowKind: "footprint",
		},
	}
	r := CrossCheckFailures(local, "tx_soroban_invalid: resource limits")
	if r.Match {
		t.Error("expected mismatch for storage_overflow vs validation")
	}
	hasStorageDetail := false
	for _, d := range r.Discrepancies {
		if strings.Contains(d, "footprint") {
			hasStorageDetail = true
		}
	}
	if !hasStorageDetail {
		t.Errorf("expected storage overflow kind in discrepancies, got: %v", r.Discrepancies)
	}
}

// ─── Nil / unknown soft-match ─────────────────────────────────────────────────

func TestCrossCheck_NilLocal(t *testing.T) {
	r := CrossCheckFailures(nil, "CpuLimitExceeded: too many instructions")
	if r.Match {
		t.Error("nil local diagnostic should not produce a match")
	}
	if r.LocalCategory != simulator.FailureUnknown {
		t.Errorf("expected UNKNOWN for nil local, got %s", r.LocalCategory)
	}
}

func TestCrossCheck_UnknownNetwork_SoftMatch(t *testing.T) {
	local := &simulator.FailureDiagnostic{
		Category: simulator.FailureCPUBudget,
		Summary:  "CPU budget exhausted.",
	}
	r := CrossCheckFailures(local, "some opaque network error")
	if !r.Match {
		t.Errorf("UNKNOWN network category should be a soft match; got mismatch: %v", r.Discrepancies)
	}
}

func TestCrossCheck_UnknownLocal_SoftMatch(t *testing.T) {
	local := &simulator.FailureDiagnostic{
		Category: simulator.FailureUnknown,
		Summary:  "Unknown failure.",
	}
	r := CrossCheckFailures(local, "CpuLimitExceeded: too many instructions")
	if !r.Match {
		t.Errorf("UNKNOWN local category should be a soft match; got mismatch: %v", r.Discrepancies)
	}
}

// ─── RenderCrossCheck smoke test ──────────────────────────────────────────────

func TestRenderCrossCheck_NoError(t *testing.T) {
	r := &CrossCheckResult{
		Match:           true,
		LocalCategory:   simulator.FailureCPUBudget,
		NetworkCategory: simulator.FailureCPUBudget,
		LocalSummary:    "CPU budget exhausted.",
		NetworkReason:   "CpuLimitExceeded",
		Explanation:     "Local and network agree.",
	}
	// Should not panic.
	RenderCrossCheck(r)
}

func TestRenderCrossCheck_Nil_NoError(t *testing.T) {
	// Nil input must not panic.
	RenderCrossCheck(nil)
}

func TestRenderCrossCheck_Mismatch_NoError(t *testing.T) {
	r := &CrossCheckResult{
		Match:           false,
		LocalCategory:   simulator.FailureAuthFailure,
		NetworkCategory: simulator.FailureContractTrap,
		LocalSummary:    "Auth failure.",
		NetworkReason:   "wasm trap: unreachable",
		Discrepancies:   []string{"local: auth, network: trap"},
		Explanation:     "Discrepancy detected.",
	}
	// Should not panic.
	RenderCrossCheck(r)
}
