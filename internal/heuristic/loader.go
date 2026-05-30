// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package heuristic

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// LoadRulesFromFile parses a single JSON or YAML rule file and returns the
// compiled rules it contains.  The file format is detected from the extension
// (.json → JSON, .yaml / .yml → YAML).
func LoadRulesFromFile(path string) ([]*Rule, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading rule file %q: %w", path, err)
	}

	ext := strings.ToLower(filepath.Ext(path))
	return rulesFromBytes(data, ext, path)
}

// LoadRulesFromDir loads all .json, .yaml, and .yml files from dir and merges
// their rules into a single sorted RuleSet.  Files are processed in
// lexicographic order so that the result is deterministic.
func LoadRulesFromDir(dir string) (*RuleSet, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading rule directory %q: %w", dir, err)
	}

	var allRules []*Rule
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext != ".json" && ext != ".yaml" && ext != ".yml" {
			continue
		}
		rules, err := LoadRulesFromFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		allRules = append(allRules, rules...)
	}

	return buildRuleSet(allRules)
}

// LoadRulesFromFiles loads rules from an explicit list of file paths and
// merges them into a single sorted RuleSet.
func LoadRulesFromFiles(paths []string) (*RuleSet, error) {
	var allRules []*Rule
	for _, p := range paths {
		rules, err := LoadRulesFromFile(p)
		if err != nil {
			return nil, err
		}
		allRules = append(allRules, rules...)
	}
	return buildRuleSet(allRules)
}

// buildRuleSet sorts rules by priority (ascending) and checks for duplicate IDs.
func buildRuleSet(rules []*Rule) (*RuleSet, error) {
	// Stable sort: rules with equal priority keep their load order.
	sort.SliceStable(rules, func(i, j int) bool {
		return rules[i].Priority < rules[j].Priority
	})

	// Duplicate ID check.
	seen := make(map[string]string, len(rules))
	for _, r := range rules {
		if prev, ok := seen[r.ID]; ok {
			return nil, fmt.Errorf("duplicate rule ID %q: first defined in %q, redefined in %q",
				r.ID, prev, r.Source)
		}
		seen[r.ID] = r.Source
	}

	return &RuleSet{rules: rules}, nil
}

// ValidateRuleFile parses and compiles a rule file, returning any validation
// errors without building a full engine.  Useful for the CLI validate command.
func ValidateRuleFile(path string) error {
	rules, err := LoadRulesFromFile(path)
	if err != nil {
		return err
	}
	if len(rules) == 0 {
		return fmt.Errorf("rule file %q contains no rules", path)
	}
	return nil
}

// DetectConflicts returns pairs of rule IDs whose pattern sets overlap, meaning
// both rules could fire on the same input.  This is informational – overlapping
// rules are not an error because priority ordering resolves which one wins.
func DetectConflicts(rs *RuleSet) []ConflictPair {
	rules := rs.Rules()
	var conflicts []ConflictPair

	for i := 0; i < len(rules); i++ {
		for j := i + 1; j < len(rules); j++ {
			a, b := rules[i], rules[j]
			if patternsOverlap(a, b) {
				conflicts = append(conflicts, ConflictPair{
					RuleA:    a.ID,
					RuleB:    b.ID,
					SourceA:  a.Source,
					SourceB:  b.Source,
					WinnerID: a.ID, // lower index = higher priority wins
				})
			}
		}
	}
	return conflicts
}

// ConflictPair describes two rules whose patterns overlap.
type ConflictPair struct {
	RuleA    string
	RuleB    string
	SourceA  string
	SourceB  string
	WinnerID string // the rule that would fire first due to priority
}

// patternsOverlap returns true when any pattern from rule a is a substring of
// any pattern from rule b, or vice versa.  This is a conservative heuristic –
// it may produce false positives for complex regexps.
func patternsOverlap(a, b *Rule) bool {
	for _, pa := range a.Patterns {
		pa = strings.ToLower(strings.TrimPrefix(pa, "re:"))
		for _, pb := range b.Patterns {
			pb = strings.ToLower(strings.TrimPrefix(pb, "re:"))
			if strings.Contains(pa, pb) || strings.Contains(pb, pa) {
				return true
			}
		}
	}
	return false
}

// jsonUnmarshalRuleFile is used by rulesFromBytes for JSON files.
func jsonUnmarshalRuleFile(data []byte, rf *RuleFile) error {
	return json.Unmarshal(data, rf)
}
