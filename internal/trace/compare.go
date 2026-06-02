// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/dotandev/glassbox/internal/visualizer"
)

// StateDiff represents the difference between two ExecutionStates
type StateDiff struct {
	Step        int
	Field       string
	Baseline    interface{}
	Current     interface{}
	Difference  string
	Divergent   bool
}

// TraceDiffResult contains all differences between two ExecutionTraces
type TraceDiffResult struct {
	HasDivergence   bool
	StatusDiff      *StatusDiff
	StateDiffs      []StateDiff
	CallPathDiffs   []CallPathDiff
	BaselineName    string
	CurrentName     string
}

// StatusDiff compares top-level trace status
type StatusDiff struct {
	Match         bool
	BaselineSteps int
	CurrentSteps  int
	BaselineTx    string
	CurrentTx     string
}

// CallPathDiff represents a divergence in call paths
type CallPathDiff struct {
	Step         int
	Reason       string
	BaselinePath string
	CurrentPath  string
}

// CompareTraces compares two ExecutionTraces and returns a TraceDiffResult
func CompareTraces(baseline, current *ExecutionTrace, baselineName, currentName string) *TraceDiffResult {
	if baselineName == "" {
		baselineName = "Baseline"
	}
	if currentName == "" {
		currentName = "Current"
	}

	result := &TraceDiffResult{
		BaselineName: baselineName,
		CurrentName:  currentName,
	}

	// Compare overall status
	result.StatusDiff = compareTraceStatus(baseline, current)
	result.HasDivergence = !result.StatusDiff.Match

	// Compare each state
	result.StateDiffs = compareExecutionStates(baseline, current)
	if len(result.StateDiffs) > 0 {
		result.HasDivergence = true
	}

	// Compare call paths
	result.CallPathDiffs = compareCallPaths(baseline, current)
	if len(result.CallPathDiffs) > 0 {
		result.HasDivergence = true
	}

	return result
}

func compareTraceStatus(baseline, current *ExecutionTrace) *StatusDiff {
	sd := &StatusDiff{
		BaselineSteps: len(baseline.States),
		CurrentSteps:  len(current.States),
		BaselineTx:    baseline.TransactionHash,
		CurrentTx:     current.TransactionHash,
	}

	sd.Match = len(baseline.States) == len(current.States) &&
		baseline.TransactionHash == current.TransactionHash

	return sd
}

func compareExecutionStates(baseline, current *ExecutionTrace) []StateDiff {
	var diffs []StateDiff

	maxSteps := len(baseline.States)
	if len(current.States) > maxSteps {
		maxSteps = len(current.States)
	}

	for i := 0; i < maxSteps; i++ {
		if i >= len(baseline.States) {
			// State only in current
			diffs = append(diffs, StateDiff{
				Step:        i,
				Field:       "State",
				Baseline:    nil,
				Current:     current.States[i],
				Difference:  "Extra state in current trace",
				Divergent:   true,
			})
			continue
		}
		if i >= len(current.States) {
			// State only in baseline
			diffs = append(diffs, StateDiff{
				Step:        i,
				Field:       "State",
				Baseline:    baseline.States[i],
				Current:     nil,
				Difference:  "Missing state in current trace",
				Divergent:   true,
			})
			continue
		}

		// Compare individual fields of the state
		bState := &baseline.States[i]
		cState := &current.States[i]

		// Compare Operation
		if bState.Operation != cState.Operation {
			diffs = append(diffs, StateDiff{
				Step:        i,
				Field:       "Operation",
				Baseline:    bState.Operation,
				Current:     cState.Operation,
				Difference:  "Operation mismatch",
				Divergent:   true,
			})
		}

		// Compare EventType
		if bState.EventType != cState.EventType {
			diffs = append(diffs, StateDiff{
				Step:        i,
				Field:       "EventType",
				Baseline:    bState.EventType,
				Current:     cState.EventType,
				Difference:  "Event type mismatch",
				Divergent:   true,
			})
		}

		// Compare ContractID
		if bState.ContractID != cState.ContractID {
			diffs = append(diffs, StateDiff{
				Step:        i,
				Field:       "ContractID",
				Baseline:    bState.ContractID,
				Current:     cState.ContractID,
				Difference:  "Contract ID mismatch",
				Divergent:   true,
			})
		}

		// Compare Function
		if bState.Function != cState.Function {
			diffs = append(diffs, StateDiff{
				Step:        i,
				Field:       "Function",
				Baseline:    bState.Function,
				Current:     cState.Function,
				Difference:  "Function mismatch",
				Divergent:   true,
			})
		}

		// Compare Error
		if bState.Error != cState.Error {
			diffs = append(diffs, StateDiff{
				Step:        i,
				Field:       "Error",
				Baseline:    bState.Error,
				Current:     cState.Error,
				Difference:  "Error mismatch",
				Divergent:   true,
			})
		}

		// Compare Arguments
		if !reflect.DeepEqual(bState.Arguments, cState.Arguments) {
			diffs = append(diffs, StateDiff{
				Step:        i,
				Field:       "Arguments",
				Baseline:    bState.Arguments,
				Current:     cState.Arguments,
				Difference:  "Arguments mismatch",
				Divergent:   true,
			})
		}

		// Compare ReturnValue
		if !reflect.DeepEqual(bState.ReturnValue, cState.ReturnValue) {
			diffs = append(diffs, StateDiff{
				Step:        i,
				Field:       "ReturnValue",
				Baseline:    bState.ReturnValue,
				Current:     cState.ReturnValue,
				Difference:  "Return value mismatch",
				Divergent:   true,
			})
		}

		// Compare WasmInstruction
		if bState.WasmInstruction != cState.WasmInstruction {
			diffs = append(diffs, StateDiff{
				Step:        i,
				Field:       "WasmInstruction",
				Baseline:    bState.WasmInstruction,
				Current:     cState.WasmInstruction,
				Difference:  "WASM instruction mismatch",
				Divergent:   true,
			})
		}
	}

	return diffs
}

func compareCallPaths(baseline, current *ExecutionTrace) []CallPathDiff {
	var diffs []CallPathDiff
	// Simple call path comparison - build path strings and compare
	bPath := buildCallPathString(baseline)
	cPath := buildCallPathString(current)
	
	if bPath != cPath {
		diffs = append(diffs, CallPathDiff{
			Step:         0,
			Reason:       "Overall call path differs",
			BaselinePath: bPath,
			CurrentPath:  cPath,
		})
	}
	
	return diffs
}

func buildCallPathString(trace *ExecutionTrace) string {
	var parts []string
	for _, state := range trace.States {
		if state.Function != "" {
			part := fmt.Sprintf("%s::%s", state.ContractID, state.Function)
			if len(parts) == 0 || parts[len(parts)-1] != part {
				parts = append(parts, part)
			}
		}
	}
	return strings.Join(parts, " → ")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 3 {
		return s[:n]
	}
	return s[:n-3] + "…"
}

func sectionTitle(title string) string {
	line := "── " + title + " " + strings.Repeat("─", max(0, 60-len(title)))
	return visualizer.Colorize(line, "bold")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Render prints a human-readable diff of a TraceDiffResult
func (r *TraceDiffResult) Render() {
	if r == nil {
		return
	}

	colWidth := 50
	sep := strings.Repeat("─", colWidth*2+3)
	
	fmt.Println(visualizer.Colorize("╔"+strings.Repeat("═", len(sep))+"╗", "cyan"))
	title := fmt.Sprintf("  TRACE COMPARISON  ─  %s vs %s  ", r.BaselineName, r.CurrentName)
	pad := len(sep) - len(title)
	if pad < 0 {
		pad = 0
	}
	fmt.Printf(visualizer.Colorize("║", "cyan")+"%s"+strings.Repeat(" ", pad)+visualizer.Colorize("║", "cyan")+"\n", title)
	fmt.Println(visualizer.Colorize("╚"+strings.Repeat("═", len(sep))+"╝", "cyan"))
	fmt.Println()

	// Status diff
	fmt.Println(sectionTitle("Overall Status"))
	renderTraceStatusDiff(r.StatusDiff, r.BaselineName, r.CurrentName)
	
	// State diffs
	if len(r.StateDiffs) > 0 {
		fmt.Println()
		fmt.Println(sectionTitle("Execution State Diffs"))
		renderStateDiffs(r.StateDiffs, r.BaselineName, r.CurrentName)
	}
	
	// Call path diffs
	if len(r.CallPathDiffs) > 0 {
		fmt.Println()
		fmt.Println(sectionTitle("Call Path Diffs"))
		renderCallPathDiffs(r.CallPathDiffs, r.BaselineName, r.CurrentName)
	}
	
	// Summary
	fmt.Println()
	fmt.Println(sectionTitle("Summary"))
	fmt.Println()
	if !r.HasDivergence {
		fmt.Printf("  %s Traces are IDENTICAL\n", visualizer.Success())
	} else {
		fmt.Printf("  %s Divergence detected\n", visualizer.Warning())
		fmt.Printf("  - %d state differences\n", len(r.StateDiffs))
		fmt.Printf("  - %d call path differences\n", len(r.CallPathDiffs))
	}
	fmt.Println()
	fmt.Println(visualizer.Colorize(sep, "dim"))
}

func renderTraceStatusDiff(sd *StatusDiff, bName, cName string) {
	fmt.Printf("  %-*s│  %s\n", 50, bName, cName)
	fmt.Printf("  %s\n", strings.Repeat("-", 103))
	
	// Steps
	fmt.Printf("  Steps: %-*d│  %d\n", 43, sd.BaselineSteps, sd.CurrentSteps)
	// Tx Hash
	fmt.Printf("  Tx:    %-*s│  %s\n", 43, truncate(sd.BaselineTx, 40), truncate(sd.CurrentTx, 40))
	
	if sd.Match {
		fmt.Printf("\n  %s  Status matches\n", visualizer.Colorize("[MATCH]", "green"))
	} else {
		fmt.Printf("\n  %s  Status mismatch\n", visualizer.Colorize("[DIFF]", "red"))
	}
}

func renderStateDiffs(diffs []StateDiff, bName, cName string) {
	for _, diff := range diffs {
		var marker string
		if diff.Divergent {
			marker = visualizer.Colorize("[DIFF]", "red")
		} else {
			marker = visualizer.Colorize("[=]", "dim")
		}
		fmt.Printf("%s  Step %d: %s\n", marker, diff.Step, diff.Difference)
		if diff.Baseline != nil {
			fmt.Printf("    %s: %v\n", bName, diff.Baseline)
		}
		if diff.Current != nil {
			fmt.Printf("    %s: %v\n", cName, diff.Current)
		}
		fmt.Println()
	}
}

func renderCallPathDiffs(diffs []CallPathDiff, bName, cName string) {
	for _, diff := range diffs {
		fmt.Printf("  %s  %s\n", visualizer.Colorize("[PATH]", "red"), diff.Reason)
		fmt.Printf("    %s: %s\n", bName, visualizer.Colorize(diff.BaselinePath, "cyan"))
		fmt.Printf("    %s: %s\n", cName, visualizer.Colorize(diff.CurrentPath, "magenta"))
		fmt.Println()
	}
}
