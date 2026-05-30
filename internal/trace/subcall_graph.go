// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"fmt"
	"strings"
)

// CallBoundary marks the boundary of a single contract invocation within a
// multi-step transaction. Each boundary tracks the range of execution steps
// that belong to the call and any nested sub-calls it makes.
type CallBoundary struct {
	// ID is a unique identifier for this boundary, e.g. "call-0" or "call-0.1".
	ID string

	// ContractID is the callee contract address.
	ContractID string

	// Function is the entry-point function name.
	Function string

	// EntryStep is the step index in ExecutionTrace.States where this call begins.
	EntryStep int

	// ExitStep is the step index where this call returns or errors.
	// -1 means the call did not complete (e.g. the trace ended mid-execution).
	ExitStep int

	// Error holds the error message when the call failed.
	Error string

	// Depth is the nesting depth: 0 = top-level, 1 = direct subcall, etc.
	Depth int

	// Parent points to the enclosing call boundary. Nil for top-level calls.
	Parent *CallBoundary

	// SubCalls lists the nested invocations made by this call, in order.
	SubCalls []*CallBoundary

	// Expanded controls whether subcalls are shown in the interactive view.
	Expanded bool
}

// StepCount returns the number of execution steps covered by this boundary.
func (b *CallBoundary) StepCount() int {
	if b.ExitStep < 0 {
		return 0
	}
	return b.ExitStep - b.EntryStep + 1
}

// HasError reports whether this call boundary ended with an error.
func (b *CallBoundary) HasError() bool {
	return b.Error != ""
}

// IsPartial reports whether the call did not complete in the trace.
func (b *CallBoundary) IsPartial() bool {
	return b.ExitStep < 0
}

// Label returns a short human-readable label for display, e.g. "contract_id::function".
func (b *CallBoundary) Label() string {
	if b.ContractID != "" && b.Function != "" {
		short := b.ContractID
		if len(short) > 12 {
			short = short[:6] + "…" + short[len(short)-6:]
		}
		return short + "::" + b.Function
	}
	if b.Function != "" {
		return b.Function
	}
	if b.ContractID != "" {
		return b.ContractID
	}
	return fmt.Sprintf("step-%d", b.EntryStep)
}

// FlattenAll returns a depth-first list of this boundary and all its descendants.
func (b *CallBoundary) FlattenAll() []*CallBoundary {
	out := []*CallBoundary{b}
	for _, sub := range b.SubCalls {
		out = append(out, sub.FlattenAll()...)
	}
	return out
}

// Flatten returns a depth-first list that respects the Expanded flag.
// Collapsed boundaries do not include their descendants.
func (b *CallBoundary) Flatten() []*CallBoundary {
	out := []*CallBoundary{b}
	if b.Expanded {
		for _, sub := range b.SubCalls {
			out = append(out, sub.Flatten()...)
		}
	}
	return out
}

// ExpandAll recursively expands this boundary and all descendants.
func (b *CallBoundary) ExpandAll() {
	b.Expanded = true
	for _, sub := range b.SubCalls {
		sub.ExpandAll()
	}
}

// CollapseAll recursively collapses this boundary and all descendants.
func (b *CallBoundary) CollapseAll() {
	b.Expanded = false
	for _, sub := range b.SubCalls {
		sub.CollapseAll()
	}
}

// SubcallGraph is the top-level container for the multi-step call graph of a
// transaction. It holds the root-level calls and provides navigation helpers.
type SubcallGraph struct {
	// TransactionHash identifies the transaction this graph belongs to.
	TransactionHash string

	// RootCalls are the top-level contract invocations in execution order.
	RootCalls []*CallBoundary

	// PartialExecution is true when the transaction did not complete — i.e.
	// at least one root call has no exit step.
	PartialExecution bool
}

// BuildSubcallGraph derives a SubcallGraph from an ExecutionTrace by scanning
// the ExecutionState steps for contract_call events and their nesting depth.
//
// The algorithm uses ContractID + Function transitions and the call-stack depth
// recorded in snapshots to infer entry and exit boundaries.
func BuildSubcallGraph(trace *ExecutionTrace) *SubcallGraph {
	g := &SubcallGraph{
		TransactionHash: trace.TransactionHash,
	}

	// Stack of open boundaries. When a new contract_call is encountered we push
	// a boundary; when the next event at a shallower depth appears we close it.
	var stack []*CallBoundary
	seenContracts := map[string]int{} // contractID -> count for unique IDs

	for i, state := range trace.States {
		if state.ContractID == "" && state.Function == "" {
			continue
		}

		currentDepth := len(stack)
		stateDepth := estimateDepth(state)

		// Pop boundaries that are deeper than the current state's depth.
		for len(stack) > 0 && stack[len(stack)-1].Depth >= stateDepth {
			top := stack[len(stack)-1]
			if top.ExitStep < 0 {
				top.ExitStep = i - 1
				if i > 0 && trace.States[i-1].Error != "" {
					top.Error = trace.States[i-1].Error
				}
			}
			stack = stack[:len(stack)-1]
		}

		_ = currentDepth // used implicitly via stack length

		// Only create a new boundary for contract_call events.
		if state.EventType != EventTypeContractCall && state.EventType != "" {
			continue
		}
		if state.ContractID == "" && state.Function == "" {
			continue
		}

		idKey := state.ContractID
		seenContracts[idKey]++
		boundaryID := fmt.Sprintf("call-%d", seenContracts[idKey]-1)
		if len(stack) > 0 {
			boundaryID = stack[len(stack)-1].ID + "." + fmt.Sprintf("%d", len(stack[len(stack)-1].SubCalls))
		}

		b := &CallBoundary{
			ID:         boundaryID,
			ContractID: state.ContractID,
			Function:   state.Function,
			EntryStep:  i,
			ExitStep:   -1,
			Depth:      stateDepth,
			Expanded:   true,
		}

		if len(stack) > 0 {
			parent := stack[len(stack)-1]
			b.Parent = parent
			parent.SubCalls = append(parent.SubCalls, b)
		} else {
			g.RootCalls = append(g.RootCalls, b)
		}

		stack = append(stack, b)
	}

	// Close any boundaries still open at the end of the trace.
	for _, open := range stack {
		if open.ExitStep < 0 {
			if len(trace.States) > 0 {
				open.ExitStep = len(trace.States) - 1
			}
			g.PartialExecution = true
		}
	}

	return g
}

// estimateDepth infers the call depth from an ExecutionState. It uses the
// EventType and depth-sensitive heuristics based on the call stack.
func estimateDepth(s ExecutionState) int {
	// Use depth encoded in the call stack if available.
	// This is a simplified heuristic; in a full implementation the simulator
	// would emit explicit depth fields.
	switch s.EventType {
	case EventTypeContractCall:
		return 0
	case EventTypeHostFunction:
		return 1
	default:
		return 0
	}
}

// AllBoundaries returns all boundaries in depth-first order across all roots.
func (g *SubcallGraph) AllBoundaries() []*CallBoundary {
	var out []*CallBoundary
	for _, root := range g.RootCalls {
		out = append(out, root.FlattenAll()...)
	}
	return out
}

// VisibleBoundaries returns boundaries respecting the Expanded state.
func (g *SubcallGraph) VisibleBoundaries() []*CallBoundary {
	var out []*CallBoundary
	for _, root := range g.RootCalls {
		out = append(out, root.Flatten()...)
	}
	return out
}

// BoundaryAt returns the innermost boundary whose step range contains step, or
// nil if no boundary covers it.
func (g *SubcallGraph) BoundaryAt(step int) *CallBoundary {
	var best *CallBoundary
	for _, b := range g.AllBoundaries() {
		if b.EntryStep <= step && (b.ExitStep < 0 || step <= b.ExitStep) {
			if best == nil || b.Depth > best.Depth {
				best = b
			}
		}
	}
	return best
}

// ExpandAll expands every boundary in the graph.
func (g *SubcallGraph) ExpandAll() {
	for _, root := range g.RootCalls {
		root.ExpandAll()
	}
}

// CollapseAll collapses every boundary in the graph.
func (g *SubcallGraph) CollapseAll() {
	for _, root := range g.RootCalls {
		root.CollapseAll()
	}
}

// PrintGraph prints a text representation of the call graph to a strings.Builder.
func (g *SubcallGraph) PrintGraph() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Transaction: %s\n", g.TransactionHash))
	if g.PartialExecution {
		sb.WriteString("  [partial execution — transaction did not complete]\n")
	}
	for _, root := range g.RootCalls {
		printBoundary(&sb, root, 0)
	}
	return sb.String()
}

func printBoundary(sb *strings.Builder, b *CallBoundary, indent int) {
	prefix := strings.Repeat("  ", indent)
	steps := ""
	if b.ExitStep >= 0 {
		steps = fmt.Sprintf("steps %d–%d", b.EntryStep, b.ExitStep)
	} else {
		steps = fmt.Sprintf("step %d (open)", b.EntryStep)
	}
	status := ""
	if b.HasError() {
		status = fmt.Sprintf(" [ERROR: %s]", b.Error)
	} else if b.IsPartial() {
		status = " [partial]"
	}
	collapsed := ""
	if !b.Expanded && len(b.SubCalls) > 0 {
		collapsed = fmt.Sprintf(" [+%d subcalls]", len(b.SubCalls))
	}
	sb.WriteString(fmt.Sprintf("%s▶ %s (%s)%s%s\n", prefix, b.Label(), steps, status, collapsed))
	if b.Expanded {
		for _, sub := range b.SubCalls {
			printBoundary(sb, sub, indent+1)
		}
	}
}

// NavigateToParentCall returns the step index of the parent call boundary for
// the boundary that contains the given step. If the step is in a root call,
// the same step is returned unchanged.
func (g *SubcallGraph) NavigateToParentCall(step int) int {
	b := g.BoundaryAt(step)
	if b == nil || b.Parent == nil {
		return step
	}
	return b.Parent.EntryStep
}

// NavigateToFirstSubcall returns the entry step of the first subcall within
// the boundary that contains the given step. Returns step unchanged when there
// are no subcalls.
func (g *SubcallGraph) NavigateToFirstSubcall(step int) int {
	b := g.BoundaryAt(step)
	if b == nil || len(b.SubCalls) == 0 {
		return step
	}
	return b.SubCalls[0].EntryStep
}

// NavigateToNextSibling returns the entry step of the next call at the same
// depth. Returns step unchanged when there is no next sibling.
func (g *SubcallGraph) NavigateToNextSibling(step int) int {
	b := g.BoundaryAt(step)
	if b == nil {
		return step
	}

	siblings := g.RootCalls
	if b.Parent != nil {
		siblings = b.Parent.SubCalls
	}

	for i, sib := range siblings {
		if sib == b && i+1 < len(siblings) {
			return siblings[i+1].EntryStep
		}
	}
	return step
}

// NavigateToPrevSibling returns the entry step of the previous call at the same
// depth. Returns step unchanged when there is no previous sibling.
func (g *SubcallGraph) NavigateToPrevSibling(step int) int {
	b := g.BoundaryAt(step)
	if b == nil {
		return step
	}

	siblings := g.RootCalls
	if b.Parent != nil {
		siblings = b.Parent.SubCalls
	}

	for i, sib := range siblings {
		if sib == b && i > 0 {
			return siblings[i-1].EntryStep
		}
	}
	return step
}

// SubcallGraph returns the SubcallGraph for this trace, building it lazily on
// first access and caching the result for subsequent calls.
func (t *ExecutionTrace) SubcallGraph() *SubcallGraph {
	if t.cachedSubcallGraph == nil {
		t.cachedSubcallGraph = BuildSubcallGraph(t)
	}
	return t.cachedSubcallGraph
}
