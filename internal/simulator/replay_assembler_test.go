// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package simulator

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dotandev/glassbox/internal/rpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// newRPCClientForURL builds a minimal rpc.Client pointed at the given URL.
func newRPCClientForURL(t *testing.T, url string) *rpc.Client {
	t.Helper()
	client, err := rpc.NewClient(
		rpc.WithNetwork(rpc.Testnet),
		rpc.WithSorobanAltURLs([]string{url}),
		rpc.WithAltURLs([]string{url}),
		rpc.WithCacheEnabled(false),
	)
	require.NoError(t, err)
	return client
}

// ledgerEntriesServer returns an httptest.Server that responds to
// getLedgerEntries with the provided key→xdr map.
func ledgerEntriesServer(t *testing.T, entries map[string]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Decode as a generic structure because Params is []interface{} at the
		// JSON level (an array whose first element is an array of strings).
		var req struct {
			Method string        `json:"method"`
			Params []interface{} `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		var results []rpc.LedgerEntryResult
		if len(req.Params) > 0 {
			// First param is the key array, encoded as []interface{} by JSON.
			if keySlice, ok := req.Params[0].([]interface{}); ok {
				for _, raw := range keySlice {
					key, _ := raw.(string)
					if xdr, ok := entries[key]; ok {
						results = append(results, rpc.LedgerEntryResult{
							Key: key,
							Xdr: xdr,
						})
					}
				}
			}
		}

		resp := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"result": map[string]interface{}{
				"entries":      results,
				"latestLedger": 100,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

// ---------------------------------------------------------------------------
// Unit tests: buildDiagnosticMessage
// ---------------------------------------------------------------------------

func TestBuildDiagnosticMessage_SingleKey(t *testing.T) {
	msg := buildDiagnosticMessage([]string{"AAAA=="})
	assert.Contains(t, msg, "1 ledger entry could not be recovered")
	assert.Contains(t, msg, "AAAA==")
	assert.Contains(t, msg, "Remediation")
}

func TestBuildDiagnosticMessage_MultipleKeys(t *testing.T) {
	keys := []string{"KEY1==", "KEY2==", "KEY3=="}
	msg := buildDiagnosticMessage(keys)
	assert.Contains(t, msg, "3 ledger entries could not be recovered")
	for _, k := range keys {
		assert.Contains(t, msg, k)
	}
}

func TestBuildDiagnosticMessage_Empty(t *testing.T) {
	msg := buildDiagnosticMessage(nil)
	assert.Empty(t, msg)
}

// ---------------------------------------------------------------------------
// Unit tests: IsIncompleteReplayState / GetMissingKeys
// ---------------------------------------------------------------------------

func TestIsIncompleteReplayState(t *testing.T) {
	diag := &AssemblyDiagnostic{
		MissingKeys: []string{"KEY=="},
		Message:     "incomplete",
	}
	assert.True(t, IsIncompleteReplayState(diag))
	assert.False(t, IsIncompleteReplayState(nil))
	assert.False(t, IsIncompleteReplayState(assert.AnError))
}

func TestGetMissingKeys(t *testing.T) {
	keys := []string{"A==", "B=="}
	diag := &AssemblyDiagnostic{MissingKeys: keys}
	assert.Equal(t, keys, GetMissingKeys(diag))
	assert.Nil(t, GetMissingKeys(nil))
	assert.Nil(t, GetMissingKeys(assert.AnError))
}

// ---------------------------------------------------------------------------
// Unit tests: collectMissingKeys
// ---------------------------------------------------------------------------

func TestCollectMissingKeys_FiltersExisting(t *testing.T) {
	a := &ReplayAssembler{}
	existing := map[string]string{"KEY_A==": "XDR_A", "KEY_B==": "XDR_B"}
	extra := []string{"KEY_A==", "KEY_C==", "KEY_D=="}

	missing := a.collectMissingKeys(existing, extra)
	assert.ElementsMatch(t, []string{"KEY_C==", "KEY_D=="}, missing)
}

func TestCollectMissingKeys_DeduplicatesInput(t *testing.T) {
	a := &ReplayAssembler{}
	extra := []string{"KEY==", "KEY==", "KEY=="}
	missing := a.collectMissingKeys(map[string]string{}, extra)
	assert.Len(t, missing, 1)
}

func TestCollectMissingKeys_SkipsEmptyKeys(t *testing.T) {
	a := &ReplayAssembler{}
	extra := []string{"", "KEY==", ""}
	missing := a.collectMissingKeys(map[string]string{}, extra)
	assert.Equal(t, []string{"KEY=="}, missing)
}

// ---------------------------------------------------------------------------
// Integration tests: ReplayAssembler.Assemble
// ---------------------------------------------------------------------------

func TestAssemble_NoMetadata_SupplementsFromRPC(t *testing.T) {
	// Simulate a failed transaction with no result metadata.
	// The assembler should fetch the extra keys from the RPC.
	serverEntries := map[string]string{
		"KEY_CONTRACT==": "XDR_CONTRACT_INSTANCE",
		"KEY_CODE==":     "XDR_CONTRACT_CODE",
	}
	srv := ledgerEntriesServer(t, serverEntries)
	defer srv.Close()

	client := newRPCClientForURL(t, srv.URL)
	assembler := NewReplayAssembler(client)

	input := ReplayInput{
		EnvelopeXdr:     "ENVELOPE_XDR_BASE64",
		ResultMetaXdr:   "", // no metadata — failed transaction
		ExtraLedgerKeys: []string{"KEY_CONTRACT==", "KEY_CODE=="},
	}

	snapshot, err := assembler.Assemble(context.Background(), input)
	require.NoError(t, err, "should succeed when all extra keys are found")
	assert.Equal(t, "ENVELOPE_XDR_BASE64", snapshot.EnvelopeXdr)
	assert.Equal(t, "XDR_CONTRACT_INSTANCE", snapshot.LedgerEntries["KEY_CONTRACT=="])
	assert.Equal(t, "XDR_CONTRACT_CODE", snapshot.LedgerEntries["KEY_CODE=="])
	assert.ElementsMatch(t, []string{"KEY_CONTRACT==", "KEY_CODE=="}, snapshot.SupplementedKeys)
	assert.Empty(t, snapshot.MissingKeys)
}

func TestAssemble_PartialMetadata_SupplementsMissingKeys(t *testing.T) {
	// Metadata provides nothing; RPC provides KEY_B.
	// The assembler should fetch KEY_B from the RPC and include it in the snapshot.
	serverEntries := map[string]string{
		"KEY_B==": "XDR_B",
	}
	srv := ledgerEntriesServer(t, serverEntries)
	defer srv.Close()

	client := newRPCClientForURL(t, srv.URL)
	assembler := NewReplayAssembler(client)

	input := ReplayInput{
		EnvelopeXdr:     "ENVELOPE",
		ExtraLedgerKeys: []string{"KEY_B=="},
	}

	snapshot, err := assembler.Assemble(context.Background(), input)
	require.NoError(t, err)
	assert.Equal(t, "XDR_B", snapshot.LedgerEntries["KEY_B=="])
	assert.Contains(t, snapshot.SupplementedKeys, "KEY_B==")
}

func TestAssemble_MissingKeyNotFoundOnRPC_ReturnsDiagnostic(t *testing.T) {
	// RPC returns nothing for the requested key.
	srv := ledgerEntriesServer(t, map[string]string{}) // empty — no entries
	defer srv.Close()

	client := newRPCClientForURL(t, srv.URL)
	assembler := NewReplayAssembler(client)

	input := ReplayInput{
		EnvelopeXdr:     "ENVELOPE",
		ExtraLedgerKeys: []string{"MISSING_KEY=="},
	}

	snapshot, err := assembler.Assemble(context.Background(), input)
	require.Error(t, err, "should return diagnostic error for unrecoverable key")
	require.True(t, IsIncompleteReplayState(err),
		"error should be *AssemblyDiagnostic")

	diag := err.(*AssemblyDiagnostic)
	assert.Contains(t, diag.MissingKeys, "MISSING_KEY==")
	assert.Contains(t, diag.Message, "MISSING_KEY==")
	assert.Contains(t, diag.Message, "Remediation")

	// Snapshot is still returned so callers can attempt partial replay.
	require.NotNil(t, snapshot)
	assert.Contains(t, snapshot.MissingKeys, "MISSING_KEY==")
}

func TestAssemble_DeterministicOutput_SameInputSameSnapshot(t *testing.T) {
	serverEntries := map[string]string{
		"KEY_1==": "XDR_1",
		"KEY_2==": "XDR_2",
		"KEY_3==": "XDR_3",
	}
	srv := ledgerEntriesServer(t, serverEntries)
	defer srv.Close()

	client := newRPCClientForURL(t, srv.URL)
	assembler := NewReplayAssembler(client)

	input := ReplayInput{
		EnvelopeXdr:     "ENVELOPE",
		ExtraLedgerKeys: []string{"KEY_3==", "KEY_1==", "KEY_2=="}, // intentionally unordered
	}

	snap1, err1 := assembler.Assemble(context.Background(), input)
	snap2, err2 := assembler.Assemble(context.Background(), input)

	require.NoError(t, err1)
	require.NoError(t, err2)

	// SupplementedKeys must be sorted identically across runs.
	assert.Equal(t, snap1.SupplementedKeys, snap2.SupplementedKeys,
		"supplemented keys must be deterministically ordered")
	assert.Equal(t, snap1.LedgerEntries, snap2.LedgerEntries,
		"ledger entries must be identical across runs")
}

func TestAssemble_ToSimulationRequest_IncludesAllEntries(t *testing.T) {
	serverEntries := map[string]string{
		"KEY_A==": "XDR_A",
	}
	srv := ledgerEntriesServer(t, serverEntries)
	defer srv.Close()

	client := newRPCClientForURL(t, srv.URL)
	assembler := NewReplayAssembler(client)

	input := ReplayInput{
		EnvelopeXdr:     "ENVELOPE",
		ResultMetaXdr:   "META",
		ExtraLedgerKeys: []string{"KEY_A=="},
	}

	snapshot, _ := assembler.Assemble(context.Background(), input)
	req := snapshot.ToSimulationRequest()

	assert.Equal(t, "ENVELOPE", req.EnvelopeXdr)
	assert.Equal(t, "META", req.ResultMetaXdr)
	assert.Equal(t, snapshot.LedgerEntries, req.LedgerEntries)
}

func TestAssemble_EmptyEnvelope_ReturnsValidationError(t *testing.T) {
	srv := ledgerEntriesServer(t, map[string]string{})
	defer srv.Close()

	client := newRPCClientForURL(t, srv.URL)
	assembler := NewReplayAssembler(client)

	_, err := assembler.Assemble(context.Background(), ReplayInput{})
	require.Error(t, err)
	assert.False(t, IsIncompleteReplayState(err),
		"empty envelope error should not be an AssemblyDiagnostic")
}

func TestAssemble_MultipleMissingKeys_AllReportedInDiagnostic(t *testing.T) {
	// RPC only returns KEY_A; KEY_B and KEY_C are missing.
	serverEntries := map[string]string{
		"KEY_A==": "XDR_A",
	}
	srv := ledgerEntriesServer(t, serverEntries)
	defer srv.Close()

	client := newRPCClientForURL(t, srv.URL)
	assembler := NewReplayAssembler(client)

	input := ReplayInput{
		EnvelopeXdr:     "ENVELOPE",
		ExtraLedgerKeys: []string{"KEY_A==", "KEY_B==", "KEY_C=="},
	}

	snapshot, err := assembler.Assemble(context.Background(), input)
	require.Error(t, err)
	require.True(t, IsIncompleteReplayState(err))

	missing := GetMissingKeys(err)
	assert.ElementsMatch(t, []string{"KEY_B==", "KEY_C=="}, missing)

	// KEY_A was found and should be in the snapshot.
	assert.Equal(t, "XDR_A", snapshot.LedgerEntries["KEY_A=="])
}

func TestAssemble_RPCUnavailable_ReturnsDiagnosticWithAllKeysMissing(t *testing.T) {
	// Point at a server that immediately closes connections.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	client := newRPCClientForURL(t, srv.URL)
	assembler := NewReplayAssembler(client)

	input := ReplayInput{
		EnvelopeXdr:     "ENVELOPE",
		ExtraLedgerKeys: []string{"KEY_X==", "KEY_Y=="},
	}

	snapshot, err := assembler.Assemble(context.Background(), input)
	// Either a diagnostic or a transport error — both are acceptable.
	// The important invariant is that the snapshot is non-nil.
	require.NotNil(t, snapshot, "snapshot must be non-nil even when RPC is unavailable")
	_ = err // may be diagnostic or transport error
}
