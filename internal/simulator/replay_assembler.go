// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

// Package simulator provides local Soroban transaction replay and simulation.
//
// ReplayAssembler is the staged pipeline that turns raw on-chain data (a
// transaction envelope + result metadata) into a complete, deterministic
// SimulationRequest.  When the result metadata does not carry a full ledger
// footprint — which happens for failed transactions and some older protocol
// versions — the assembler queries the Soroban RPC for the missing entries
// before handing the snapshot to the simulator.
package simulator

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/dotandev/glassbox/internal/errors"
	"github.com/dotandev/glassbox/internal/logger"
	"github.com/dotandev/glassbox/internal/rpc"
)

// ReplayInput is the raw on-chain data needed to start assembly.
type ReplayInput struct {
	// EnvelopeXdr is the base64-encoded TransactionEnvelope XDR.
	EnvelopeXdr string

	// ResultMetaXdr is the base64-encoded TransactionResultMeta XDR.
	// May be empty for failed transactions that produced no metadata.
	ResultMetaXdr string

	// ExtraLedgerKeys is an optional caller-supplied list of additional
	// base64-encoded XDR LedgerKeys to fetch.  Use this when you know
	// ahead of time that certain entries are required but absent from the
	// result metadata (e.g. contract code for a deploy transaction).
	ExtraLedgerKeys []string

	// ContractIDs is an optional list of contract IDs (strkey C… or hex)
	// whose instance + code entries should be fetched unconditionally.
	// This is useful when replaying invocations where the contract was
	// deployed in a prior transaction and is therefore absent from the
	// current result metadata.
	ContractIDs []string
}

// ReplaySnapshot is the fully assembled, deterministic state snapshot that
// can be passed directly to the simulator.
type ReplaySnapshot struct {
	// EnvelopeXdr is the transaction envelope, unchanged from the input.
	EnvelopeXdr string

	// ResultMetaXdr is the result metadata, unchanged from the input.
	ResultMetaXdr string

	// LedgerEntries is the merged map of base64 XDR key → base64 XDR entry
	// that the simulator needs to reproduce the transaction locally.
	LedgerEntries map[string]string

	// MissingKeys lists any keys that could not be recovered.  A non-empty
	// slice means the snapshot is partial; the simulator may still succeed
	// if the missing entries are not actually accessed during replay.
	MissingKeys []string

	// SupplementedKeys records which keys were fetched from the RPC rather
	// than extracted from the result metadata.  Useful for diagnostics.
	SupplementedKeys []string
}

// ToSimulationRequest converts the snapshot into a SimulationRequest ready
// for Runner.Run.
func (s *ReplaySnapshot) ToSimulationRequest() *SimulationRequest {
	return &SimulationRequest{
		EnvelopeXdr:   s.EnvelopeXdr,
		ResultMetaXdr: s.ResultMetaXdr,
		LedgerEntries: s.LedgerEntries,
	}
}

// AssemblyDiagnostic carries a human-readable explanation of what went wrong
// when the assembler cannot recover all required state.
type AssemblyDiagnostic struct {
	MissingKeys      []string
	SupplementedKeys []string
	Message          string
}

func (d *AssemblyDiagnostic) Error() string { return d.Message }

// ReplayAssembler builds deterministic replay snapshots from partial on-chain
// data by supplementing missing ledger entries via Soroban RPC queries.
type ReplayAssembler struct {
	client *rpc.Client
}

// NewReplayAssembler creates an assembler backed by the given RPC client.
func NewReplayAssembler(client *rpc.Client) *ReplayAssembler {
	return &ReplayAssembler{client: client}
}

// Assemble runs the full staged pipeline:
//
//  1. Extract ledger entries from the result metadata (may be empty for
//     failed transactions).
//  2. Detect missing read-only / read-write footprint entries.
//  3. Supplement missing entries via GetLedgerEntries.
//  4. Fetch contract instance + code for any ContractIDs in the input.
//  5. Merge everything into a deterministic snapshot.
//
// The returned snapshot is always non-nil.  If some entries could not be
// recovered, MissingKeys is populated and an *AssemblyDiagnostic is returned
// as the error so callers can decide whether to proceed with a partial replay.
func (a *ReplayAssembler) Assemble(ctx context.Context, input ReplayInput) (*ReplaySnapshot, error) {
	if input.EnvelopeXdr == "" {
		return nil, errors.WrapValidationError("EnvelopeXdr is required for replay assembly")
	}

	snapshot := &ReplaySnapshot{
		EnvelopeXdr:   input.EnvelopeXdr,
		ResultMetaXdr: input.ResultMetaXdr,
		LedgerEntries: make(map[string]string),
	}

	// ── Stage 1: extract entries from result metadata ──────────────────────
	if input.ResultMetaXdr != "" {
		extracted, err := rpc.ExtractLedgerEntriesFromMeta(input.ResultMetaXdr)
		if err != nil {
			logger.Logger.Warn("Failed to extract ledger entries from result metadata; "+
				"will attempt full supplementary fetch",
				"error", err)
			// Non-fatal: continue with an empty base and rely on supplementary fetch.
		} else {
			for k, v := range extracted {
				snapshot.LedgerEntries[k] = v
			}
			logger.Logger.Debug("Extracted ledger entries from result metadata",
				"count", len(extracted))
		}
	} else {
		logger.Logger.Debug("No result metadata provided; skipping extraction stage")
	}

	// ── Stage 2: detect missing footprint entries ───────────────────────────
	// Build the union of keys we need: extra keys + contract IDs.
	keysToFetch := a.collectMissingKeys(snapshot.LedgerEntries, input.ExtraLedgerKeys)

	// ── Stage 3: supplement missing entries via GetLedgerEntries ───────────
	if len(keysToFetch) > 0 {
		supplemented, missing, err := a.fetchMissingEntries(ctx, keysToFetch, snapshot.LedgerEntries)
		if err != nil {
			logger.Logger.Warn("Supplementary ledger entry fetch encountered errors",
				"error", err)
		}
		for k, v := range supplemented {
			snapshot.LedgerEntries[k] = v
			snapshot.SupplementedKeys = append(snapshot.SupplementedKeys, k)
		}
		snapshot.MissingKeys = append(snapshot.MissingKeys, missing...)
	}

	// ── Stage 4: fetch contract instance + code ─────────────────────────────
	if len(input.ContractIDs) > 0 {
		contractEntries, err := rpc.FetchBytecodeForTraceContractCalls(
			ctx, a.client, input.ContractIDs, snapshot.LedgerEntries,
		)
		if err != nil {
			logger.Logger.Warn("Contract bytecode fetch encountered errors", "error", err)
		}
		for k, v := range contractEntries {
			if _, exists := snapshot.LedgerEntries[k]; !exists {
				snapshot.LedgerEntries[k] = v
				snapshot.SupplementedKeys = append(snapshot.SupplementedKeys, k)
			}
		}
	}

	// ── Stage 5: deterministic ordering ─────────────────────────────────────
	// Sort supplemented and missing key slices so the snapshot is stable
	// across runs regardless of map iteration order.
	sort.Strings(snapshot.SupplementedKeys)
	sort.Strings(snapshot.MissingKeys)

	logger.Logger.Debug("Replay snapshot assembled",
		"total_entries", len(snapshot.LedgerEntries),
		"supplemented", len(snapshot.SupplementedKeys),
		"missing", len(snapshot.MissingKeys),
	)

	// Return a diagnostic error when entries could not be recovered so callers
	// can surface actionable messages rather than a cryptic simulator crash.
	if len(snapshot.MissingKeys) > 0 {
		return snapshot, &AssemblyDiagnostic{
			MissingKeys:      snapshot.MissingKeys,
			SupplementedKeys: snapshot.SupplementedKeys,
			Message:          buildDiagnosticMessage(snapshot.MissingKeys),
		}
	}

	return snapshot, nil
}

// collectMissingKeys returns the subset of extraKeys that are not already
// present in the existing entries map.
func (a *ReplayAssembler) collectMissingKeys(existing map[string]string, extraKeys []string) []string {
	var missing []string
	seen := make(map[string]struct{}, len(extraKeys))
	for _, k := range extraKeys {
		if k == "" {
			continue
		}
		if _, dup := seen[k]; dup {
			continue
		}
		seen[k] = struct{}{}
		if _, ok := existing[k]; !ok {
			missing = append(missing, k)
		}
	}
	return missing
}

// fetchMissingEntries calls GetLedgerEntries for the given keys and returns:
//   - supplemented: keys that were successfully fetched
//   - missing: keys that the RPC returned no data for
//   - err: any transport-level error (non-fatal; partial results are still returned)
func (a *ReplayAssembler) fetchMissingEntries(
	ctx context.Context,
	keys []string,
	existing map[string]string,
) (supplemented map[string]string, missing []string, err error) {
	supplemented = make(map[string]string)

	fetched, fetchErr := a.client.GetLedgerEntries(ctx, keys)
	if fetchErr != nil {
		// Return whatever we got (may be nil) and surface the error.
		return supplemented, keys, fetchErr
	}

	for _, k := range keys {
		v, ok := fetched[k]
		if !ok || v == "" {
			missing = append(missing, k)
			logger.Logger.Warn("Ledger entry not found during supplementary fetch",
				"key", k)
		} else {
			supplemented[k] = v
		}
	}

	return supplemented, missing, nil
}

// buildDiagnosticMessage produces a human-readable explanation of which
// ledger entries or code objects are missing so operators can act on it.
func buildDiagnosticMessage(missingKeys []string) string {
	if len(missingKeys) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(
		"replay state is incomplete: %d ledger entr%s could not be recovered from the RPC.\n",
		len(missingKeys),
		pluralSuffix(len(missingKeys), "y", "ies"),
	))
	sb.WriteString("Missing keys:\n")
	for _, k := range missingKeys {
		sb.WriteString("  • ")
		sb.WriteString(k)
		sb.WriteString("\n")
	}
	sb.WriteString("\nPossible causes:\n")
	sb.WriteString("  • The entry was archived or expired before this replay was attempted.\n")
	sb.WriteString("  • The RPC node does not retain ledger history for this sequence.\n")
	sb.WriteString("  • The contract was deployed in a prior transaction not included in the replay input.\n")
	sb.WriteString("\nRemediation:\n")
	sb.WriteString("  • Pass the contract ID(s) via ReplayInput.ContractIDs to fetch code on demand.\n")
	sb.WriteString("  • Pass the missing key(s) via ReplayInput.ExtraLedgerKeys.\n")
	sb.WriteString("  • Use an archival RPC node that retains full ledger history.\n")
	return sb.String()
}

func pluralSuffix(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}

// IsIncompleteReplayState reports whether err is an *AssemblyDiagnostic,
// meaning the snapshot was assembled but some entries are missing.
func IsIncompleteReplayState(err error) bool {
	_, ok := err.(*AssemblyDiagnostic)
	return ok
}

// GetMissingKeys extracts the missing key list from an *AssemblyDiagnostic
// error.  Returns nil if err is not an *AssemblyDiagnostic.
func GetMissingKeys(err error) []string {
	if d, ok := err.(*AssemblyDiagnostic); ok {
		return d.MissingKeys
	}
	return nil
}
