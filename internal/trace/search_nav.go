// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"fmt"
	"strings"
)

// SearchMode tracks whether the viewer is in normal navigation or search mode.
type SearchMode int

const (
	// SearchModeOff — normal navigation, no active query.
	SearchModeOff SearchMode = iota
	// SearchModeInput — user is typing a query (after pressing /).
	SearchModeInput
	// SearchModeActive — query committed, n/N navigate matches.
	SearchModeActive
)

// FilterMode controls whether unmatched nodes are hidden in the tree.
type FilterMode int

const (
	// FilterModeOff — all nodes visible regardless of search.
	FilterModeOff FilterMode = iota
	// FilterModeOn — hide nodes outside the current search scope while
	// preserving parent branches for context.
	FilterModeOn
)

// SearchState bundles all mutable search/filter state for the interactive
// viewer. It is designed to be embedded in InteractiveViewer and persists
// across command transitions and refreshes.
type SearchState struct {
	engine     *SearchEngine
	mode       SearchMode
	filterMode FilterMode
	inputBuf   strings.Builder // accumulates characters while SearchModeInput
	// flatNodes is the last-indexed flat node list; rebuilt on demand.
	flatNodes []*TraceNode
}

// NewSearchState creates a zeroed SearchState with a fresh engine.
func NewSearchState() *SearchState {
	return &SearchState{
		engine: NewSearchEngine(),
		mode:   SearchModeOff,
	}
}

// IndexNodes indexes a flat list of trace nodes for searching.
// Call this whenever the trace tree changes.
func (s *SearchState) IndexNodes(nodes []*TraceNode) {
	s.flatNodes = nodes
	if s.mode == SearchModeActive && s.engine.GetQuery() != "" {
		s.engine.Search(nodes)
	}
}

// BeginInput switches to input mode, resetting the input buffer.
func (s *SearchState) BeginInput() {
	s.mode = SearchModeInput
	s.inputBuf.Reset()
}

// AppendChar adds a character to the input buffer while in input mode.
func (s *SearchState) AppendChar(ch rune) {
	if s.mode == SearchModeInput {
		s.inputBuf.WriteRune(ch)
	}
}

// Backspace removes the last character from the input buffer.
func (s *SearchState) Backspace() {
	if s.mode != SearchModeInput {
		return
	}
	buf := s.inputBuf.String()
	if len(buf) == 0 {
		return
	}
	runes := []rune(buf)
	s.inputBuf.Reset()
	s.inputBuf.WriteString(string(runes[:len(runes)-1]))
}

// CommitQuery finalises the input buffer as the active query and runs the
// search. Switches to SearchModeActive. Returns the number of matches found.
func (s *SearchState) CommitQuery() int {
	query := strings.TrimSpace(s.inputBuf.String())
	s.engine.SetQuery(query)
	if query == "" {
		s.mode = SearchModeOff
		return 0
	}
	s.engine.Search(s.flatNodes)
	s.mode = SearchModeActive
	return s.engine.MatchCount()
}

// ClearSearch resets all search state and returns to normal navigation.
func (s *SearchState) ClearSearch() {
	s.engine.SetQuery("")
	s.mode = SearchModeOff
	s.filterMode = FilterModeOff
	s.inputBuf.Reset()
}

// NextMatch advances to the next match and returns it (nil if none).
func (s *SearchState) NextMatch() *NodeMatch {
	if s.mode != SearchModeActive {
		return nil
	}
	return s.engine.NextMatch()
}

// PrevMatch moves to the previous match and returns it (nil if none).
func (s *SearchState) PrevMatch() *NodeMatch {
	if s.mode != SearchModeActive {
		return nil
	}
	return s.engine.PreviousMatch()
}

// CurrentMatch returns the current match without advancing.
func (s *SearchState) CurrentMatch() *NodeMatch {
	return s.engine.CurrentMatch()
}

// ToggleFilterMode switches between FilterModeOff and FilterModeOn.
// Filter mode is only meaningful when a query is active.
func (s *SearchState) ToggleFilterMode() FilterMode {
	if s.filterMode == FilterModeOff {
		s.filterMode = FilterModeOn
	} else {
		s.filterMode = FilterModeOff
	}
	return s.filterMode
}

// IsFiltering returns true when filter mode is on and a query is active.
func (s *SearchState) IsFiltering() bool {
	return s.filterMode == FilterModeOn && s.mode == SearchModeActive
}

// MatchCount returns the total number of matches for the current query.
func (s *SearchState) MatchCount() int {
	return s.engine.MatchCount()
}

// CurrentMatchNumber returns the 1-based index of the current match.
func (s *SearchState) CurrentMatchNumber() int {
	return s.engine.CurrentMatchNumber()
}

// Query returns the active search query string.
func (s *SearchState) Query() string {
	return s.engine.GetQuery()
}

// InputBuf returns the current (uncommitted) input buffer contents.
func (s *SearchState) InputBuf() string {
	return s.inputBuf.String()
}

// Mode returns the current search mode.
func (s *SearchState) Mode() SearchMode {
	return s.mode
}

// MatchCounterLine returns a compact status string for the viewer header/footer.
// Examples:
//
//	"[1/12 matches: transfer]"
//	"[no matches: xyz]"
//	"[filter ON | 1/12: transfer]"
func (s *SearchState) MatchCounterLine() string {
	if s.mode == SearchModeInput {
		return fmt.Sprintf("[search: %s_]", s.inputBuf.String())
	}
	if s.mode != SearchModeActive || s.engine.GetQuery() == "" {
		return ""
	}
	total := s.engine.MatchCount()
	if total == 0 {
		return fmt.Sprintf("[no matches: %s]", s.engine.GetQuery())
	}
	cur := s.engine.CurrentMatchNumber()
	filterTag := ""
	if s.filterMode == FilterModeOn {
		filterTag = "filter ON | "
	}
	return fmt.Sprintf("[%s%d/%d: %s]", filterTag, cur, total, s.engine.GetQuery())
}

// IsNodeVisible reports whether node should be shown in the tree when filter
// mode is active. A node is visible if:
//   - filter mode is off, OR
//   - the node itself matches the query, OR
//   - any ancestor of the node matches (preserve context branches).
func (s *SearchState) IsNodeVisible(node *TraceNode) bool {
	if !s.IsFiltering() {
		return true
	}
	return s.nodeOrAncestorMatches(node)
}

// nodeOrAncestorMatches returns true if node or any of its ancestors is a
// search match. This preserves the hierarchical context for matched branches.
func (s *SearchState) nodeOrAncestorMatches(node *TraceNode) bool {
	if node == nil {
		return false
	}
	// Check if this node itself matches.
	if s.nodeMatches(node) {
		return true
	}
	// Check descendants — if any child matches, keep this node visible as context.
	if s.anyDescendantMatches(node) {
		return true
	}
	return false
}

// nodeMatches returns true if the given node is in the current match list.
func (s *SearchState) nodeMatches(node *TraceNode) bool {
	for _, m := range s.engine.matches {
		if m.NodeID == node.ID {
			return true
		}
	}
	return false
}

// anyDescendantMatches returns true if any descendant of node is a match.
func (s *SearchState) anyDescendantMatches(node *TraceNode) bool {
	for _, child := range node.Children {
		if s.nodeMatches(child) || s.anyDescendantMatches(child) {
			return true
		}
	}
	return false
}

// FilteredFlatNodes returns the flat node list filtered by the current search
// state. When filter mode is off, returns all nodes unchanged.
func (s *SearchState) FilteredFlatNodes() []*TraceNode {
	if !s.IsFiltering() {
		return s.flatNodes
	}
	out := make([]*TraceNode, 0, len(s.flatNodes))
	for _, n := range s.flatNodes {
		if s.IsNodeVisible(n) {
			out = append(out, n)
		}
	}
	return out
}
