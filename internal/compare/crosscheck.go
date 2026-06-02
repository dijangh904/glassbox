// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package compare

import (
	"fmt"
	"strings"

	"github.com/dotandev/glassbox/internal/simulator"
)

// CrossCheckResult holds the outcome of comparing a local simulation diagnostic
// with the network-reported transaction failure reason.
type CrossCheckResult struct {
	// Match is true when local simulation and network agree on the failure category.
	Match bool

	// LocalCategory is the failure category from local simulation.
	LocalCategory simulator.FailureCategory

	// NetworkCategory is the inferred failure category of the network reason string.
	NetworkCategory simulator.FailureCategory

	// LocalSummary is the local simulation's diagnostic summary.
	LocalSummary string

	// NetworkReason is the raw network-reported failure reason string.
	NetworkReason string

	// Discrepancies lists specific differences found between the two results.
	Discrepancies []string

	// Explanation provides guidance for interpreting any mismatch.
	Explanation string
}

// CrossCheckFailures compares the local simulation FailureDiagnostic with the
// raw network transaction failure reason and returns a structured CrossCheckResult.
// If local is nil the result captures only the network-side view.
func CrossCheckFailures(local *simulator.FailureDiagnostic, networkReason string) *CrossCheckResult {
	result := &CrossCheckResult{
		NetworkReason:   networkReason,
		NetworkCategory: classifyNetworkReason(networkReason),
	}

	if local != nil {
		result.LocalCategory = local.Category
		result.LocalSummary = local.Summary
	} else {
		result.LocalCategory = simulator.FailureUnknown
	}

	result.Match = crossCheckCategoriesAgree(result.LocalCategory, result.NetworkCategory)

	if result.Match {
		result.Explanation = fmt.Sprintf(
			"Local simulation and network agree: the transaction failed due to %s.",
			result.LocalCategory,
		)
	} else {
		result.Discrepancies = buildCrossCheckDiscrepancies(result.LocalCategory, result.NetworkCategory, local)
		result.Explanation = buildCrossCheckExplanation(result.LocalCategory, result.NetworkCategory)
	}

	return result
}

// crossCheckCategoriesAgree returns true when the local and network categories match
// or when one side is UNKNOWN (insufficient information to declare a mismatch).
func crossCheckCategoriesAgree(local, network simulator.FailureCategory) bool {
	if local == network {
		return true
	}
	if local == simulator.FailureUnknown || network == simulator.FailureUnknown {
		return true
	}
	return false
}

// classifyNetworkReason infers a FailureCategory from a raw network failure reason string
// using the same pattern set as ClassifyFailure in the simulator package.
func classifyNetworkReason(reason string) simulator.FailureCategory {
	if reason == "" {
		return simulator.FailureUnknown
	}
	lc := strings.ToLower(reason)

	if strings.Contains(lc, "cpulimitexceeded") ||
		strings.Contains(lc, "cpu limit exceeded") ||
		strings.Contains(lc, "error(budget, cpu") {
		return simulator.FailureCPUBudget
	}
	if strings.Contains(lc, "memlimitexceeded") ||
		strings.Contains(lc, "memory limit exceeded") ||
		strings.Contains(lc, "error(budget, mem") {
		return simulator.FailureMemoryBudget
	}
	if strings.Contains(lc, "error(auth") ||
		strings.Contains(lc, "not authorized") ||
		strings.Contains(lc, "require_auth") ||
		strings.Contains(lc, "auth failed") ||
		strings.Contains(lc, "missing authorization") {
		return simulator.FailureAuthFailure
	}
	if strings.Contains(lc, "wasm trap") ||
		strings.Contains(lc, "unreachable") ||
		strings.Contains(lc, "panic:") ||
		strings.Contains(lc, "contract_invocation_failed") {
		return simulator.FailureContractTrap
	}
	if strings.Contains(lc, "storagefull") ||
		strings.Contains(lc, "storage full") ||
		strings.Contains(lc, "storage limit exceeded") ||
		strings.Contains(lc, "error(storage, full") ||
		strings.Contains(lc, "ledger entry count limit") {
		return simulator.FailureStorageOverflow
	}
	if strings.Contains(lc, "tx_soroban_invalid") ||
		strings.Contains(lc, "tx_malformed") ||
		strings.Contains(lc, "bad sequence") ||
		strings.Contains(lc, "insufficient fee") {
		return simulator.FailureValidation
	}
	return simulator.FailureUnknown
}

func buildCrossCheckDiscrepancies(
	local, network simulator.FailureCategory,
	localDiag *simulator.FailureDiagnostic,
) []string {
	var d []string
	d = append(d, fmt.Sprintf(
		"local simulation: %s — network report: %s",
		local, network,
	))

	switch {
	case local == simulator.FailureAuthFailure && network == simulator.FailureContractTrap:
		d = append(d, "the contract may have panicked on-chain before the auth check was reached; ensure authorization setup happens before any panic-path logic")
	case local == simulator.FailureContractTrap && network == simulator.FailureAuthFailure:
		d = append(d, "the contract traps locally but fails auth on-chain; check that all required auth entries are provided in the local simulation")
	case local == simulator.FailureCPUBudget && network != simulator.FailureCPUBudget:
		d = append(d, "resource consumption differs between local and network environments; verify that the contract version and ledger state are identical in both runs")
	case local == simulator.FailureStorageOverflow && network != simulator.FailureStorageOverflow:
		d = append(d, "the local environment may not enforce the same storage limits as the network; check protocol version and ledger configuration")
	case local == simulator.FailureUnknown:
		d = append(d, "local simulation could not classify the failure; inspect the raw error message for more context")
	}

	if localDiag != nil && localDiag.StorageDetails != nil {
		d = append(d, fmt.Sprintf("local storage overflow kind: %s", localDiag.StorageDetails.OverflowKind))
	}

	return d
}

func buildCrossCheckExplanation(local, network simulator.FailureCategory) string {
	return fmt.Sprintf(
		"The local simulation reports %s but the network failure indicates %s. "+
			"This discrepancy may be caused by differences in ledger state, contract version, "+
			"or host environment configuration between local and network execution.",
		local, network,
	)
}
