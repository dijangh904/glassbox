// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"testing"

	"github.com/dotandev/glassbox/internal/simulator"
)

func TestParseVerbosity(t *testing.T) {
	v, err := ParseVerbosity("summary")
	if err != nil || v != VerbositySummary {
		t.Fatalf("summary: v=%v err=%v", v, err)
	}
	v, err = ParseVerbosity("verbose")
	if err != nil || v != VerbosityVerbose {
		t.Fatalf("verbose: v=%v err=%v", v, err)
	}
}

func TestFilterExecutionTrace_Summary(t *testing.T) {
	tr := &ExecutionTrace{
		States: []ExecutionState{{
			Step:            1,
			Operation:       "call",
			WasmInstruction: "i32.const",
			SourceFile:      "lib.rs",
			GitHubLink:      "https://example.com",
		}},
	}
	filtered := FilterExecutionTrace(tr, VerbositySummary)
	s := filtered.States[0]
	if s.WasmInstruction != "" || s.SourceFile != "" || s.GitHubLink != "" {
		t.Fatalf("expected summary to strip details: %+v", s)
	}
}

func TestFilterDiagnosticEvents(t *testing.T) {
	inst := "call"
	events := []simulator.DiagnosticEvent{{Data: "payload", WasmInstruction: &inst}}
	out := FilterDiagnosticEvents(events, VerbosityNormal)
	if out[0].WasmInstruction != nil {
		t.Fatal("normal verbosity should strip wasm instructions")
	}
	out = FilterDiagnosticEvents(events, VerbositySummary)
	if out[0].Data != "" {
		t.Fatal("summary verbosity should strip event data")
	}
}
