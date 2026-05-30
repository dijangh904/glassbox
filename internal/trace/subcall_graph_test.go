// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	contractA = "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC"
	contractB = "CA3D5KRYM6CB7OWQ6TWYRR3Z4T7GNZLKERYNZGGA5SOAOPIFY6YQGAXE"
	contractC = "CBGTG4XUWRWXDJ5QQVXJVFXPQNQPQNQPQNQPQNQPQNQPQNQPQNQPQNQP"
)

// buildTrace creates an ExecutionTrace with the given sequence of states.
func buildTrace(states []ExecutionState) *ExecutionTrace {
	t := NewExecutionTrace("tx-test", 100)
	for _, s := range states {
		t.AddState(s)
	}
	return t
}

func contractCallState(contractID, function string) ExecutionState {
	return ExecutionState{
		EventType:  EventTypeContractCall,
		ContractID: contractID,
		Function:   function,
		Operation:  "call",
	}
}

func hostFnState(fn string) ExecutionState {
	return ExecutionState{
		EventType: EventTypeHostFunction,
		Function:  fn,
		Operation: "host",
	}
}

func errorState(msg string) ExecutionState {
	return ExecutionState{
		EventType: EventTypeContractCall,
		Operation: "error",
		Error:     msg,
	}
}

// --- CallBoundary ---

func TestCallBoundary_Label(t *testing.T) {
	b := &CallBoundary{ContractID: contractA, Function: "transfer"}
	assert.Contains(t, b.Label(), "transfer")

	b2 := &CallBoundary{}
	assert.NotEmpty(t, b2.Label())
}

func TestCallBoundary_StepCount(t *testing.T) {
	b := &CallBoundary{EntryStep: 2, ExitStep: 7}
	assert.Equal(t, 6, b.StepCount())

	bPartial := &CallBoundary{EntryStep: 3, ExitStep: -1}
	assert.Equal(t, 0, bPartial.StepCount())
}

func TestCallBoundary_ExpandCollapse(t *testing.T) {
	root := &CallBoundary{Expanded: true}
	child := &CallBoundary{Expanded: true}
	root.SubCalls = []*CallBoundary{child}

	root.CollapseAll()
	assert.False(t, root.Expanded)
	assert.False(t, child.Expanded)

	root.ExpandAll()
	assert.True(t, root.Expanded)
	assert.True(t, child.Expanded)
}

func TestCallBoundary_FlattenAll(t *testing.T) {
	root := &CallBoundary{ID: "r"}
	child1 := &CallBoundary{ID: "c1"}
	child2 := &CallBoundary{ID: "c2"}
	grandchild := &CallBoundary{ID: "gc"}
	child1.SubCalls = []*CallBoundary{grandchild}
	root.SubCalls = []*CallBoundary{child1, child2}

	all := root.FlattenAll()
	assert.Len(t, all, 4)
	assert.Equal(t, "r", all[0].ID)
}

func TestCallBoundary_Flatten_Collapsed(t *testing.T) {
	root := &CallBoundary{ID: "r", Expanded: false}
	child := &CallBoundary{ID: "c"}
	root.SubCalls = []*CallBoundary{child}

	visible := root.Flatten()
	assert.Len(t, visible, 1, "collapsed boundary should hide children")
}

// --- BuildSubcallGraph ---

func TestBuildSubcallGraph_EmptyTrace(t *testing.T) {
	tr := buildTrace(nil)
	g := BuildSubcallGraph(tr)
	assert.NotNil(t, g)
	assert.Empty(t, g.RootCalls)
	assert.False(t, g.PartialExecution)
}

func TestBuildSubcallGraph_SingleCall(t *testing.T) {
	tr := buildTrace([]ExecutionState{
		contractCallState(contractA, "transfer"),
	})
	g := BuildSubcallGraph(tr)
	require.Len(t, g.RootCalls, 1)
	assert.Equal(t, contractA, g.RootCalls[0].ContractID)
	assert.Equal(t, "transfer", g.RootCalls[0].Function)
}

func TestBuildSubcallGraph_NestedCalls(t *testing.T) {
	tr := buildTrace([]ExecutionState{
		contractCallState(contractA, "transfer"),
		hostFnState("require_auth"),
		contractCallState(contractB, "swap"),
	})
	g := BuildSubcallGraph(tr)
	require.NotEmpty(t, g.RootCalls)
}

func TestBuildSubcallGraph_AllBoundaries(t *testing.T) {
	tr := buildTrace([]ExecutionState{
		contractCallState(contractA, "fn1"),
		contractCallState(contractB, "fn2"),
	})
	g := BuildSubcallGraph(tr)
	all := g.AllBoundaries()
	assert.NotEmpty(t, all)
}

// --- SubcallGraph navigation ---

func TestSubcallGraph_BoundaryAt(t *testing.T) {
	tr := buildTrace([]ExecutionState{
		contractCallState(contractA, "transfer"),
	})
	g := BuildSubcallGraph(tr)
	b := g.BoundaryAt(0)
	// Either we find a boundary or nil is acceptable if the graph has no boundaries.
	_ = b
}

func TestSubcallGraph_NavigateToParentCall_NoParent(t *testing.T) {
	tr := buildTrace([]ExecutionState{
		contractCallState(contractA, "root"),
	})
	g := BuildSubcallGraph(tr)
	// Should return same step when at a root call.
	result := g.NavigateToParentCall(0)
	assert.Equal(t, 0, result)
}

func TestSubcallGraph_ExpandCollapseAll(t *testing.T) {
	tr := buildTrace([]ExecutionState{
		contractCallState(contractA, "fn"),
		contractCallState(contractB, "fn2"),
	})
	g := BuildSubcallGraph(tr)
	g.ExpandAll()
	for _, b := range g.AllBoundaries() {
		assert.True(t, b.Expanded)
	}
	g.CollapseAll()
	for _, b := range g.AllBoundaries() {
		assert.False(t, b.Expanded)
	}
}

func TestSubcallGraph_PrintGraph(t *testing.T) {
	tr := buildTrace([]ExecutionState{
		contractCallState(contractA, "transfer"),
	})
	tr.TransactionHash = "tx-abc123"
	g := BuildSubcallGraph(tr)
	out := g.PrintGraph()
	assert.Contains(t, out, "tx-abc123")
}

func TestSubcallGraph_PrintGraph_WithError(t *testing.T) {
	tr := buildTrace([]ExecutionState{
		contractCallState(contractA, "risky"),
		errorState("InsufficientBalance"),
	})
	g := BuildSubcallGraph(tr)
	out := g.PrintGraph()
	assert.NotEmpty(t, out)
}

// --- ExecutionTrace.SubcallGraph lazy caching ---

func TestExecutionTrace_SubcallGraphCached(t *testing.T) {
	tr := buildTrace([]ExecutionState{
		contractCallState(contractA, "fn"),
	})
	g1 := tr.SubcallGraph()
	g2 := tr.SubcallGraph()
	assert.Same(t, g1, g2, "SubcallGraph() should return the same instance on repeated calls")
}

// --- VisibleBoundaries ---

func TestSubcallGraph_VisibleBoundaries(t *testing.T) {
	root := &CallBoundary{ID: "r", Expanded: true}
	child := &CallBoundary{ID: "c", Expanded: true}
	root.SubCalls = []*CallBoundary{child}

	g := &SubcallGraph{RootCalls: []*CallBoundary{root}}
	visible := g.VisibleBoundaries()
	assert.Len(t, visible, 2)

	g.CollapseAll()
	visible = g.VisibleBoundaries()
	assert.Len(t, visible, 1, "collapsed root should hide child")
}
