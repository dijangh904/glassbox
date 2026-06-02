// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package report

import (
	"strings"
	"testing"

	"github.com/dotandev/glassbox/internal/trace"
)

func TestNewDiagnosticReportSeveritySummary(t *testing.T) {
	tr := trace.NewExecutionTrace("tx123", 0)
	tr.AddState(trace.ExecutionState{
		Operation:  "invoke",
		ContractID: "CA",
		Error:      "panic: index out of bounds",
	})
	tr.AddState(trace.ExecutionState{
		Operation:       "invoke",
		ContractID:      "CB",
		WasmInstruction: "unreachable",
	})

	r := NewDiagnosticReport(tr)
	if r.Counts["critical"] != 2 {
		t.Fatalf("critical count = %d, want 2", r.Counts["critical"])
	}
	if len(r.Diagnostics) != 2 {
		t.Fatalf("diagnostics len = %d, want 2", len(r.Diagnostics))
	}
}

func TestDiagnosticReportText(t *testing.T) {
	tr := trace.NewExecutionTrace("tx456", 0)
	tr.AddState(trace.ExecutionState{Operation: "invoke", Error: "failed"})

	text := NewDiagnosticReport(tr).Text()
	if !strings.Contains(text, "[HIGH] execution: Execution error") {
		t.Fatalf("text report missing high severity diagnostic: %s", text)
	}
}
