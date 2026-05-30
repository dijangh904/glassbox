// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─────────────────────────────────────────────────────────────────────────────
// Fixtures
// ─────────────────────────────────────────────────────────────────────────────

// buildFixtureNodes returns a deterministic flat node list for testing.
//
//	node-0: type=contract_call, function=transfer, contractID=CAAA...
//	node-1: type=host_fn,       function=get_balance
//	node-2: type=contract_call, function=transfer, contractID=CBBB...
//	node-3: type=error,         error=budget exceeded
//	node-4: type=host_fn,       function=emit_event, eventData=transfer
func buildFixtureNodes() []*TraceNode {
	nodes := make([]*TraceNode, 5)

	nodes[0] = NewTraceNode("n0", "contract_call")
	nodes[0].Function = "transfer"
	nodes[0].ContractID = "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA1"

	nodes[1] = NewTraceNode("n1", "host_fn")
	nodes[1].Function = "get_balance"

	nodes[2] = NewTraceNode("n2", "contract_call")
	nodes[2].Function = "transfer"
	nodes[2].ContractID = "CBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB2"

	nodes[3] = NewTraceNode("n3", "error")
	nodes[3].Error = "budget exceeded"

	nodes[4] = NewTraceNode("n4", "host_fn")
	nodes[4].Function = "emit_event"
	nodes[4].EventData = "transfer"

	return nodes
}

// buildFixtureTree returns a small tree:
//
//	root (contract_call / transfer)
//	├── child0 (host_fn / get_balance)
//	├── child1 (contract_call / transfer)
//	│   └── grandchild (error / budget exceeded)
//	└── child2 (host_fn / emit_event)
func buildFixtureTree() *TraceNode {
	root := NewTraceNode("root", "contract_call")
	root.Function = "transfer"

	c0 := NewTraceNode("c0", "host_fn")
	c0.Function = "get_balance"

	c1 := NewTraceNode("c1", "contract_call")
	c1.Function = "transfer"

	gc := NewTraceNode("gc", "error")
	gc.Error = "budget exceeded"
	c1.AddChild(gc)

	c2 := NewTraceNode("c2", "host_fn")
	c2.Function = "emit_event"

	root.AddChild(c0)
	root.AddChild(c1)
	root.AddChild(c2)

	return root
}

// ─────────────────────────────────────────────────────────────────────────────
// SearchState unit tests
// ─────────────────────────────────────────────────────────────────────────────

func TestSearchState_IndexAndSearch_FindsMatches(t *testing.T) {
	nodes := buildFixtureNodes()
	s := NewSearchState()
	s.IndexNodes(nodes)

	s.engine.SetQuery("transfer")
	matches := s.engine.Search(nodes)

	// nodes[0], nodes[2] have Function=transfer; nodes[4] has EventData=transfer
	assert.GreaterOrEqual(t, len(matches), 2, "should find at least 2 matches for 'transfer'")
}

func TestSearchState_NextPrev_Navigation(t *testing.T) {
	nodes := buildFixtureNodes()
	s := NewSearchState()
	s.IndexNodes(nodes)

	s.engine.SetQuery("transfer")
	s.engine.Search(nodes)
	s.mode = SearchModeActive

	total := s.MatchCount()
	require.Greater(t, total, 1, "need at least 2 matches to test navigation")

	first := s.NextMatch()
	require.NotNil(t, first)
	assert.Equal(t, 1, s.CurrentMatchNumber())

	second := s.NextMatch()
	require.NotNil(t, second)
	assert.Equal(t, 2, s.CurrentMatchNumber())

	// PrevMatch should go back to first.
	back := s.PrevMatch()
	require.NotNil(t, back)
	assert.Equal(t, 1, s.CurrentMatchNumber())
}

func TestSearchState_NextMatch_WrapsAround(t *testing.T) {
	nodes := buildFixtureNodes()
	s := NewSearchState()
	s.IndexNodes(nodes)

	s.engine.SetQuery("transfer")
	s.engine.Search(nodes)
	s.mode = SearchModeActive

	total := s.MatchCount()
	// Advance past all matches.
	for i := 0; i < total; i++ {
		s.NextMatch()
	}
	// One more should wrap to match 1.
	wrapped := s.NextMatch()
	require.NotNil(t, wrapped)
	assert.Equal(t, 1, s.CurrentMatchNumber(), "should wrap around to first match")
}

func TestSearchState_MatchCounterLine_Formats(t *testing.T) {
	nodes := buildFixtureNodes()
	s := NewSearchState()
	s.IndexNodes(nodes)

	// No active search.
	assert.Empty(t, s.MatchCounterLine())

	// Input mode.
	s.BeginInput()
	s.AppendChar('t')
	s.AppendChar('x')
	assert.Contains(t, s.MatchCounterLine(), "search:")
	assert.Contains(t, s.MatchCounterLine(), "tx")

	// Active search with matches.
	s.engine.SetQuery("transfer")
	s.engine.Search(nodes)
	s.mode = SearchModeActive
	s.NextMatch()
	line := s.MatchCounterLine()
	assert.Contains(t, line, "1/")
	assert.Contains(t, line, "transfer")

	// No matches.
	s.engine.SetQuery("zzznomatch")
	s.engine.Search(nodes)
	s.mode = SearchModeActive
	line = s.MatchCounterLine()
	assert.Contains(t, line, "no matches")
}

func TestSearchState_ClearSearch_ResetsAll(t *testing.T) {
	nodes := buildFixtureNodes()
	s := NewSearchState()
	s.IndexNodes(nodes)

	s.engine.SetQuery("transfer")
	s.engine.Search(nodes)
	s.mode = SearchModeActive
	s.filterMode = FilterModeOn

	s.ClearSearch()

	assert.Equal(t, SearchModeOff, s.Mode())
	assert.Equal(t, FilterModeOff, s.filterMode)
	assert.Empty(t, s.Query())
	assert.Empty(t, s.MatchCounterLine())
}

func TestSearchState_InputBuffer_AppendBackspace(t *testing.T) {
	s := NewSearchState()
	s.BeginInput()
	s.AppendChar('a')
	s.AppendChar('b')
	s.AppendChar('c')
	assert.Equal(t, "abc", s.InputBuf())

	s.Backspace()
	assert.Equal(t, "ab", s.InputBuf())

	s.Backspace()
	s.Backspace()
	assert.Equal(t, "", s.InputBuf())

	// Backspace on empty buffer should not panic.
	s.Backspace()
	assert.Equal(t, "", s.InputBuf())
}

func TestSearchState_CommitQuery_SetsActiveMode(t *testing.T) {
	nodes := buildFixtureNodes()
	s := NewSearchState()
	s.IndexNodes(nodes)

	s.BeginInput()
	s.AppendChar('t')
	s.AppendChar('r')
	s.AppendChar('a')
	s.AppendChar('n')
	s.AppendChar('s')
	s.AppendChar('f')
	s.AppendChar('e')
	s.AppendChar('r')

	count := s.CommitQuery()
	assert.Equal(t, SearchModeActive, s.Mode())
	assert.Greater(t, count, 0)
}

func TestSearchState_CommitEmptyQuery_ClearsSearch(t *testing.T) {
	s := NewSearchState()
	s.BeginInput()
	// Don't append anything.
	count := s.CommitQuery()
	assert.Equal(t, SearchModeOff, s.Mode())
	assert.Equal(t, 0, count)
}

// ─────────────────────────────────────────────────────────────────────────────
// Filter mode tests
// ─────────────────────────────────────────────────────────────────────────────

func TestSearchState_FilterMode_HidesNonMatchingNodes(t *testing.T) {
	nodes := buildFixtureNodes()
	s := NewSearchState()
	s.IndexNodes(nodes)

	s.engine.SetQuery("budget")
	s.engine.Search(nodes)
	s.mode = SearchModeActive
	s.filterMode = FilterModeOn

	filtered := s.FilteredFlatNodes()
	// Only node-3 (error: "budget exceeded") should be visible.
	require.Len(t, filtered, 1)
	assert.Equal(t, "n3", filtered[0].ID)
}

func TestSearchState_FilterMode_PreservesParentBranches(t *testing.T) {
	root := buildFixtureTree()
	allNodes := root.FlattenAll()

	s := NewSearchState()
	s.IndexNodes(allNodes)

	// Search for "budget exceeded" — only the grandchild matches.
	s.engine.SetQuery("budget")
	s.engine.Search(allNodes)
	s.mode = SearchModeActive
	s.filterMode = FilterModeOn

	// The grandchild matches; its parent (c1) should be visible as context.
	assert.True(t, s.IsNodeVisible(root), "root should be visible as ancestor context")

	// Find c1 node.
	var c1 *TraceNode
	for _, n := range allNodes {
		if n.ID == "c1" {
			c1 = n
			break
		}
	}
	require.NotNil(t, c1)
	assert.True(t, s.IsNodeVisible(c1), "parent of matched node should be visible for context")

	// c0 (get_balance) has no matching descendants — should be hidden.
	var c0 *TraceNode
	for _, n := range allNodes {
		if n.ID == "c0" {
			c0 = n
			break
		}
	}
	require.NotNil(t, c0)
	assert.False(t, s.IsNodeVisible(c0), "unrelated node should be hidden in filter mode")
}

func TestSearchState_FilterMode_Off_ShowsAllNodes(t *testing.T) {
	nodes := buildFixtureNodes()
	s := NewSearchState()
	s.IndexNodes(nodes)

	s.engine.SetQuery("budget")
	s.engine.Search(nodes)
	s.mode = SearchModeActive
	s.filterMode = FilterModeOff // explicitly off

	filtered := s.FilteredFlatNodes()
	assert.Len(t, filtered, len(nodes), "all nodes visible when filter mode is off")
}

func TestSearchState_ToggleFilterMode(t *testing.T) {
	s := NewSearchState()
	assert.Equal(t, FilterModeOff, s.filterMode)

	mode := s.ToggleFilterMode()
	assert.Equal(t, FilterModeOn, mode)

	mode = s.ToggleFilterMode()
	assert.Equal(t, FilterModeOff, mode)
}

// ─────────────────────────────────────────────────────────────────────────────
// Pagination / large trace tests
// ─────────────────────────────────────────────────────────────────────────────

func TestSearchState_LargeTrace_SearchIsResponsive(t *testing.T) {
	// Build 10 000 nodes — search must complete without timeout.
	const n = 10_000
	nodes := make([]*TraceNode, n)
	for i := 0; i < n; i++ {
		node := NewTraceNode(fmt.Sprintf("node-%d", i), "host_fn")
		node.Function = fmt.Sprintf("fn_%d", i)
		if i%100 == 0 {
			node.Function = "transfer"
		}
		nodes[i] = node
	}

	s := NewSearchState()
	s.IndexNodes(nodes)
	s.engine.SetQuery("transfer")
	matches := s.engine.Search(nodes)

	assert.Equal(t, n/100, len(matches), "should find exactly 100 matches in 10k nodes")
}

func TestSearchState_SearchAccuracy_ExactField(t *testing.T) {
	nodes := buildFixtureNodes()
	s := NewSearchState()
	s.IndexNodes(nodes)

	// Search for error text.
	s.engine.SetQuery("budget exceeded")
	matches := s.engine.Search(nodes)

	require.Len(t, matches, 1)
	assert.Equal(t, "n3", matches[0].NodeID)
	assert.Equal(t, "error", matches[0].MatchRanges[0].Field)
}

func TestSearchState_SearchAccuracy_ContractID(t *testing.T) {
	nodes := buildFixtureNodes()
	s := NewSearchState()
	s.IndexNodes(nodes)

	s.engine.SetQuery("CAAA")
	matches := s.engine.Search(nodes)

	require.GreaterOrEqual(t, len(matches), 1)
	assert.Equal(t, "n0", matches[0].NodeID)
}

// ─────────────────────────────────────────────────────────────────────────────
// Stable state transitions
// ─────────────────────────────────────────────────────────────────────────────

func TestSearchState_StateTransitions(t *testing.T) {
	nodes := buildFixtureNodes()
	s := NewSearchState()
	s.IndexNodes(nodes)

	// Off → Input
	s.BeginInput()
	assert.Equal(t, SearchModeInput, s.Mode())

	// Input → Active (commit)
	s.AppendChar('t')
	s.AppendChar('r')
	count := s.CommitQuery()
	assert.Equal(t, SearchModeActive, s.Mode())
	assert.Greater(t, count, 0)

	// Active → Off (clear)
	s.ClearSearch()
	assert.Equal(t, SearchModeOff, s.Mode())
	assert.Equal(t, FilterModeOff, s.filterMode)

	// Off → Input → Off (escape via empty commit)
	s.BeginInput()
	s.CommitQuery() // empty query
	assert.Equal(t, SearchModeOff, s.Mode())
}
